package nri

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/utils"

	"github.com/Mellanox/rdmamap"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/vishvananda/netlink"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/containernetworking/cni/libcni"
	cni100 "github.com/containernetworking/cni/pkg/types/100"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/client-go/util/retry"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ stub.RunPodInterface          = (*nriPlugin)(nil)
	_ stub.StopPodInterface         = (*nriPlugin)(nil)
	_ stub.CreateContainerInterface = (*nriPlugin)(nil)
)

var (
	defaultCniResultCacheDir = "/var/lib/spidernet/nri"
	defaultCniBinPath        = "/opt/cni/bin"
)

type nriPlugin struct {
	nodeName            string
	spiderpoolNamespace string
	cniBinPath          string
	gpuResourceNames    map[string]struct{}
	logger              *zap.Logger
	nri                 stub.Stub
	kubeletClient       podresourcesapi.PodResourcesListerClient
	conn                *grpc.ClientConn
	client              client.Client
}

func Run(ctx context.Context, client client.Client, nodeName string) error {
	// Check RDMA namespace mode, ensure it is "exclusive" mode
	rdmaNsMode, err := netlink.RdmaSystemGetNetnsMode()
	if err != nil {
		return fmt.Errorf("failed to get RDMA namespace mode: %v", err)
	}
	if rdmaNsMode != "exclusive" {
		return fmt.Errorf("NRI plugin must work in exclusive RDMA namespace mode, current mode: %s", rdmaNsMode)
	}

	n := &nriPlugin{
		nodeName:            nodeName,
		spiderpoolNamespace: utils.GetAgentNamespace(),
		logger:              logutils.Logger.Named("nri"),
		gpuResourceNames:    make(map[string]struct{}),
		client:              client,
	}
	// register the NRI plugin
	nriOpts := []stub.Option{
		stub.WithPluginName(constant.DRADriverName),
		stub.WithPluginIdx("00"),
	}
	stub, err := stub.New(n, nriOpts...)
	if err != nil {
		return fmt.Errorf("failed to create plugin stub: %v", err)
	}
	n.nri = stub

	n.kubeletClient, n.conn, err = GetKubeletResourceClient()
	if err != nil {
		return err
	}

	// TODO: make it configuiretable
	n.gpuResourceNames[NvidiaGPUResourceName] = struct{}{}

	go func() {
		if err = n.nri.Run(ctx); err != nil {
			n.logger.Error("failed to start nri plugin", zap.Error(err))
			n.nri.Stop()
			n.conn.Close()
		}
	}()

	return nil
}

func (n *nriPlugin) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return nil
}

