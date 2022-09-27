// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"errors"

	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/election"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
)

type subnetManager struct {
	config        *SubnetManagerConfig
	client        client.Client
	runtimeMgr    ctrl.Manager
	ipPoolManager ippoolmanagertypes.IPPoolManager

	innerCtx context.Context
	leader   election.SpiderLeaseElector
}

func NewSubnetManager(c *SubnetManagerConfig, mgr ctrl.Manager, ipPoolManager ippoolmanagertypes.IPPoolManager) (subnetmanagertypes.SubnetManager, error) {
	if c == nil {
		return nil, errors.New("subnet manager config must be specified")
	}
	if mgr == nil {
		return nil, errors.New("k8s manager must be specified")
	}
	if ipPoolManager == nil {
		return nil, errors.New("ippool manager must be specified")
	}

	return &subnetManager{
		config:        c,
		client:        mgr.GetClient(),
		runtimeMgr:    mgr,
		ipPoolManager: ipPoolManager,
	}, nil
}

func (sm *subnetManager) GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error) {
	var subnet spiderpoolv1.SpiderSubnet
	if err := sm.client.Get(ctx, apitypes.NamespacedName{Name: subnetName}, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

func (sm *subnetManager) ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error) {
	subnetList := &spiderpoolv1.SpiderSubnetList{}
	if err := sm.client.List(ctx, subnetList, opts...); err != nil {
		return nil, err
	}

	return subnetList, nil
}

func (sm *subnetManager) UpdateSubnetStatusOnce(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	if err := sm.client.Status().Update(ctx, subnet); err != nil {
		return err
	}

	return nil
}
