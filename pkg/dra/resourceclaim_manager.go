package dra

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceClaimManager interface {
	GetResourceClaim(ctx context.Context, useCache bool, name, namespace string) (*resourcev1beta1.ResourceClaim, error)
	ListResourceClaims(ctx context.Context) (*resourcev1beta1.ResourceClaimList, error)
}

type resourceClaim struct {
	client    client.Client
	apiReader client.Reader
}

func NewResourceClaimManager(client client.Client, apiReader client.Reader) (ResourceClaimManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	return &resourceClaim{
		client:    client,
		apiReader: apiReader,
	}, nil
}

func (rc *resourceClaim) GetResourceClaim(ctx context.Context, useCache bool, name, namespace string) (*resourcev1beta1.ResourceClaim, error) {
	reader := rc.apiReader
	if useCache {
		reader = rc.client
	}

	var resourceClaim resourcev1beta1.ResourceClaim
	if err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &resourceClaim); err != nil {
		return nil, err
	}

	return &resourceClaim, nil
}

func (rc *resourceClaim) ListResourceClaims(ctx context.Context) (*resourcev1beta1.ResourceClaimList, error) {
	return nil, nil
}