func (n *nriPlugin) CreateContainer(ctx context.Context, sandbox *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	l := n.logger.With(zap.String("CNI_COMMAND", "ADD"), zap.String("podName", sandbox.Name), zap.String("namespace", sandbox.Namespace))

	isHostNetwork := true
	for _, namespace := range sandbox.Linux.Namespaces {
		if namespace.Type == "network" {
			isHostNetwork = false
		}
	}

	if isHostNetwork {
		l.Info("Don't need to setup DRA network for hostNetwork pod, ignore")
		return nil, nil, nil
	}
	// Check if devices have already been allocated for this pod
	// If devices have already been allocated for this pod, skip allocation
	k8sPod := &corev1.Pod{}
	if err := n.client.Get(ctx, client.ObjectKey{Name: sandbox.GetName(), Namespace: sandbox.GetNamespace()}, k8sPod); err != nil {
		l.Error("Failed to get pod", zap.Error(err))
		return nil, nil, err
	}

	if _, ok := k8sPod.Annotations["dra.spidernet.io/nri"]; !ok && len(k8sPod.Spec.ResourceClaims) == 0 {
		l.Info("Pod has no dra resource claim and dra annotation, ignore")
		return nil, nil, nil
	}

	netStatus, err := n.getNetworkStatusFromCache(sandbox.Namespace, sandbox.Name, sandbox.Id)
	if err != nil || len(netStatus) == 0 {
		l.Debug("Failed to get network status from cache, try to get status from pod annotations", zap.Error(err))
		netStatus, err = n.getNetworkStatusFromPodAnnotations(sandbox, k8sPod)
		if err != nil {
			l.Error("Failed to get network status from pod annotations", zap.Error(err))
			return nil, nil, err
		}
	}

	// If devices have already been allocated for this pod, skip allocation but mount the devices
	if len(netStatus) > 0 {
		// Convert the allocated RDMA devices to mounts
		mounts := n.parseRDMACharDevicesToMounts(netStatus)
		l.Info("Pod has already set the dra Network, just mount required RDMA devices to container",
			zap.String("containerName", container.Name),
			zap.Int("mountCount", len(mounts)))

		return &api.ContainerAdjustment{
			Mounts: mounts,
		}, nil, nil
	}

	l.Debug("Pod using dra network and not be setup yet, start to setup dra network", zap.String("containerName", container.Name))
	// Continue with device allocation for the first container
	gpus, err := n.getAllocatedGpusForPodSandbox(ctx, sandbox)
	if err != nil {
		l.Error("Failed to get allocated gpus", zap.Error(err))
		return nil, nil, err
	}

	if len(gpus) == 0 {
		// no GPU allocated to this pod
		n.logger.Info("No GPU resources allocated to this pod, Ignore setup dra network",
			zap.String("podName", sandbox.GetName()),
			zap.String("namespace", sandbox.GetNamespace()))
		return nil, nil, nil
	}

	// var resourceClaimName string
	// for _, rc := range pod.Spec.ResourceClaims {
	// 	if rc.ResourceClaimTemplateName != nil && *rc.ResourceClaimTemplateName != "" {
	// 		resourceClaimName = *rc.ResourceClaimTemplateName
	// 	}
	// 	if rc.ResourceClaimName != nil && *rc.ResourceClaimName != "" {
	// 		resourceClaimName = *rc.ResourceClaimName
	// 	}
	// }

	// if resourceClaimName == "" {
	// 	// no resource claim allocated to this pod
	// 	return nil, nil, nil
	// }

	// rct := &resourcev1beta1.ResourceClaimTemplate{}
	// if err := n.client.Get(ctx, client.ObjectKey{Name: resourceClaimName, Namespace: pod.Namespace}, rct); err != nil {
	// 	return nil, nil, err
	// }

	// isContinue := false
	// for _, req := range rct.Spec.Spec.Devices.Requests {
	// 	if req.DeviceClassName == constant.DRANRIDeviceClass {
	// 		isContinue = true
	// 		break
	// 	}
	// }

	// if !isContinue {
	// 	return nil, nil, nil
	// }

	resourceSlice, err := n.getResourceSliceByNode(ctx)
	if err != nil {
		n.logger.Error("Failed to get resource slice", zap.Error(err))
		return nil, nil, err
	}

	deviceToCniConfigs := filterPfToCniConfigsWithGpuRdmaAffinity(gpus, resourceSlice)
	if len(deviceToCniConfigs) == 0 {
		l.Info("No matched CNI configs with GPU Affinity, Ignore setup dra network")
		return nil, nil, nil
	}

	l.Debug("Found matched CNI configs with GPU Affinity", zap.Any("deviceToCniConfigs", deviceToCniConfigs))
	status, err := n.initPodRdmaNetwork(ctx, l, deviceToCniConfigs, sandbox)
	if err != nil || len(status) == 0 {
		l.Error("Failed to set pod network with gpu affinity", zap.Error(err))
		return nil, nil, err
	}

	statusJSON, err := json.Marshal(status)
	if err != nil {
		l.Error("Failed to marshal network status", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to marshal network status: %v", err)
	}

	// Serialize netStatus to JSON string and update pod annotations in Kubernetes
	if err = n.updatePodNetworkStatus(ctx, l, string(statusJSON), k8sPod); err != nil {
		l.Error("Failed to update pod network status", zap.Error(err))
		return nil, nil, err
	}

	l.Debug("Successfully Dynamically Setup Pod RDMA Network, Updated Pod annotations in Kubernetes with network status",
		zap.String("podNamespace", k8sPod.Namespace),
		zap.String("podName", k8sPod.Name),
		zap.String("podUID", string(k8sPod.UID)),
		zap.Any("netStatus", status))

	return &api.ContainerAdjustment{
		Mounts: n.parseRDMACharDevicesToMounts(status),
	}, nil, nil
}

func (n *nriPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	l := n.logger.With(zap.String("CNI_COMMAND", "DEL"), zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))
	k8sPod := &corev1.Pod{}
	if err := n.client.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, k8sPod); err != nil {
		l.Error("Failed to get pod", zap.Error(err))
		return nil
	}

	if _, ok := k8sPod.Annotations["dra.spidernet.io/nri"]; !ok && len(k8sPod.Spec.ResourceClaims) == 0 {
		l.Info("Pod has no dra resource claim and dra annotation, ignore invoke CNI DEL")
		return nil
	}

	l.Debug("Pod is using DRA network, Start to invoke CNI DEL")
	// First try to get network status from local cache file
	netStatus, err := n.getNetworkStatusFromCache(pod.Namespace, pod.Name, pod.Id)
	if err != nil || len(netStatus) == 0 {
		l.Debug("Failed to get network status from local cache, trying to get from Pod object", zap.Error(err))

		// If not found in cache, try to get from Pod object
		netStatus, err = n.getNetworkStatusFromPodAnnotations(pod, k8sPod)
		if err != nil || len(netStatus) == 0 {
			l.Error("Failed to get network status from Pod annotations", zap.Error(err))
			return nil
		}
	}

	if len(netStatus) == 0 {
		l.Info("No network status found for pod, ignore invoke CNI DEL")
		return nil
	}

	podNetNs, err := n.getPodNetworkNamespace(pod)
	if err != nil {
		l.Error("Failed to get pod network namespace", zap.Error(err))
		return fmt.Errorf("failed to get pod network namespace: %v", err)
	}

	for _, status := range netStatus {
		if status.Name == "" || status.DeviceInfo == nil {
			l.Error("Invalid network status entry", zap.Any("status", status))
			continue
		}

		confList, err := n.loadCniConfig(ctx, l, status.Name, status.DeviceInfo.PciAddress)
		if err != nil {
			l.Error("Failed to load CNI config", zap.Error(err))
			continue
		}

		l.Debug("Got final CNI config, Start invoke CNI DEL", zap.Any("confList", confList))
		rc := buildRuntimeConfig(pod, DeviceInfo{PciAddress: status.DeviceInfo.PciAddress}, status.Interface, podNetNs)
		if err := cniDel(ctx, confList, rc); err != nil {
			l.Error("Failed to delete pod network", zap.Error(err))
			continue
		}
	}

	// Delete the network status cache file
	if err := n.deleteNetworkStatusCache(pod.Namespace, pod.Name, pod.Id); err != nil {
		// Just log the error and continue, don't return an error
		l.Error("Failed to delete network status cache file", zap.Error(err))
	}

	l.Info("Successfully cleaned up device allocation")
	return nil
}

