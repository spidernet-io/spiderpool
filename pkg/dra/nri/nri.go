package nri

import (
	"context"
	"fmt"
	"os"

	"github.com/Mellanox/rdmamap"
	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go.uber.org/zap"

	"google.golang.org/grpc"

	_ "github.com/containernetworking/cni/libcni"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

var (
	_ stub.ConfigureInterface       = (*nriPlugin)(nil)
	_ stub.StopPodInterface         = (*nriPlugin)(nil)
	_ stub.CreateContainerInterface = (*nriPlugin)(nil)
)

type nriPlugin struct {
	nodeName         string
	gpuResourceNames map[string]struct{}
	logger           *zap.Logger
	nri              stub.Stub
	kubeletClient    podresourcesapi.PodResourcesListerClient
	conn             *grpc.ClientConn
	clientSet        *kubernetes.Clientset
}

func Run(ctx context.Context, clientSet *kubernetes.Clientset, nodeName string) error {
	n := &nriPlugin{
		nodeName:         nodeName,
		logger:           logutils.Logger.Named("nri"),
		gpuResourceNames: make(map[string]struct{}),
		clientSet:        clientSet,
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

func (n *nriPlugin) Configure(ctx context.Context, config, runtime, version string) (api.EventMask, error) {
	n.logger.Info("Configure is called",
		zap.String("config", config),
		zap.String("runtime", runtime),
		zap.String("version", version))

	return api.EventMask(
		api.Event_CREATE_CONTAINER |
			api.Event_STOP_POD_SANDBOX |
			api.Event_REMOVE_POD_SANDBOX), nil
}

func (n *nriPlugin) CreateContainer(ctx context.Context, sandbox *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	l := n.logger.With(zap.String("podName", sandbox.Name), zap.String("namespace", sandbox.Namespace))
	l.Debug("CreateContainer is called", zap.String("containerName", container.Name))

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
		mounts := ParseRDMACharDevicesToMounts(allocation.VFToRDMACharDevices)
		l.Debug("Pod network has been set,Mounting RDMA devices to container",
			zap.String("containerName", container.Name),
			zap.Int("mountCount", len(mounts)))

		return &api.ContainerAdjustment{
			Mounts: mounts,
		}, nil, nil
	}

	l.Debug("Pod network has not been set, continue with device allocation for the first container")
	// Continue with device allocation for the first container
	gpus, err := n.getAllocatedGpusForPodSandbox(ctx, sandbox)
	if err != nil {
		l.Error("Failed to get allocated gpus", zap.Error(err))
		return nil, nil, err
	}

	if len(gpus) == 0 {
		// no GPU allocated to this pod
		return nil, nil, nil
	}

	pod, err := n.clientSet.CoreV1().Pods(sandbox.Namespace).Get(ctx, sandbox.Name, metav1.GetOptions{})
	if err != nil {
		l.Error("Failed to get pod", zap.Error(err))
		return nil, nil, err
	}

	var resourceClaimName string
	for _, rc := range pod.Spec.ResourceClaims {
		if rc.ResourceClaimTemplateName != nil && *rc.ResourceClaimTemplateName != "" {
			resourceClaimName = *rc.ResourceClaimTemplateName
		}
		if rc.ResourceClaimName != nil && *rc.ResourceClaimName != "" {
			resourceClaimName = *rc.ResourceClaimName
		}
	}

	if resourceClaimName == "" {
		// no resource claim allocated to this pod
		return nil, nil, nil
	}

	rct, err := n.clientSet.ResourceV1beta1().ResourceClaimTemplates(pod.Namespace).Get(ctx, resourceClaimName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	isContinue := false
	for _, req := range rct.Spec.Spec.Devices.Requests {
		if req.DeviceClassName == constant.DRANRIDeviceClass {
			isContinue = true
			break
		}
	}

	if !isContinue {
		return nil, nil, nil
	}

	resourceSlice, err := n.getResourceSliceByNode(ctx)
	if err != nil {
		n.logger.Error("Failed to get resource slice", zap.Error(err))
		return nil, nil, err
	}

	deviceToCniConfigs := filterCniConfigsWithGpuRdmaAffinity(gpus, resourceSlice)
	if len(deviceToCniConfigs) == 0 {
		l.Info("No matched CNI configs with GPU Affinity")
		return nil, nil, nil
	}

	l.Debug("Found Matched CNI configs with GPU Affinity, Start to set pod network", zap.Any("deviceToCniConfigs", deviceToCniConfigs))
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
		Mounts: ParseRDMACharDevicesToMounts(deviceAllocation.VFToRDMACharDevices),
	}, nil, nil
}

func (n *nriPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	n.logger.Info("StopPodSandbox is called", zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))
	return nil
}

func (n *nriPlugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	l := n.logger.With(zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))

	// Clean up device allocation when pod is removed
	if err := DeleteDeviceAllocation(l, string(pod.Id)); err != nil {
		l.Error("Failed to delete device allocation", zap.Error(err))
		// Continue even if we fail to delete the allocation record
	}

	return nil
}

