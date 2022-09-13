// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

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