func (n *nriPlugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return nil
}

func (n *nriPlugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (n *nriPlugin) Shutdown(_ context.Context) {
	n.logger.Info("NRI plugin shutting down...")
}

func (n *nriPlugin) initPodRdmaNetwork(ctx context.Context, l *zap.Logger, deviceToCniConfigs map[string]string, sandbox *api.PodSandbox) ([]*NetworkStatus, error) {
	podNetNs, err := n.getPodNetworkNamespace(sandbox)
	if err != nil {
		l.Error("Failed to get pod network namespace", zap.Error(err))
		return nil, fmt.Errorf("failed to get pod network namespace: %v", err)
	}

	var netStatus []*NetworkStatus
	idx := 1
	for pf, cniConfigName := range deviceToCniConfigs {
		podNicName := fmt.Sprintf("net%d", idx)
		// Inject RDMA device to pod network namespace
		deviceInfo, err := n.allocatedRdmaDeviceToPod(l, pf)
		if err != nil {
			l.Error("Failed to allocate RDMA device to pod network namespace",
				zap.String("pfName", pf), zap.Error(err))
			return nil, err
		}

		var result *cni100.Result
		result, err = n.setupPodNetwork(ctx, l, sandbox, cniConfigName, deviceInfo, podNicName, podNetNs)
		if err != nil {
			l.Error("Failed to setup pod network", zap.Error(err))
			return nil, err
		}

		status := &NetworkStatus{
			Name:       cniConfigName,
			DeviceInfo: &deviceInfo,
		}
		status.parseNetworkStatus(result)
		netStatus = append(netStatus, status)
		idx += 1
	}

	return netStatus, nil
}

// setupPodNetwork sets up the pod network
func (n *nriPlugin) setupPodNetwork(ctx context.Context, l *zap.Logger, pod *api.PodSandbox, cniConfigName string, deviceInfo DeviceInfo, nicName string, podNetNs string) (*cni100.Result, error) {
	// Get NetworkAttachmentDefinition object
	confList, err := n.loadCniConfig(ctx, l, cniConfigName, deviceInfo.PciAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to load CNI config: %v", err)
	}

	l.Debug("Got final CNI config, Start invoke CNI ADD", zap.String("confList", string(confList.Bytes)))
	result, err := cniAdd(ctx, confList, buildRuntimeConfig(pod, deviceInfo, nicName, podNetNs))
	if err != nil {
		return nil, fmt.Errorf("failed to add network: %v", err)
	}

	res, err := cni100.NewResultFromResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to convert result: %v", err)
	}

	return res, nil
}

