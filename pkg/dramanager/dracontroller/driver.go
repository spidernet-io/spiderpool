package dracontroller

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	"k8s.io/dynamic-resource-allocation/controller"
)

type driver struct {
	spiderClientset clientset.Interface
}

func NewDriver() *driver {
	return &driver{}
}

func (d driver) GetClassParameters(ctx context.Context, class *resourcev1alpha2.ResourceClass) (interface{}, error) {
	return nil, nil
}

func (d driver) GetClaimParameters(ctx context.Context, claim *resourcev1alpha2.ResourceClaim, class *resourcev1alpha2.ResourceClass, classParameters interface{}) (interface{}, error) {
	if claim.Spec.ParametersRef == nil {
		// TODO: we can
		return &v2beta1.ClaimParameterSpec{}, nil
	}

	if claim.Spec.ParametersRef.APIGroup != constant.SpiderpoolAPIGroup {
		return nil, fmt.Errorf("incorrect API Group: %w", claim.Spec.ParametersRef.APIGroup)
	}

	scp, err := d.spiderClientset.SpiderpoolV2beta1().RESTClient().Get()
	return nil, nil
}

func (d driver) Allocate(ctx context.Context, claims []*controller.ClaimAllocation, selectedNode string) {
	return
}

func (d driver) Deallocate(ctx context.Context, claim *resourcev1alpha2.ResourceClaim) error {
	return nil
}

func (d driver) UnsuitableNodes(ctx context.Context, pod *v1.Pod, claims []*controller.ClaimAllocation, potentialNodes []string) error {
	return nil
}
