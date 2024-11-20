// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
)

func RetrieveIPAllocation(uid, nic string, endpoint *spiderpoolv2beta1.SpiderEndpoint, isStatic bool) *spiderpoolv2beta1.PodIPAllocation {
	if endpoint == nil {
		return nil
	}

	if endpoint.Status.Current.UID == uid || isStatic {
		// if multiple NIC with no name mode, this slice would be in order sequence.
		// In the first allocation, the spiderpool will allocate all NICs IP addresses and patch them to the SpiderEndpoint status,
		// and we only record the first NIC name with "eth0" in the SpiderEndpoint status, the others NIC name are empty.
		// The latter NICs allocation will retrieve the IP addresses from the NIC and update the SpiderEndpoint status with real NIC name.
		for _, d := range endpoint.Status.Current.IPs {
			if d.NIC == nic || d.NIC == "" {
				return &endpoint.Status.Current
			}
		}
	}

	return nil
}