// getPodNetworkNamespace gets the network namespace of a Pod
func (n *nriPlugin) getPodNetworkNamespace(pod *api.PodSandbox) (string, error) {
	// Get the network namespace path of the Pod
	for _, namespace := range pod.Linux.GetNamespaces() {
		if namespace.Type == "network" {
			return namespace.Path, nil
		}
	}

	return "", fmt.Errorf("failed to get network namespace from pod %s", pod.Id)
}

func (n *nriPlugin) allocatedRdmaDeviceToPod(l *zap.Logger, device string) (DeviceInfo, error) {
	l.Debug("Allocating RDMA device to pod network namespace")
	// Get a VF from the device
	vfDevices, err := networking.GetSriovAvailableVfPciAddressesForNetDev(device)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("failed to get VFs from device %s: %v", device, err)
	}

	//
	vfName, err := networking.GetNetNameFromPciAddress(vfDevices[0])
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("failed to get vf name from pci address: %v", err)
	}
	l.Debug("Found Available VFs for device, Allocated the first VF to pod", zap.String("pciAddress", vfDevices[0]), zap.String("vfName", vfName))

	deviceInfo := DeviceInfo{
		PciAddress: vfDevices[0],
	}

	// Inject RDMA device for the VF
	rdmaDevice, err := rdmamap.GetRdmaDeviceForNetdevice(vfName)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("failed to get rdma device for network device %s: %v", vfName, err)
	}

	if rdmaDevice != "" {
		// Add the RDMA device to the allocation record
		charRdmaDevices := rdmamap.GetRdmaCharDevices(rdmaDevice)
		deviceInfo.RdmaDevice = rdmaDevice
		deviceInfo.RdmaCharDevices = charRdmaDevices
	}

	// NOTE: rdma-cni do same thing like this
	// Inject RDMA device to pod network namespace
	// hostDev, err := netlink.RdmaLinkByName(rdmaDevice)
	// if err != nil {
	// 	return DeviceInfo{}, fmt.Errorf("failed to get rdma link for network device %s: %v", rdmaDevice, err)
	// }

	// err = netlink.RdmaLinkSetNsFd(hostDev, uint32(podNetNs))
	// if err != nil {
	// 	return DeviceInfo{}, fmt.Errorf("failed to set RDMA device for network device %s: %v", device, err)
	// }

	l.Debug("Successfully allocated RDMA devices with GPU Affinity to pod network namespace")
	return deviceInfo, nil
}

