// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

type ReservedIPManager interface {
	GetReservedIPRanges(ctx context.Context, version string) ([]string, error)
}

type reservedIPManager struct {
	client client.Client
}

func NewReservedIPManager(c client.Client) (ReservedIPManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}

	return &reservedIPManager{
		client: c,
	}, nil
}

func (r *reservedIPManager) GetReservedIPRanges(ctx context.Context, version string) ([]string, error) {
	var reservedIPs spiderpoolv1.ReservedIPList
	if err := r.client.List(ctx, &reservedIPs, client.MatchingFields{".spec.ipVersion": version}); err != nil {
		return nil, err
	}

	var reservedIPRanges []string
	for _, r := range reservedIPs.Items {
		reservedIPRanges = append(reservedIPRanges, r.Spec.IPs...)
	}

	return reservedIPRanges, nil
}
