// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package draController

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/dynamic-resource-allocation/controller"
)

type driver struct {
	spiderClientset clientset.Interface
}

func NewDriver(spiderClientset clientset.Interface) *driver {
	return &driver{spiderClientset: spiderClientset}
}

func (d driver) GetClassParameters(ctx context.Context, class *resourcev1alpha2.ResourceClass) (interface{}, error) {
	return nil, nil
}

func (d driver) GetClaimParameters(ctx context.Context, claim *resourcev1alpha2.ResourceClaim, class *resourcev1alpha2.ResourceClass, classParameters interface{}) (interface{}, error) {
	if claim.Spec.ParametersRef == nil {
		// TODO(@cyclinder): we can give it a default ClaimParameterSpec?
		return &v2beta1.ClaimParameterSpec{}, nil
	}

	if claim.Spec.ParametersRef.APIGroup != constant.SpiderpoolAPIGroup {
		return nil, fmt.Errorf("incorrect API Group: %v", claim.Spec.ParametersRef.APIGroup)
	}

	scp, err := d.spiderClientset.SpiderpoolV2beta1().SpiderClaimParameters(claim.Namespace).Get(ctx, claim.Spec.ParametersRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to getting SpiderClaimParameters %s/%s: %w", claim.Namespace, claim.Spec.ParametersRef.Name, err)
	}

	return scp, nil
}

func (d driver) Allocate(ctx context.Context, cas []*controller.ClaimAllocation, selectedNode string) {
	for _, ca := range cas {
		ca.Allocation, ca.Error = d.allocate(ctx, ca.Claim, ca.ClaimParameters, ca.Class, ca.ClassParameters, selectedNode)
	}
}

func (d driver) allocate(ctx context.Context, claim *resourcev1alpha2.ResourceClaim, claimParameters interface{}, class *resourcev1alpha2.ResourceClass, classParameters interface{}, selectedNode string) (*resourcev1alpha2.AllocationResult, error) {
	if selectedNode == "" {
		return nil, fmt.Errorf("TODO: immediate allocations not yet supported")
	}

	// TODO(@cyclinder): do some checks
	nodeSelector := &v1.NodeSelector{
		NodeSelectorTerms: []v1.NodeSelectorTerm{
			{
				MatchFields: []v1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: "In",
						Values:   []string{selectedNode},
					},
				},
			},
		},
	}

	return &resourcev1alpha2.AllocationResult{
		AvailableOnNodes: nodeSelector,
	}, nil
}

// Deallocate
func (d driver) Deallocate(ctx context.Context, claim *resourcev1alpha2.ResourceClaim) error {
	// TODO(@cyclinder): maybe we need clean the NodeState resource.
	return nil
}

// UnsuitableNodes
func (d driver) UnsuitableNodes(ctx context.Context, pod *v1.Pod, claims []*controller.ClaimAllocation, potentialNodes []string) error {
	// TODO(@cyclinder): we need a new CRD resource like NodeState, dra-plugin check the node's state and
	// update it to the NodeState resource, dra-controller read the NodeState resource and check the node
	// if is unsuitable.
	return nil
}
