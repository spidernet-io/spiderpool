// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"errors"

	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

type ReservedIPManager interface {
	SetupWebhook() error
	GetReservedIPByName(ctx context.Context, rIPName string) (*spiderpoolv1.SpiderReservedIP, error)
	ListReservedIPs(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderReservedIPList, error)
}

type reservedIPManager struct {
	config     *ReservedIPManagerConfig
	client     client.Client
	runtimeMgr ctrl.Manager
}

func NewReservedIPManager(c *ReservedIPManagerConfig, mgr ctrl.Manager) (ReservedIPManager, error) {
	if c == nil {
		return nil, errors.New("reserved IP manager config must be specified")
	}
	if mgr == nil {
		return nil, errors.New("k8s manager must be specified")
	}

	return &reservedIPManager{
		config:     c,
		client:     mgr.GetClient(),
		runtimeMgr: mgr,
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
