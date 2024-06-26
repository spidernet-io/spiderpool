// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/kubevirtmanager"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/manager/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

type IPAM interface {
	Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error)
	Release(ctx context.Context, delArgs *models.IpamDelArgs) error
	ReleaseIPs(ctx context.Context, delArgs *models.IpamBatchDelArgs) error
	Start(ctx context.Context) error
}

type ipam struct {
	config      IPAMConfig
	ipamLimiter limiter.Limiter
	failure     *failureCache

	ipPoolManager   ippoolmanager.IPPoolManager
	endpointManager workloadendpointmanager.WorkloadEndpointManager
	nodeManager     nodemanager.NodeManager
	nsManager       namespacemanager.NamespaceManager
	podManager      podmanager.PodManager
	stsManager      statefulsetmanager.StatefulSetManager
	subnetManager   subnetmanager.SubnetManager
	kubevirtManager kubevirtmanager.KubevirtManager
}

func NewIPAM(
	config IPAMConfig,
	ipPoolManager ippoolmanager.IPPoolManager,
	endpointManager workloadendpointmanager.WorkloadEndpointManager,
	nodeManager nodemanager.NodeManager,
	nsManager namespacemanager.NamespaceManager,
	podManager podmanager.PodManager,
	stsManager statefulsetmanager.StatefulSetManager,
	subnetManager subnetmanager.SubnetManager,
	kubevirtManager kubevirtmanager.KubevirtManager,
) (IPAM, error) {
	if ipPoolManager == nil {
		return nil, fmt.Errorf("ippool manager %w", constant.ErrMissingRequiredParam)
	}
	if endpointManager == nil {
		return nil, fmt.Errorf("endpoint manager %w", constant.ErrMissingRequiredParam)
	}
	if nodeManager == nil {
		return nil, fmt.Errorf("node manager %w", constant.ErrMissingRequiredParam)
	}
	if nsManager == nil {
		return nil, fmt.Errorf("namespace manager %w", constant.ErrMissingRequiredParam)
	}
	if podManager == nil {
		return nil, fmt.Errorf("pod manager %w", constant.ErrMissingRequiredParam)
	}
	if stsManager == nil {
		return nil, fmt.Errorf("statefulset manager %w", constant.ErrMissingRequiredParam)
	}
	if config.EnableSpiderSubnet && subnetManager == nil {
		return nil, fmt.Errorf("subnet manager %w", constant.ErrMissingRequiredParam)
	}
	if kubevirtManager == nil {
		return nil, fmt.Errorf("kubevirt manager %w", constant.ErrMissingRequiredParam)
	}

	return &ipam{
		config:          setDefaultsForIPAMConfig(config),
		ipamLimiter:     limiter.NewLimiter(limiter.LimiterConfig{}),
		failure:         newFailureCache(),
		ipPoolManager:   ipPoolManager,
		endpointManager: endpointManager,
		nodeManager:     nodeManager,
		nsManager:       nsManager,
		podManager:      podManager,
		stsManager:      stsManager,
		subnetManager:   subnetManager,
		kubevirtManager: kubevirtManager,
	}, nil
}

func (i *ipam) Start(ctx context.Context) error {
	errCh := make(chan error)
	go func() {
		if err := i.ipamLimiter.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

type failureCache struct {
	l       lock.RWMutex
	entries map[string][]*types.AllocationResult
}

func newFailureCache() *failureCache {
	return &failureCache{
		lock.RWMutex{},
		map[string][]*types.AllocationResult{},
	}
}

func (c *failureCache) addFailureIPs(uid string, results []*types.AllocationResult) {
	c.l.Lock()
	defer c.l.Unlock()

	c.entries[uid] = results
}

func (c *failureCache) rmFailureIPs(uid string) {
	if c.getFailureIPs(uid) == nil {
		return
	}

	c.l.Lock()
	defer c.l.Unlock()

	delete(c.entries, uid)
}

func (c *failureCache) getFailureIPs(uid string) []*types.AllocationResult {
	c.l.RLock()
	defer c.l.RUnlock()

	results, ok := c.entries[uid]
	if !ok {
		return nil
	}

	return results
}