func (n *nriPlugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (n *nriPlugin) Shutdown(_ context.Context) {
	n.logger.Info("NRI plugin shutting down...")
}

func (n *nriPlugin) initPodRdmaNetwork(ctx context.Context, l *zap.Logger, deviceToCniConfigs map[string]string, pod *api.PodSandbox) (*DeviceAllocation, error) {
	// Check RDMA namespace mode, ensure it is "exclusive" mode
	rdmaNsMode, err := netlink.RdmaSystemGetNetnsMode()
	if err != nil {
		l.Error("Failed to get RDMA namespace mode", zap.Error(err))
		return nil, fmt.Errorf("failed to get RDMA namespace mode: %v", err)
	}
	if rdmaNsMode != "exclusive" {
		l.Error("RDMA namespace mode is not set to exclusive", zap.String("current_mode", rdmaNsMode))
		return nil, fmt.Errorf("RDMA namespace mode is not set to exclusive, current mode: %s", rdmaNsMode)
	}

	podNetNs, err := n.getPodNetworkNamespace(pod)
	if err != nil {
		l.Error("Failed to get pod network namespace", zap.Error(err))
		return nil, fmt.Errorf("failed to get pod network namespace: %v", err)
	}
	defer podNetNs.Close()

	// Create a device allocation record
	deviceAllocation := &DeviceAllocation{
		PodUID:              pod.Id,
		PodNamespace:        pod.Namespace,
		PodName:             pod.Name,
		VFToRDMACharDevices: make(map[string][]string, len(deviceToCniConfigs)),
	}

	for vf := range deviceToCniConfigs {
		// Inject RDMA device to pod network namespace
		if err := n.allocatedRdmaDeviceToPod(l, vf, podNetNs, deviceAllocation); err != nil {
			l.Error("Failed to inject RDMA device to pod network namespace",
				zap.String("vfName", vf), zap.Error(err))
			return nil, err
		}

		if err := n.setupPodNetwork(ctx, l, deviceToCniConfigs); err != nil {
			l.Error("Failed to setup pod network", zap.Error(err))
			return nil, err
		}
	}

	l.Info("Successfully Setup Pod RDMA Network",
		zap.String("podUID", string(pod.Id)))

	return deviceAllocation, nil
}

func (n *nriPlugin) setupPodNetwork(ctx context.Context, l *zap.Logger, deviceToCniConfigs map[string]string) error {
	/*
		1. Get NetworkAttachement CRD by the cniConfigName, read its configs and convert the config to cni struct
		2. inject the vf deviceId to stdin args and call CNI
		3.
	*/

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

// moveDeviceToPodNetNs moves a device to the Pod's network namespace
func (n *nriPlugin) moveDeviceToPodNetNs(devicePath string, podNetNs netns.NsHandle) error {
	// Get the device's major and minor numbers
	stat, err := os.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("failed to stat device %s: %v", devicePath, err)
	}

	// Get the device's permissions
	mode := stat.Mode()
	if (mode & os.ModeDevice) == 0 {
		return fmt.Errorf("%s is not a device", devicePath)
	}

	// Get the device's system interface
	sys := stat.Sys()
	if sys == nil {
		return fmt.Errorf("failed to get system interface for device %s", devicePath)
	}

	// Create the device in the Pod's network namespace
	// Note: This is a simplified implementation. In practice, we would need to use
	// the mknod system call to create the device in the Pod's network namespace.
	// Since this requires privileged operations, we assume the device is already
	// visible in the Pod's network namespace.

	return nil
}

func (n *nriPlugin) allocatedRdmaDeviceToPod(l *zap.Logger, device string, podNetNs netns.NsHandle, deviceAllocation *DeviceAllocation) error {
	l.Debug("Injecting RDMA device to pod network namespace")
	// Get a VF from the device
	vfDevices, err := networking.GetSriovAvailableVfPciAddressesForNetDev(device)
	if err != nil {
		l.Error("Failed to get VFs from device", zap.String("device", device), zap.Error(err))
		return fmt.Errorf("failed to get VFs from device %s: %v", device, err)
	}

	l.Debug("Found Available VFs for device, Allocated the first VF to pod", zap.String("device", device), zap.String("vf", vfDevices[0]))

	// Add the VF to the allocation record
	deviceAllocation.VFToRDMACharDevices[vfDevices[0]] = []string{}

	// Inject RDMA device for the VF
	rdmaDevice, err := rdmamap.GetRdmaDeviceForNetdevice(vfDevices[0])
	if err != nil {
		l.Error("Failed to get RDMA device for network device", zap.Error(err))
		return fmt.Errorf("failed to get RDMA device for network device %s: %v", vfDevices[0], err)
	}

	if rdmaDevice != "" {
		// Add the RDMA device to the allocation record
		charRdmaDevices := rdmamap.GetRdmaCharDevices(rdmaDevice)
		deviceAllocation.VFToRDMACharDevices[vfDevices[0]] = append(deviceAllocation.VFToRDMACharDevices[vfDevices[0]], charRdmaDevices...)
	}

	// Inject RDMA device to pod network namespace
	hostDev, err := netlink.RdmaLinkByName(device)
	if err != nil {
		return err
	}

	err = netlink.RdmaLinkSetNsFd(hostDev, uint32(podNetNs))
	if err != nil {
		return err
	}

	l.Info("Successfully injected RDMA devices to pod network namespace")
	return nil
}

func (n *nriPlugin) getResourceSliceByNode(ctx context.Context) (*resourcev1beta1.ResourceSlice, error) {
	// Use field selectors to filter ResourceSlices by both nodeName and DRADriverName
	fieldSelector := fmt.Sprintf("%s=%s,%s=%s",
		resourcev1beta1.ResourceSliceSelectorNodeName, n.nodeName,
		resourcev1beta1.ResourceSliceSelectorDriver, constant.DRADriverName)

	listOptions := metav1.ListOptions{
		FieldSelector: fieldSelector,
	}

	rsList, err := n.clientSet.ResourceV1beta1().ResourceSlices().List(ctx, listOptions)
	if err != nil {
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
