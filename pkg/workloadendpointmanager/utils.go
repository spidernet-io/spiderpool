// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

func RetrieveIPAllocation(uid, nic string, endpoint *spiderpoolv2beta1.SpiderEndpoint, isSTS bool) *spiderpoolv2beta1.PodIPAllocation {
	if endpoint == nil {
		return nil
	}

	if endpoint.Status.Current.UID == uid || isSTS {
		for _, d := range endpoint.Status.Current.IPs {
			if d.NIC == nic {
				return &endpoint.Status.Current
			}
		}
	}

	return nil
}
