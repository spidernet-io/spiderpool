// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"errors"
	"net"

	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ReservedIPManager interface {
	GetReservedIPByName(ctx context.Context, rIPName string) (*spiderpoolv1.SpiderReservedIP, error)
	ListReservedIPs(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderReservedIPList, error)
	GetReservedIPsByIPVersion(ctx context.Context, version types.IPVersion, rIPList *spiderpoolv1.SpiderReservedIPList) ([]net.IP, error)
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

func (rm *reservedIPManager) GetReservedIPByName(ctx context.Context, rIPName string) (*spiderpoolv1.SpiderReservedIP, error) {
	var rIP spiderpoolv1.SpiderReservedIP
	if err := rm.client.Get(ctx, apitypes.NamespacedName{Name: rIPName}, &rIP); err != nil {
		return nil, err
	}

	return &rIP, nil
}

func (rm *reservedIPManager) ListReservedIPs(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderReservedIPList, error) {
	var rIPList spiderpoolv1.SpiderReservedIPList
	if err := rm.client.List(ctx, &rIPList, opts...); err != nil {
		return nil, err
	}

	return &rIPList, nil
}

func (rm *reservedIPManager) GetReservedIPsByIPVersion(ctx context.Context, version types.IPVersion, rIPList *spiderpoolv1.SpiderReservedIPList) ([]net.IP, error) {
	var ips []net.IP
	for _, r := range rIPList.Items {
		if *r.Spec.IPVersion != version {
			continue
		}

		rIPs, err := spiderpoolip.ParseIPRanges(version, r.Spec.IPs)
		if err != nil {
			return nil, err
		}
		ips = append(ips, rIPs...)
	}

	return ips, nil
}
