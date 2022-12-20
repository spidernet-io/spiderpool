// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (rw *ReservedIPWebhook) mutateReservedIP(ctx context.Context, rIP *spiderpoolv1.SpiderReservedIP) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate ReservedIP")

	if rIP.DeletionTimestamp != nil {
		logger.Info("Terminating ReservedIP, noting to mutate")
		return nil
	}

	if len(rIP.Spec.IPs) == 0 {
		return errors.New("empty 'spec.ips', noting to mutate")
	}

	if rIP.Spec.IPVersion == nil {
		var version types.IPVersion
		if spiderpoolip.IsIPv4IPRange(rIP.Spec.IPs[0]) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6IPRange(rIP.Spec.IPs[0]) {
			version = constant.IPv6
		} else {
			return errors.New("invalid 'spec.ipVersion', noting to mutate")
		}

		rIP.Spec.IPVersion = new(types.IPVersion)
		*rIP.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	if len(rIP.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*rIP.Spec.IPVersion, rIP.Spec.IPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.ips': %v", err)
		}

		rIP.Spec.IPs = mergedIPs
		logger.Sugar().Debugf("Merge 'spec.ips':\n%v\n\nto:\n\n%v", rIP.Spec.IPs, mergedIPs)
	}

	return nil
}