func (n *nriPlugin) getResourceSliceByNode(ctx context.Context) (*resourcev1beta1.ResourceSlice, error) {
	// Use field selectors to filter ResourceSlices by both nodeName and DRADriverName
	// Create field selector for controller-runtime client
	fieldSelector := client.MatchingFields(map[string]string{
		resourcev1beta1.ResourceSliceSelectorNodeName: n.nodeName,
		resourcev1beta1.ResourceSliceSelectorDriver:   constant.DRADriverName,
	})

	rsList := &resourcev1beta1.ResourceSliceList{}
	if err := n.client.List(ctx, rsList, fieldSelector); err != nil {
		return nil, err
	}

	// Expect only one ResourceSlice to be returned
	if len(rsList.Items) == 0 {
		return nil, fmt.Errorf("no ResourceSlice found for node %s and driver %s", n.nodeName, constant.DRADriverName)
	}

	if len(rsList.Items) > 1 {
		n.logger.Warn("Multiple ResourceSlices found when only one was expected",
			zap.String("nodeName", n.nodeName),
			zap.String("driver", constant.DRADriverName),
			zap.Int("count", len(rsList.Items)))
	}

	// Use the first ResourceSlice
	return &rsList.Items[0], nil
}

func (n *nriPlugin) loadCniConfig(ctx context.Context, l *zap.Logger, netName, deviceId string) (*libcni.NetworkConfigList, error) {
	nad := &netv1.NetworkAttachmentDefinition{}
	if err := n.client.Get(ctx, client.ObjectKey{Namespace: n.spiderpoolNamespace, Name: netName}, nad); err != nil {
		return nil, fmt.Errorf("failed to get NetworkAttachmentDefinition %s/%s: %v", n.spiderpoolNamespace, netName, err)
	}

	if nad.Spec.Config == "" {
		return nil, fmt.Errorf("NetworkAttachmentDefinition %s/%s has empty config", n.spiderpoolNamespace, nad.Name)
	}

	l.Debug("Got CNI config from NetworkAttachmentDefinition", zap.String("nadName", nad.Name), zap.String("config", nad.Spec.Config))
	confList, err := buildSecondaryCniConfig(nad.Name, nad.Spec.Config, deviceId)
	if err != nil {
		return nil, fmt.Errorf("failed to build CNI config from NetworkAttachmentDefinition %s/%s: %v", n.spiderpoolNamespace, nad.Name, err)
	}

	return confList, nil
}

func (n *nriPlugin) updatePodNetworkStatus(ctx context.Context, l *zap.Logger, netStatusJSON string, k8sPod *corev1.Pod) error {
	// Initialize annotations map if it doesn't exist
	if k8sPod.Annotations == nil {
		k8sPod.Annotations = make(map[string]string)
	}

	// Update local pod annotations with network status
	k8sPod.Annotations[constant.AnnoDRAPodNetworkStatus] = string(netStatusJSON)

	// Update Pod in Kubernetes API with retry logic
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the Pod before attempting an update
		latestPod := &corev1.Pod{}
		err := n.client.Get(ctx, client.ObjectKey{Namespace: k8sPod.Namespace, Name: k8sPod.Name}, latestPod)
		if err != nil {
			l.Error("Failed to get latest Pod version", zap.Error(err))
			return err
		}

		// Apply our annotation update to the latest version
		if latestPod.Annotations == nil {
			latestPod.Annotations = make(map[string]string)
		}
		latestPod.Annotations[constant.AnnoDRAPodNetworkStatus] = k8sPod.Annotations[constant.AnnoDRAPodNetworkStatus]

		// Update the Pod with the latest version
		return n.client.Update(ctx, latestPod)
	})

	if err != nil {
		l.Error("Failed to update Pod annotations in Kubernetes after retries", zap.Error(err))
		return fmt.Errorf("failed to update Pod annotations in Kubernetes after retries: %v", err)
	}

	// Save network status to local file
	if err := n.cacheNetworkStatusToFile(l, k8sPod.Namespace, k8sPod.Name, string(k8sPod.UID), string(netStatusJSON)); err != nil {
		l.Error("Failed to save network status to local file", zap.Error(err))
		// Don't return error here, as we've already updated the Pod in Kubernetes
		// Just log the error and continue
	}

	return nil
}

