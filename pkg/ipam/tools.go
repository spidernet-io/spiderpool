// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func getPoolFromPodAnnoPools(ctx context.Context, anno, nic string) ([]*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPools)

	var annoPodIPPools types.AnnoPodIPPoolsValue
	errPrefix := fmt.Errorf("%w of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodIPPools)
	err := json.Unmarshal([]byte(anno), &annoPodIPPools)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPrefix, err)
	}
	if len(annoPodIPPools) == 0 {
		return nil, fmt.Errorf("%w, value requires at least one item", errPrefix)
	}

	nicSet := map[string]struct{}{}
	for _, v := range annoPodIPPools {
		if v.NIC == "" {
			return nil, fmt.Errorf("%w, interface must be specified", errPrefix)
		}
		if len(v.IPv4Pools) == 0 && len(v.IPv6Pools) == 0 {
			return nil, fmt.Errorf("%w, at least one pool must be specified", errPrefix)
		}
		nicSet[v.NIC] = struct{}{}
	}
	if len(nicSet) < len(annoPodIPPools) {
		return nil, fmt.Errorf("%w, duplicate interface", errPrefix)
	}
	if _, ok := nicSet[nic]; !ok {
		return nil, fmt.Errorf("%w, interfaces do not contain that requested by runtime", errPrefix)
	}

	var tt []*ToBeAllocated
	for _, v := range annoPodIPPools {
		t := &ToBeAllocated{
			NIC:          v.NIC,
			CleanGateway: v.CleanGateway,
		}
		if len(v.IPv4Pools) != 0 {
			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv4,
				Pools:     v.IPv4Pools,
			})
		}
		if len(v.IPv6Pools) != 0 {
			t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
				IPVersion: constant.IPv6,
				Pools:     v.IPv6Pools,
			})
		}
		tt = append(tt, t)
	}

	return tt, nil
}

func getPoolFromPodAnnoPool(ctx context.Context, anno, nic string, cleanGateway bool) (*ToBeAllocated, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Infof("Use IPPools from Pod annotation '%s'", constant.AnnoPodIPPool)

	var annoPodIPPool types.AnnoPodIPPoolValue
	errPrifix := fmt.Errorf("%w of Pod annotation '%s'", constant.ErrWrongInput, constant.AnnoPodIPPool)
	if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
		return nil, fmt.Errorf("%w: %v", errPrifix, err)
	}

	if annoPodIPPool.NIC != nil && *annoPodIPPool.NIC != nic {
		return nil, fmt.Errorf("%w, interface is different from that requested by runtime", errPrifix)
	}

	if len(annoPodIPPool.IPv4Pools) == 0 && len(annoPodIPPool.IPv6Pools) == 0 {
		return nil, fmt.Errorf("%w, at least one pool must be specified", errPrifix)
	}

	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}
	if len(annoPodIPPool.IPv4Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     annoPodIPPool.IPv4Pools,
		})
	}
	if len(annoPodIPPool.IPv6Pools) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     annoPodIPPool.IPv6Pools,
		})
	}

	return t, nil
}

func getPoolFromNetConf(ctx context.Context, nic string, netConfV4Pool, netConfV6Pool []string, cleanGateway bool) *ToBeAllocated {
	logger := logutils.FromContext(ctx)

	if len(netConfV4Pool) == 0 && len(netConfV6Pool) == 0 {
		return nil
	}

	logger.Info("Use IPPools from CNI network configuration")
	t := &ToBeAllocated{
		NIC:          nic,
		CleanGateway: cleanGateway,
	}
	if len(netConfV4Pool) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv4,
			Pools:     netConfV4Pool,
		})
	}
	if len(netConfV6Pool) != 0 {
		t.PoolCandidates = append(t.PoolCandidates, &PoolCandidate{
			IPVersion: constant.IPv6,
			Pools:     netConfV6Pool,
		})
	}

	return t
}

func getCustomRoutes(ctx context.Context, pod *corev1.Pod) ([]*models.Route, error) {
	// TODO(iiiceoo): Check Pod annotations, use pkg ip
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

// TODO(iiiceoo): Refactor
func groupCustomRoutesByGW(ctx context.Context, customRoutes *[]*models.Route, ip *models.IPConfig) ([]*models.Route, error) {
	if len(*customRoutes) == 0 {
		return nil, nil
	}

	_, ipNet, err := net.ParseCIDR(*ip.Address)
	if err != nil {
		return nil, err
	}

	var routes []*models.Route
	for i := 0; i < len(*customRoutes); i++ {
		if ipNet.Contains(net.ParseIP(*(*customRoutes)[i].Gw)) {
			*(*customRoutes)[i].IfName = *ip.Nic
			routes = append(routes, (*customRoutes)[i])
			*customRoutes = append((*customRoutes)[:i], (*customRoutes)[i+1:]...)
			i--
		}
	}

	return routes, nil
}
