// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

var controllersLogger *zap.Logger

type ReconcileAppInformersFunc func(ctx context.Context, oldObj, newObj interface{}) error
type CleanUpAPPInformersFunc func(ctx context.Context, obj interface{}) error

type controller struct {
	reconcileFunc ReconcileAppInformersFunc
	cleanupFunc   CleanUpAPPInformersFunc
}

func NewSubnetController(reconcile ReconcileAppInformersFunc, cleanup CleanUpAPPInformersFunc, logger *zap.Logger) (*controller, error) {
	if reconcile == nil {
		return nil, fmt.Errorf("the controllers informers reconcile function must be specified")
	}
	if cleanup == nil {
		return nil, fmt.Errorf("the controllers informers cleanup function must be specified")
	}

	controllersLogger = logger

	return &controller{reconcileFunc: reconcile, cleanupFunc: cleanup}, nil
}
