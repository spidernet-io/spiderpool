// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func getPoolFromPodAnnoPools(ctx context.Context, anno, nic string) ([]*ToBeAllocated, error) {
	// TODO(iiiceoo): Check Pod annotations
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPools)

	var annoPodIPPools types.AnnoPodIPPoolsValue
	err := json.Unmarshal([]byte(anno), &annoPodIPPools)
	if err != nil {
		return nil, err
	}

	var validIface bool
	for _, v := range annoPodIPPools {
		if v.NIC == nic {
			validIface = true
			break
		}
	}

	if !validIface {
		return nil, fmt.Errorf("the interface of the Pod annotation does not contain that requested by runtime: %w", constant.ErrWrongInput)
	}

	var tt []*ToBeAllocated
	for _, v := range annoPodIPPools {
		var routeType types.DefaultRouteType
		if v.DefaultRoute {
			routeType = constant.MultiNICDefaultRoute
		} else {
			routeType = constant.MultiNICNotDefaultRoute
		}

		tt = append(tt, &ToBeAllocated{
			NIC:              v.NIC,
			DefaultRouteType: routeType,
			V4PoolCandidates: v.IPv4Pools,
			V6PoolCandidates: v.IPv6Pools,
		})
	}

	return tt, nil
}

func getPoolFromPodAnnoPool(ctx context.Context, anno, nic string) (*ToBeAllocated, error) {
	// TODO(iiiceoo): Check Pod annotations
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPool)

	var annoPodIPPool types.AnnoPodIPPoolValue
	if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
		return nil, err
	}

	if annoPodIPPool.NIC != nil && *annoPodIPPool.NIC != nic {
		return nil, fmt.Errorf("the interface of Pod annotation is different from that requested by runtime: %w", constant.ErrWrongInput)
	}

	return &ToBeAllocated{
		NIC:              nic,
		DefaultRouteType: constant.SingleNICDefaultRoute,
		V4PoolCandidates: annoPodIPPool.IPv4Pools,
		V6PoolCandidates: annoPodIPPool.IPv6Pools,
	}, nil
}

func getPoolFromNetConf(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string) *ToBeAllocated {
	logger := logutils.FromContext(ctx)

	var t *ToBeAllocated
	if len(netConfV4Pool) != 0 || len(netConfV6Pool) != 0 {
		logger.Info("Use IPPools from CNI network configuration")
		t = &ToBeAllocated{
			NIC:              nic,
			DefaultRouteType: constant.SingleNICDefaultRoute,
			V4PoolCandidates: netConfV4Pool,
			V6PoolCandidates: netConfV6Pool,
		}
	}

	return t
}

func getCustomRoutes(ctx context.Context, pod *corev1.Pod) ([]*models.Route, error) {
	anno, ok := pod.Annotations[constant.AnnoPodRoutes]
	if !ok {
		return nil, nil
	}

	var annoPodRoutes types.AnnoPodRoutesValue
	err := json.Unmarshal([]byte(anno), &annoPodRoutes)
	if err != nil {
		return nil, err
	}

	return convertAnnoPodRoutesToOAIRoutes(annoPodRoutes), nil
}
