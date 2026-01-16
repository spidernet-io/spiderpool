// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (sw *SubnetWebhook) mutateSubnet(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) error {
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
			return fmt.Errorf("failed to generate 'spec.ipVersion' from 'spec.subnet' %s, nothing to mutate", subnet.Spec.Subnet)
		}

		subnet.Spec.IPVersion = new(types.IPVersion)
		*subnet.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	cidr, err := spiderpoolip.CIDRToLabelValue(*subnet.Spec.IPVersion, subnet.Spec.Subnet)
	if err != nil {
		return fmt.Errorf("failed to parse 'spec.subnet' %s as a valid label value: %w", subnet.Spec.Subnet, err)
	}

	if v, ok := subnet.Labels[constant.LabelSubnetCIDR]; !ok || v != cidr {
		if subnet.Labels == nil {
			subnet.Labels = make(map[string]string)
		}
		subnet.Labels[constant.LabelSubnetCIDR] = cidr
		logger.Sugar().Infof("Set label %s: %s", constant.LabelSubnetCIDR, cidr)
	}

	if len(subnet.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*subnet.Spec.IPVersion, subnet.Spec.IPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.ips': %w", err)
		}

		ips := subnet.Spec.IPs
		subnet.Spec.IPs = mergedIPs
		logger.Sugar().Debugf("Merge 'spec.ips' %v to %v", ips, mergedIPs)
	}

	if len(subnet.Spec.ExcludeIPs) > 1 {
		mergedExcludeIPs, err := spiderpoolip.MergeIPRanges(*subnet.Spec.IPVersion, subnet.Spec.ExcludeIPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.excludeIPs': %w", err)
		}

		excludeIPs := subnet.Spec.ExcludeIPs
		subnet.Spec.ExcludeIPs = mergedExcludeIPs
		logger.Sugar().Debugf("Merge 'spec.excludeIPs' %v to  %v", excludeIPs, mergedExcludeIPs)
	}

	return nil
}
