package nri

import (
	"context"
	"fmt"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
)

var (
	_ stub.ConfigureInterface = (*nriPlugin)(nil)
	_ stub.RunPodInterface    = (*nriPlugin)(nil)
	_ stub.StopPodInterface   = (*nriPlugin)(nil)
)

type nriPlugin struct {
	logger *zap.Logger
	nri    stub.Stub
	ck     ResourceClient
}

func Run(ctx context.Context) error {
	n := &nriPlugin{
		logger: logutils.Logger.Named("nri"),
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

	ck, err := GetResourceClient("")
	if err != nil {
		return err
	}
	n.ck = ck

	go func() {
		if err = n.nri.Run(ctx); err != nil {
			n.logger.Fatal("failed to start nri plugin", zap.Error(err))
		}
	}()

	return nil
}

func (n *nriPlugin) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	n.logger.Info("RunPodSandbox is called", zap.Any("pod", pod))
	// 1. get pod net namespace
	// 2. get multus config
	// 3. get
	gpus, _ := n.getAllocatedGpusForPodSandbox(pod)
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
			api.Event_REMOVE_POD_SANDBOX |
			api.Event_CREATE_CONTAINER), nil
}

func (n *nriPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	n.logger.Info("StopPodSandbox is called", zap.Any("pod", pod))
	return nil
}

func (n *nriPlugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	n.getAllocatedGpusForContainer(container)
	n.logger.Info("CreateContainer is called", zap.Any("container", container))
	return nil, nil, nil
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
