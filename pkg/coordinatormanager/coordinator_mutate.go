// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"
	"fmt"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func mutateCoordinator(ctx context.Context, coord *spiderpoolv2beta1.SpiderCoordinator) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate Coordinator")

	if coord.Spec.Mode == nil {
		coord.Spec.Mode = pointer.String("underlay")
	}
	if coord.Spec.TunePodRoutes == nil {
		coord.Spec.TunePodRoutes = pointer.Bool(true)
	}
	if coord.Spec.HostRuleTable == nil {
		coord.Spec.HostRuleTable = pointer.Int(500)
	}
	if coord.Spec.HostRPFilter == nil {
		coord.Spec.HostRPFilter = pointer.Int(0)
	}
	if coord.Spec.DetectIPConflict == nil {
		coord.Spec.DetectIPConflict = pointer.Bool(false)
	}
	if coord.Spec.DetectGateway == nil {
		coord.Spec.DetectGateway = pointer.Bool(false)
	}

	if coord.DeletionTimestamp != nil {
		logger.Info("Terminating Coordinator, noting to mutate")
		return nil
	}

	if coord.Spec.PodCIDRType == nil {
		return fmt.Errorf("PodCIDRType is not allowed to be empty")
	}

	if !controllerutil.ContainsFinalizer(coord, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(coord, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	return nil
}
