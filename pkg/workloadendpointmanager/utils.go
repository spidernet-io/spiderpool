// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"strings"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func RetrieveIPAllocation(containerID, nic string, includeHistory bool, we *spiderpoolv1.SpiderEndpoint) (*spiderpoolv1.PodIPAllocation, bool) {
	if we == nil || we.Status.Current == nil {
		return nil, false
	}
	if we.Status.Current.ContainerID != containerID {
		return nil, false
	}
	for _, d := range we.Status.Current.IPs {
		if d.NIC == nic {
			return we.Status.Current, true
		}
	}

	if !includeHistory {
		return nil, false
	}
	if len(we.Status.History) == 0 {
		return nil, false
	}
	for _, a := range we.Status.History[1:] {
		if a.ContainerID != containerID {
			continue
		}
		for _, d := range a.IPs {
			if d.NIC == nic {
				return &a, false
			}
		}
		break
	}

	return nil, false
}

// TODO(iiiceoo): refactor
func mergeIPDetails(target, delta *spiderpoolv1.IPAllocationDetail) {
	if target.IPv4 == nil {
		target.IPv4 = delta.IPv4
	}

	if target.IPv4Pool == nil {
		target.IPv4Pool = delta.IPv4Pool
	}

	if target.IPv4Gateway == nil {
		target.IPv4Gateway = delta.IPv4Gateway
	}

	if target.IPv6 == nil {
		target.IPv6 = delta.IPv6
	}

	if target.IPv6Pool == nil {
		target.IPv6Pool = delta.IPv6Pool
	}

	if target.IPv6Gateway == nil {
		target.IPv6Gateway = delta.IPv6Gateway
	}

	target.Routes = append(target.Routes, delta.Routes...)
}

// ListAllHistoricalIPs collect wep history IPs and classify them with each pool name.
func ListAllHistoricalIPs(se *spiderpoolv1.SpiderEndpoint) map[string][]types.IPAndCID {
	// key: IPPool name
	// value: usedIP and container ID
	wepHistoryIPs := make(map[string][]types.IPAndCID)

	recordHistoryIPs := func(poolName, ipAndCIDR *string, containerID string) {
		if poolName != nil {
			if ipAndCIDR == nil {
				logutils.Logger.Sugar().Errorf("SpiderEndpoint data broken, pod '%s/%s' containerID '%s' used ippool '%s' with no ip",
					se.Namespace, se.Name, containerID, *poolName)

				return
			}

			ip, _, _ := strings.Cut(*ipAndCIDR, "/")

			ips, ok := wepHistoryIPs[*poolName]
			if !ok {
				ips = []types.IPAndCID{{IP: ip, ContainerID: containerID}}
			} else {
				ips = append(ips, types.IPAndCID{IP: ip, ContainerID: containerID})
			}
			wepHistoryIPs[*poolName] = ips
		}
	}

	// circle to traverse each allocation
	for _, PodIPAllocation := range se.Status.History {
		// circle to traverse each NIC
		for _, ipAllocationDetail := range PodIPAllocation.IPs {
			// collect IPv4
			recordHistoryIPs(ipAllocationDetail.IPv4Pool, ipAllocationDetail.IPv4, PodIPAllocation.ContainerID)

			// collect IPv6
			recordHistoryIPs(ipAllocationDetail.IPv6Pool, ipAllocationDetail.IPv6, PodIPAllocation.ContainerID)
		}
	}

	return wepHistoryIPs
}
