// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

func (ew *WorkloadEndpointWebhook) mutateWorkloadEndpoint(ctx context.Context, endpoint *spiderpoolv1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate Endpoint")

	if endpoint.DeletionTimestamp != nil {
		logger.Info("Terminating Endpoint, noting to mutate")
		return nil
	}

	if !controllerutil.ContainsFinalizer(endpoint, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(endpoint, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	return nil
}
