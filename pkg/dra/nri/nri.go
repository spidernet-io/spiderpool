package nri

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/utils"

	"github.com/Mellanox/rdmamap"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/containernetworking/cni/libcni"
	cni100 "github.com/containernetworking/cni/pkg/types/100"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ stub.RunPodInterface          = (*nriPlugin)(nil)
	_ stub.StopPodInterface         = (*nriPlugin)(nil)
	_ stub.CreateContainerInterface = (*nriPlugin)(nil)
)

var (
	defaultCniResultCacheDir = "/var/lib/spidernet/cni"
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

	// Initialize device tracker
	if err := InitDeviceTracker(); err != nil {
		n.logger.Error("failed to initialize device tracker", zap.Error(err))
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
	l := n.logger.With(zap.String("podName", sandbox.Name), zap.String("namespace", sandbox.Namespace))
	l.Debug("CreateContainer is called", zap.String("containerName", container.Name), zap.Any("annotations", sandbox.Annotations), zap.Any("namespaces", sandbox.Linux.Namespaces))

	isHostNetwork := true
	for _, namespace := range sandbox.Linux.Namespaces {
		if namespace.Type == "network" {
			isHostNetwork = false
		}
	}

	if isHostNetwork {
		l.Info("No need handle hostNetwork pod")
	}
	// Check if devices have already been allocated for this pod
	// If devices have already been allocated for this pod, skip allocation
	allocation, err := GetDeviceAllocation(l, sandbox.Id)
	if err != nil {
		l.Error("Failed to check device allocation", zap.Error(err))
		return nil, nil, err
	}

	// If devices have already been allocated for this pod, skip allocation but mount the devices
	if allocation != nil {
		// Convert the allocated RDMA devices to mounts
		mounts := ParseRDMACharDevicesToMounts(allocation.DeviceInfo)
		l.Debug("Pod network has been set,Mounting RDMA devices to container",
			zap.String("containerName", container.Name),
			zap.Int("mountCount", len(mounts)))

		return &api.ContainerAdjustment{
			Mounts: mounts,
		}, nil, nil
	}

	// Continue with device allocation for the first container
	gpus, err := n.getAllocatedGpusForPodSandbox(ctx, sandbox)
	if err != nil {
		l.Error("Failed to get allocated gpus", zap.Error(err))
		return nil, nil, err
	}

	if len(gpus) == 0 {
		// no GPU allocated to this pod
		n.logger.Debug("No GPU resources allocated to this pod, skip init rdma network for the pod",
			zap.String("podName", sandbox.GetName()),
			zap.String("namespace", sandbox.GetNamespace()))
		return nil, nil, nil
	}

	pod := &corev1.Pod{}
	if err := n.client.Get(ctx, client.ObjectKey{Name: sandbox.Name, Namespace: sandbox.Namespace}, pod); err != nil {
		l.Error("Failed to get pod", zap.Error(err))
		return nil, nil, err
	}

	if _, ok := pod.Annotations["dra.spidernet.io/nri"]; !ok {
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
		l.Info("No matched CNI configs with GPU Affinity")
		return nil, nil, nil
	}

	l.Debug("Found Matched CNI configs with GPU Affinity, Start to set pod RDMA network", zap.Any("deviceToCniConfigs", deviceToCniConfigs))
	deviceAllocation, err := n.initPodRdmaNetwork(ctx, l, deviceToCniConfigs, sandbox)
	if err != nil {
		l.Error("Failed to set pod network with gpu affinity", zap.Error(err))
		return nil, nil, err
	}

	// Save the device allocation record
	if err := SaveDeviceAllocation(l, deviceAllocation); err != nil {
		l.Error("Failed to save device allocation", zap.Error(err))
		return nil, nil, err
	}

	return &api.ContainerAdjustment{
		Mounts: ParseRDMACharDevicesToMounts(deviceAllocation.DeviceInfo),
	}, nil, nil
}

func (n *nriPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	l := n.logger.With(zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))
	l.Debug("StopPodSandbox is called")

	allocation, err := GetDeviceAllocation(l, pod.Uid)
	if err != nil || allocation == nil {
		l.Error("Failed to get device allocation", zap.Error(err))
		return nil
	}

	podNetNs, err := n.getPodNetworkNamespace(pod)
	if err != nil {
		l.Error("Failed to get pod network namespace", zap.Error(err))
		return fmt.Errorf("failed to get pod network namespace: %v", err)
	}
	defer podNetNs.Close()

	nhNs, err := netlink.NewHandleAt(podNetNs)
	if err != nil {
		return fmt.Errorf("could not get network namespace handle: %w", err)
	}
	defer nhNs.Close()

	rootNs, err := netns.Get()
	if err != nil {
		return err
	}
	defer rootNs.Close()

	for _, deviceInfo := range allocation.DeviceInfo {
		dev, err := nhNs.RdmaLinkByName(deviceInfo.Iface)
		if err != nil {
			return fmt.Errorf("failed to find %q: %v", deviceInfo.Iface, err)
		}

		if err := nhNs.RdmaLinkSetNsFd(dev, uint32(rootNs)); err != nil {
			return fmt.Errorf("failed to set %q to root network namespace: %v", deviceInfo.Iface, err)
		}
	}

	// Clean up device allocation when pod is removed
	if err := DeleteDeviceAllocation(l, pod.Uid); err != nil {
		l.Error("Failed to delete device allocation", zap.Error(err))
	}
	l.Debug("Successfully cleaned up device allocation")
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

func (n *nriPlugin) initPodRdmaNetwork(ctx context.Context, l *zap.Logger, deviceToCniConfigs map[string]string, pod *api.PodSandbox) (*DeviceAllocation, error) {
	podNetNs, err := n.getPodNetworkNamespace(pod)
	if err != nil {
		l.Error("Failed to get pod network namespace", zap.Error(err))
		return nil, fmt.Errorf("failed to get pod network namespace: %v", err)
	}
	defer podNetNs.Close()

	// Create a device allocation record
	deviceAllocation := &DeviceAllocation{
		PodUID:       pod.Id,
		PodNamespace: pod.Namespace,
		PodName:      pod.Name,
		DeviceInfo:   make([]DeviceInfo, len(deviceToCniConfigs)),
	}

	var netStatus []NetworkStatus
	idx := 1
	for pf, cniConfigName := range deviceToCniConfigs {
		// Inject RDMA device to pod network namespace
		deviceInfo, err := n.allocatedRdmaDeviceToPod(l, pf, podNetNs)
		if err != nil {
			l.Error("Failed to allocate RDMA device to pod network namespace",
				zap.String("pfName", pf), zap.Error(err))
			return nil, err
		}

		var result *cni100.Result
		result, err = n.setupPodNetwork(ctx, l, cniConfigName, buildRuntimeConfig(pod, deviceInfo, idx, podNetNs))
		if err != nil {
			l.Error("Failed to setup pod network", zap.Error(err))
			return nil, err
		}

		status := NetworkStatus{
			Name:       cniConfigName,
			DeviceInfo: &deviceInfo,
		}
		status.parseNetworkStatus(result)
		netStatus = append(netStatus, status)
		idx += 1
	}

	l.Info("Successfully Setup Pod RDMA Network",
		zap.String("podUID", pod.Id), zap.Any("status", netStatus))

	return deviceAllocation, nil
}

// setupPodNetwork sets up the pod network
func (n *nriPlugin) setupPodNetwork(ctx context.Context, l *zap.Logger, cniConfigName string, rc libcni.RuntimeConf) (*cni100.Result, error) {
	// Get NetworkAttachmentDefinition object
	nad := &netv1.NetworkAttachmentDefinition{}
	if err := n.client.Get(ctx, client.ObjectKey{Namespace: n.spiderpoolNamespace, Name: cniConfigName}, nad); err != nil {
		return nil, fmt.Errorf("failed to get NetworkAttachmentDefinition %s/%s: %v", n.spiderpoolNamespace, cniConfigName, err)
	}

	// Get NetworkAttachmentDefinition configuration
	if nad.Spec.Config == "" {
		return nil, fmt.Errorf("NetworkAttachmentDefinition %s/%s has empty config", n.spiderpoolNamespace, nad.Name)
	}

	confList, err := buildSecondaryCniConfig(nad.Name, nad.Spec.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to build CNI config from NetworkAttachmentDefinition %s/%s: %v", n.spiderpoolNamespace, nad.Name, err)
	}

	l.Debug("Got final CNI config, Start invoke CNI ADD", zap.Any("confList", confList))
	result, err := cniAdd(ctx, confList.Bytes, rc)
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
func (n *nriPlugin) getPodNetworkNamespace(pod *api.PodSandbox) (netns.NsHandle, error) {
	// Get the network namespace path of the Pod
	for _, namespace := range pod.Linux.GetNamespaces() {
		if namespace.Type == "network" {
			return netns.GetFromPath(namespace.Path)
		}
	}

	return netns.None(), fmt.Errorf("failed to get network namespace from pod %s", pod.Id)
}

func (n *nriPlugin) allocatedRdmaDeviceToPod(l *zap.Logger, device string, podNetNs netns.NsHandle) (DeviceInfo, error) {
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
		Iface:      vfName,
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

	// Inject RDMA device to pod network namespace
	hostDev, err := netlink.RdmaLinkByName(rdmaDevice)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("failed to get rdma link for network device %s: %v", rdmaDevice, err)
	}

	err = netlink.RdmaLinkSetNsFd(hostDev, uint32(podNetNs))
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("failed to set RDMA device for network device %s: %v", device, err)
	}

	l.Debug("Successfully allocated RDMA devices to pod network namespace")
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