func (n *nriPlugin) parseRDMACharDevicesToMounts(status []*NetworkStatus) []*api.Mount {
	if len(status) == 0 {
		return []*api.Mount{}
	}

	mounts := make([]*api.Mount, 0, len(status)*4)

	// Add each RDMA character device as a mount
	for _, d := range status {
		for _, charDevice := range d.DeviceInfo.RdmaCharDevices {
			if charDevice == rdmamap.RdmaUcmDevice {
				continue
			}

			// Create a mount for the device
			mount := &api.Mount{
				Source:      charDevice,
				Destination: charDevice,
				Type:        "bind",
				Options:     []string{"bind", "rw"},
			}
			mounts = append(mounts, mount)
		}
	}

	// Add the RDMA CM device if it's not already included
	mounts = append(mounts, &api.Mount{
		Source:      rdmamap.RdmaUcmDevice,
		Destination: rdmamap.RdmaUcmDevice,
		Type:        "bind",
		Options:     []string{"bind", "rw"},
	})

	return mounts
}

// cacheNetworkStatusToFile saves the network status JSON to a local file
func (n *nriPlugin) cacheNetworkStatusToFile(l *zap.Logger, namespace, podName, podUID, networkStatusJSON string) error {
	// Create the directory structure if it doesn't exist
	networkStatusDir := filepath.Join(defaultCniResultCacheDir, "network-status")
	if err := os.MkdirAll(networkStatusDir, 0755); err != nil {
		return fmt.Errorf("failed to create network status directory %s: %v", networkStatusDir, err)
	}

	// Create a filename based on the pod's namespace and name
	// This ensures uniqueness and makes it easy to find the file for a specific pod
	fileName := fmt.Sprintf("%s_%s_%s_network_status.json", namespace, podName, podUID)
	filePath := filepath.Join(networkStatusDir, fileName)

	// Write the network status JSON to the file
	if err := os.WriteFile(filePath, []byte(networkStatusJSON), 0644); err != nil {
		return fmt.Errorf("failed to write network status to file %s: %v", filePath, err)
	}

	l.Debug("Successfully saved network status to local file",
		zap.String("namespace", namespace),
		zap.String("podName", podName),
		zap.String("podUID", podUID),
		zap.String("filePath", filePath))

	return nil
}

// getNetworkStatusFromCache reads network status from the local cache file
func (n *nriPlugin) getNetworkStatusFromCache(namespace, podName, podUID string) ([]*NetworkStatus, error) {
	// Construct the expected file path
	networkStatusDir := filepath.Join(defaultCniResultCacheDir, "network-status")
	fileName := fmt.Sprintf("%s_%s_%s.json", namespace, podName, podUID)
	filePath := filepath.Join(networkStatusDir, fileName)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("network status file not found: %s", filePath)
	}

	// Read the file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read network status file %s: %v", filePath, err)
	}

	// Parse the JSON data
	var netStatus []*NetworkStatus
	if err := json.Unmarshal(data, &netStatus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal network status from file: %v", err)
	}

	return netStatus, nil
}

func (n *nriPlugin) getNetworkStatusFromPodAnnotations(sandbox *api.PodSandbox, k8sPod *corev1.Pod) ([]*NetworkStatus, error) {
	var netStatus []*NetworkStatus
	// Check if network status annotation exists
	if k8sPod.Annotations == nil || k8sPod.Annotations[constant.AnnoDRAPodNetworkStatus] == "" {
		return nil, nil
	}

	// Parse network status from annotation
	if err := json.Unmarshal([]byte(k8sPod.Annotations[constant.AnnoDRAPodNetworkStatus]), &netStatus); err != nil {
		n.logger.Error("Failed to unmarshal network status from Pod annotation", zap.Error(err))
		return nil, err
	}

	return netStatus, nil
}

// deleteNetworkStatusCache deletes the network status cache file
// If the file doesn't exist, it returns nil (no error)
func (n *nriPlugin) deleteNetworkStatusCache(namespace, podName, podUID string) error {
	// Construct the expected file path
	networkStatusDir := filepath.Join(defaultCniResultCacheDir, "network-status")
	fileName := fmt.Sprintf("%s_%s_%s.json", namespace, podName, podUID)
	filePath := filepath.Join(networkStatusDir, fileName)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist, nothing to delete
		return nil
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete network status file %s: %v", filePath, err)
	}

	return nil
}
