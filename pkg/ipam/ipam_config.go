// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type IPAMConfig struct {
	EnabledStatefulSet       bool
	EnableIPv4               bool
	EnableIPv6               bool
	ClusterDefaultIPv4IPPool []string
	ClusterDefaultIPv6IPPool []string
	LimiterMaxQueueSize      int
	LimiterMaxWaitTime       time.Duration
}

func (c *IPAMConfig) getClusterDefaultPool(ctx context.Context, nic string) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	if len(c.ClusterDefaultIPv4IPPool) == 0 && len(c.ClusterDefaultIPv6IPPool) == 0 {
		return nil, fmt.Errorf("%w, no IPPool selection rules of any kind are specified", constant.ErrNoAvailablePool)
	}
	logger.Info("Use IPPools from cluster default pools")

	return &ToBeAllocated{
		NIC:              nic,
		DefaultRouteType: constant.SingleNICDefaultRoute,
		V4PoolCandidates: c.ClusterDefaultIPv4IPPool,
		V6PoolCandidates: c.ClusterDefaultIPv6IPPool,
	}, nil
}

func (c *IPAMConfig) checkIPVersionEnable(ctx context.Context, tt []*ToBeAllocated) error {
	logger := logutils.FromContext(ctx)

	var version types.IPVersion
	if c.EnableIPv4 && !c.EnableIPv6 {
		logger.Sugar().Infof("IPv4 network")
		version = constant.IPv4
	}
	if !c.EnableIPv4 && c.EnableIPv6 {
		logger.Sugar().Infof("IPv6 network")
		version = constant.IPv6
	}
	if c.EnableIPv4 && c.EnableIPv6 {
		logger.Sugar().Infof("Dual stack network")
		version = constant.Dual
	}

	var errs []error
	for _, t := range tt {
		t.IPVersion = version
		if err := c.checkPoolMisspecified(ctx, t); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w", utilerrors.NewAggregate(errs))
	}

	return nil
}

func (c *IPAMConfig) checkPoolMisspecified(ctx context.Context, t *ToBeAllocated) error {
	v4PoolCount := len(t.V4PoolCandidates)
	v6PoolCount := len(t.V6PoolCandidates)

	if c.EnableIPv4 && v4PoolCount == 0 {
		return fmt.Errorf("%w in interface %s, IPv4 IPPool is not specified when IPv4 is enabled", constant.ErrWrongInput, t.NIC)
	}
	if c.EnableIPv6 && v6PoolCount == 0 {
		return fmt.Errorf("%w in interface %s, IPv6 IPPool is not specified when IPv6 is enabled", constant.ErrWrongInput, t.NIC)
	}
	if !c.EnableIPv4 && v4PoolCount != 0 {
		return fmt.Errorf("%w in interface %s, IPv4 IPPool is specified when IPv4 is disabled", constant.ErrWrongInput, t.NIC)
	}
	if !c.EnableIPv6 && v6PoolCount != 0 {
		return fmt.Errorf("%w in interface %s, IPv6 IPPool is specified when IPv6 is disabled", constant.ErrWrongInput, t.NIC)
	}

	return nil
}
