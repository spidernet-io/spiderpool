// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func mutateCoordinator(ctx context.Context, coord *spiderpoolv2beta1.SpiderCoordinator) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate Coordinator")

	if coord.DeletionTimestamp != nil {
		logger.Info("Terminating Coordinator, noting to mutate")
		return nil
	}

	if !controllerutil.ContainsFinalizer(coord, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(coord, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	return nil
}
