// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

type IPAMConfig struct {
	EnableIPv4               bool
	EnableIPv6               bool
	ClusterDefaultIPv4IPPool []string
	ClusterDefaultIPv6IPPool []string

	EnableSpiderSubnet bool
	EnableStatefulSet  bool

	LimiterConfig *limiter.LimiterConfig

	WaitSubnetPoolRetries int
	WaitSubnetPoolTime    time.Duration
}

func (c *IPAMConfig) getClusterDefaultPool(ctx context.Context, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)

	if len(c.ClusterDefaultIPv4IPPool) == 0 && len(c.ClusterDefaultIPv6IPPool) == 0 {
		return nil, fmt.Errorf("%w, no IPPool selection rules of any kind are specified", constant.ErrNoAvailablePool)
	}
	logger.Info("Use IPPools from cluster default pools")

	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}
	if len(c.ClusterDefaultIPv4IPPool) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     c.ClusterDefaultIPv4IPPool,
		})
	}
	if len(c.ClusterDefaultIPv6IPPool) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     c.ClusterDefaultIPv6IPPool,
		})
	}

	return t, nil
}

func (c *IPAMConfig) checkIPVersionEnable(ctx context.Context, tt []*ToBeAllocated) error {
	logger := logutils.FromContext(ctx)

	if c.EnableIPv4 && !c.EnableIPv6 {
		logger.Sugar().Infof("IPv4 network")
	}
	if !c.EnableIPv4 && c.EnableIPv6 {
		logger.Sugar().Infof("IPv6 network")
	}
	if c.EnableIPv4 && c.EnableIPv6 {
		logger.Sugar().Infof("Dual stack network")
	}

	var errs []error
	for _, t := range tt {
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
	var v4Count, v6Count int
	for _, c := range t.PoolCandidates {
		if c.IPVersion == constant.IPv4 {
			v4Count++
		} else if c.IPVersion == constant.IPv6 {
			v6Count++
		}
	}

	if c.EnableIPv4 && v4Count == 0 {
		return fmt.Errorf("%w in interface %s, IPv4 IPPool is not specified when IPv4 is enabled", constant.ErrWrongInput, t.NIC)
	}
	if c.EnableIPv6 && v6Count == 0 {
		return fmt.Errorf("%w in interface %s, IPv6 IPPool is not specified when IPv6 is enabled", constant.ErrWrongInput, t.NIC)
	}
	if !c.EnableIPv4 && v4Count != 0 {
		return fmt.Errorf("%w in interface %s, IPv4 IPPool is specified when IPv4 is disabled", constant.ErrWrongInput, t.NIC)
	}
	if !c.EnableIPv6 && v6Count != 0 {
		return fmt.Errorf("%w in interface %s, IPv6 IPPool is specified when IPv6 is disabled", constant.ErrWrongInput, t.NIC)
	}

	return nil
}
