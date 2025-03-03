package dra

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	drapb "k8s.io/kubelet/pkg/apis/dra/v1beta1"
)

const (
	kubeletPluginRegistryPath = "/var/lib/kubelet/plugins_registry"
	kubeletPluginPath         = "/var/lib/kubelet/plugins"
)

type DraDriver struct {
	kubeClient   kubernetes.Interface
	draPlugin    kubeletplugin.DRAPlugin
	podManager   podmanager.PodManager
	claimManager ResourceClaimManager
}

func Start(ctx context.Context, podManager podmanager.PodManager, claimManager ResourceClaimManager) error {

	return nil
}

func (d *DraDriver) NodePrepareResources(ctx context.Context, request *drapb.NodePrepareResourcesRequest) (*drapb.NodePrepareResourcesResponse, error) {

	for _, c := range request.Claims {
		claim, err := d.claimManager.GetResourceClaim(ctx, true, c.Name, c.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get resource claim '%s/%s': %v", c.Namespace, c.Name, err)
		}

		// only case our 
		claim.Spec.
	}
	return nil, nil
}

func (d *DraDriver) NodeUnprepareResources(ctx context.Context, request *drapb.NodeUnprepareResourcesRequest) (*drapb.NodeUnprepareResourcesResponse, error) {
	return nil, nil
}
