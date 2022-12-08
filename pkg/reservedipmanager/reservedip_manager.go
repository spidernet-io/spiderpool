// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"fmt"

	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

type ReservedIPManager interface {
	GetReservedIPByName(ctx context.Context, rIPName string) (*spiderpoolv1.SpiderReservedIP, error)
	ListReservedIPs(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderReservedIPList, error)
}

type reservedIPManager struct {
	client client.Client
}

func NewReservedIPManager(client client.Client) (ReservedIPManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}

	return &reservedIPManager{
		client: client,
	}, nil
}

func (rm *reservedIPManager) GetReservedIPByName(ctx context.Context, rIPName string) (*spiderpoolv1.SpiderReservedIP, error) {
	var rIP spiderpoolv1.SpiderReservedIP
	if err := rm.client.Get(ctx, apitypes.NamespacedName{Name: rIPName}, &rIP); err != nil {
		return nil, err
	}

	return &rIP, nil
}

func (rm *reservedIPManager) ListReservedIPs(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderReservedIPList, error) {
	var rIPList spiderpoolv1.SpiderReservedIPList
	if err := rm.client.List(ctx, &rIPList, opts...); err != nil {
		return nil, err
	}

	return &rIPList, nil
}
