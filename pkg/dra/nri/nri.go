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

	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

var (
	_ stub.ConfigureInterface = (*nriPlugin)(nil)
	_ stub.RunPodInterface    = (*nriPlugin)(nil)
	_ stub.StopPodInterface   = (*nriPlugin)(nil)
)

type nriPlugin struct {
	gpuResourceNames map[string]struct{}
	logger           *zap.Logger
	nri              stub.Stub
	kubeletClient    podresourcesapi.PodResourcesListerClient
	conn             *grpc.ClientConn
}

func Run(ctx context.Context) error {
	n := &nriPlugin{
		logger:           logutils.Logger.Named("nri"),
		gpuResourceNames: make(map[string]struct{}),
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
	n.logger.Info("RunPodSandbox is called", zap.String("podName", pod.Name), zap.String("namespace", pod.Namespace))
	gpus, err := n.getAllocatedGpusForPodSandbox(ctx, pod)
	if err != nil {
		n.logger.Error(err.Error())
		return err
	}
	n.logger.Info("Allocated GPUs for pod", zap.Strings("gpus", gpus))
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
