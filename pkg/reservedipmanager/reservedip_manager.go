// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ReservedIPManager interface {
	GetReservedIPRanges(ctx context.Context, version types.IPVersion) ([]string, error)
}

type reservedIPManager struct {
	client     client.Client
	runtimeMgr ctrl.Manager
}

func NewReservedIPManager(mgr ctrl.Manager) (ReservedIPManager, error) {
	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}

	return &reservedIPManager{
		client:     mgr.GetClient(),
		runtimeMgr: mgr,
	}, nil
}

func (r *reservedIPManager) GetReservedIPRanges(ctx context.Context, version types.IPVersion) ([]string, error) {
	var reservedIPs spiderpoolv1.ReservedIPList
	if err := r.client.List(ctx, &reservedIPs); err != nil {
		return nil, err
	}

	var reservedIPRanges []string
	for _, r := range reservedIPs.Items {
		if *r.Spec.IPVersion == version {
			reservedIPRanges = append(reservedIPRanges, r.Spec.IPs...)
		}
	}

	return reservedIPRanges, nil
}
