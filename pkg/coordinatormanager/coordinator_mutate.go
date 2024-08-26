// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	coordinator_cmd "github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func mutateCoordinator(ctx context.Context, coord *spiderpoolv2beta1.SpiderCoordinator) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate Coordinator")

	if coord.Spec.Mode == nil {
		coord.Spec.Mode = ptr.To(string(coordinator_cmd.ModeAuto))
	}
	if coord.Spec.TunePodRoutes == nil {
		coord.Spec.TunePodRoutes = ptr.To(true)
	}
	if coord.Spec.HostRuleTable == nil {
		coord.Spec.HostRuleTable = ptr.To(500)
	}
	if coord.Spec.HostRPFilter == nil {
		coord.Spec.HostRPFilter = ptr.To(0)
	}
	if coord.Spec.PodRPFilter == nil {
		coord.Spec.PodRPFilter = ptr.To(0)
	}
	if coord.Spec.DetectIPConflict == nil {
		coord.Spec.DetectIPConflict = ptr.To(false)
	}
	if coord.Spec.DetectGateway == nil {
		coord.Spec.DetectGateway = ptr.To(false)
	}

	if coord.Spec.TxQueueLen == nil {
		coord.Spec.TxQueueLen = ptr.To(0)
	}

	if coord.DeletionTimestamp != nil {
		logger.Info("Terminating Coordinator, noting to mutate")
		return nil
	}

	if coord.Spec.PodCIDRType == nil {
		return fmt.Errorf("PodCIDRType is not allowed to be empty")
	}

	for idx, cidr := range coord.Spec.HijackCIDR {
		if !strings.Contains(cidr, "/") {
			nAddr, err := netip.ParseAddr(cidr)
			if err != nil {
				return fmt.Errorf("invalid IP address: %v", cidr)
			}

			if nAddr.Is4() {
				coord.Spec.HijackCIDR[idx] = fmt.Sprintf("%s/%d", cidr, 32)
			} else if nAddr.Is6() {
				coord.Spec.HijackCIDR[idx] = fmt.Sprintf("%s/%d", cidr, 128)
			}
		}
	}

	if !controllerutil.ContainsFinalizer(coord, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(coord, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	return nil
}
