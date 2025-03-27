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

type nriPlugin struct {
	logger *zap.Logger
	nri    stub.Stub
}

func Run(ctx context.Context) error {
	n := nriPlugin{
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
	n.getAllocatedGpusForPodSandbox(pod)
	return nil
}
func (n *nriPlugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return nil
}

func (n *nriPlugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	return nil
}
