// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (sw *SubnetWebhook) mutateSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate Subnet")

	if subnet.DeletionTimestamp != nil {
		logger.Info("Terminating Subnet, noting to mutate")
		return nil
	}

	if !controllerutil.ContainsFinalizer(subnet, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(subnet, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	if subnet.Spec.IPVersion == nil {
		var version types.IPVersion
		if spiderpoolip.IsIPv4CIDR(subnet.Spec.Subnet) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6CIDR(subnet.Spec.Subnet) {
			version = constant.IPv6
		} else {
			return errors.New("invalid 'spec.ipVersion', noting to mutate")
		}

		subnet.Spec.IPVersion = new(types.IPVersion)
		*subnet.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	if len(subnet.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*subnet.Spec.IPVersion, subnet.Spec.IPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.ips': %v", err)
		}

		subnet.Spec.IPs = mergedIPs
		logger.Sugar().Debugf("Merge 'spec.ips':\n%v\n\nto:\n\n%v", subnet.Spec.IPs, mergedIPs)
	}

	if len(subnet.Spec.ExcludeIPs) > 1 {
		mergedExcludeIPs, err := spiderpoolip.MergeIPRanges(*subnet.Spec.IPVersion, subnet.Spec.ExcludeIPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.excludeIPs': %v", err)
		}

		subnet.Spec.ExcludeIPs = mergedExcludeIPs
		logger.Sugar().Debugf("Merge 'spec.excludeIPs':\n%v\n\nto:\n\n%v", subnet.Spec.ExcludeIPs, mergedExcludeIPs)
	}

	return nil
}
