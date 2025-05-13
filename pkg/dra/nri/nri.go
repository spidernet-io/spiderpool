package nri

import (
	"context"
	"fmt"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"

	"google.golang.org/grpc"

	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

var (
	_ stub.ConfigureInterface = (*nriPlugin)(nil)
	_ stub.RunPodInterface    = (*nriPlugin)(nil)
	_ stub.StopPodInterface   = (*nriPlugin)(nil)
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

func (n *nriPlugin) RunPodSandbox(ctx context.Context, sandbox *api.PodSandbox) error {
	l := n.logger.With(zap.String("podName", sandbox.Name), zap.String("namespace", sandbox.Namespace))
	l.Debug("RunPodSandbox is called")
	gpus, err := n.getAllocatedGpusForPodSandbox(ctx, sandbox)
	if err != nil {
		l.Error("Failed to get allocated gpus", zap.Error(err))
		return err
	}

	if len(gpus) == 0 {
		// no GPU allocated to this pod
		return nil
	}

	pod, err := n.clientSet.CoreV1().Pods(sandbox.Namespace).Get(ctx, sandbox.Name, metav1.GetOptions{})
	if err != nil {
		l.Error("Failed to get pod", zap.Error(err))
		return err
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
		return nil
	}

	rct, err := n.clientSet.ResourceV1beta1().ResourceClaimTemplates(pod.Namespace).Get(ctx, resourceClaimName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	isContinue := false
	for _, req := range rct.Spec.Spec.Devices.Requests {
		if req.DeviceClassName == constant.DRANRIDeviceClass {
			isContinue = true
			break
		}
	}

	if !isContinue {
		return nil
	}

	resourceSlice, err := n.getResourceSliceByNode(ctx)
	if err != nil {
		n.logger.Error("Failed to get resource slice", zap.Error(err))
		return err
	}

	matchCniConfigs := filterCniConfigsWithGpuAffinity(gpus, resourceSlice)
	if len(matchCniConfigs) == 0 {
		l.Info("No matched CNI configs with GPU Affinity")
		return nil
	}

	l.Debug("Found Matched CNI configs with GPU Affinity, Start to set pod network", zap.Strings("CNIConfigs", matchCniConfigs))
	if err := n.dynamicSetPodNetworkWithGpuAffinity(ctx, matchCniConfigs, sandbox); err != nil {
		l.Error("Failed to set pod network with gpu affinity", zap.Error(err))
		return err
	}

	return nil
}

func (n *nriPlugin) Configure(ctx context.Context, config, runtime, version string) (api.EventMask, error) {
	n.logger.Info("Configure is called",
		zap.String("config", config),
		zap.String("runtime", runtime),
		zap.String("version", version))

	return api.EventMask(
		api.Event_RUN_POD_SANDBOX |
			api.Event_STOP_POD_SANDBOX |
			api.Event_REMOVE_POD_SANDBOX), nil
}

func (n *nriPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	n.logger.Info("StopPodSandbox is called", zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))
	return nil
}

func (n *nriPlugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	n.logger.Info("RemovePodSandbox is called", zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))
	return nil
}

func (n *nriPlugin) Synchronize(_ context.Context, pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (n *nriPlugin) Shutdown(_ context.Context) {
	n.logger.Info("NRI plugin shutting down...")
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

func (n *nriPlugin) dynamicSetPodNetworkWithGpuAffinity(ctx context.Context, cniConfigs []string, pod *api.PodSandbox) error {

	return nil
}
