// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"fmt"
	"net"
	"strconv"

	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type ReservedIPManager interface {
	GetReservedIPByName(ctx context.Context, rIPName string) (*spiderpoolv1.SpiderReservedIP, error)
	ListReservedIPs(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderReservedIPList, error)
	AssembleReservedIPs(ctx context.Context, version types.IPVersion) ([]net.IP, error)
}

type reservedIPManager struct {
	client client.Client
}

func NewReservedIPManager(client client.Client) (ReservedIPManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}

	return &reservedIPManager{
		client: client,
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

func (rm *reservedIPManager) AssembleReservedIPs(ctx context.Context, version types.IPVersion) ([]net.IP, error) {
	if err := spiderpoolip.IsIPVersion(version); err != nil {
		return nil, err
	}

	rIPList, err := rm.ListReservedIPs(ctx, client.MatchingFields{"spec.ipVersion": strconv.FormatInt(version, 10)})
	if err != nil {
		return nil, err
	}

	var ranges []string
	for _, r := range rIPList.Items {
		if r.DeletionTimestamp == nil {
			ranges = append(ranges, r.Spec.IPs...)
		}
	}

	ips, err := spiderpoolip.ParseIPRanges(version, ranges)
	if err != nil {
		return nil, err
	}

	return ips, nil
}
