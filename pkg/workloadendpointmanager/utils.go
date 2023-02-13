// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"strings"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func RetrieveIPAllocation(containerID, nic string, endpoint *spiderpoolv1.SpiderEndpoint) *spiderpoolv1.PodIPAllocation {
	if endpoint == nil {
		return nil
	}
	if endpoint.Status.Current == nil {
		return nil
	}

	if endpoint.Status.Current.ContainerID == containerID {
		for _, d := range endpoint.Status.Current.IPs {
			if d.NIC == nic {
				return endpoint.Status.Current
			}
		}
	}

	return nil
}

// ListAllHistoricalIPs collect wep history IPs and classify them with each pool name.
func ListAllHistoricalIPs(endpoint *spiderpoolv1.SpiderEndpoint) map[string][]types.IPAndCID {
	// key: IPPool name
	// value: usedIP and container ID
	wepHistoryIPs := make(map[string][]types.IPAndCID)

	recordHistoryIPs := func(poolName, ipAndCIDR *string, containerID string) {
		if poolName != nil {
			if ipAndCIDR == nil {
				logutils.Logger.Sugar().Errorf("SpiderEndpoint data broken, pod '%s/%s' containerID '%s' used ippool '%s' with no ip",
					endpoint.Namespace, endpoint.Name, containerID, *poolName)

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
	for _, PodIPAllocation := range endpoint.Status.History {
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
