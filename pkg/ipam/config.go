// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	// "fmt"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

type IPAMConfig struct {
	EnableIPv4 bool
	EnableIPv6 bool

	EnableSpiderSubnet                   bool
	EnableStatefulSet                    bool
	EnableKubevirtStaticIP               bool
	EnableReleaseConflictIPsForStateless bool
	EnableIPConflictDetection            bool
	EnableGatewayDetection               bool

	OperationRetries     int
	OperationGapDuration time.Duration

	MultusClusterNetwork *string
	AgentNamespace       string
}

func setDefaultsForIPAMConfig(config IPAMConfig) IPAMConfig {
	return config
}

func (c *IPAMConfig) checkIPVersionEnable(ctx context.Context, tt ToBeAllocateds) error {
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
		if err := c.filterPoolMisspecified(ctx, t); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}

	return nil
}

func (c *IPAMConfig) filterPoolMisspecified(ctx context.Context, t *ToBeAllocated) error {
	logger := logutils.FromContext(ctx)

	var v4Count, v6Count int
	var validPoolCandidates []*PoolCandidate
	for _, pc := range t.PoolCandidates {
		if pc.IPVersion == constant.IPv4 && c.EnableIPv4 {
			v4Count++
			validPoolCandidates = append(validPoolCandidates, pc)
		} else if pc.IPVersion == constant.IPv6 && c.EnableIPv6 {
			v6Count++
			validPoolCandidates = append(validPoolCandidates, pc)
		} else {
			logger.Sugar().Debugf("IPv%d is disabled, ignoring to allocate IPv%d IP to NIC %s from IPPool %v", pc.IPVersion, pc.IPVersion, t.NIC, pc.Pools)
		}
	}
	t.PoolCandidates = validPoolCandidates

	// for dual stack environment, support only ipv4 or ipv6 address
	// if c.EnableIPv4 && v4Count == 0 {
	//	return fmt.Errorf("%w, IPv4 is enabled, but no IPv4 IPPool specified for NIC %s", constant.ErrWrongInput, t.NIC)
	// }
	// if c.EnableIPv6 && v6Count == 0 {
	//	return fmt.Errorf("%w, IPv6 is enabled, but no IPv6 IPPool specified for NIC %s", constant.ErrWrongInput, t.NIC)
	// }

	return nil
}
