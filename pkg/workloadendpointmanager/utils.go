// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

func RetrieveIPAllocation(uid, nic string, endpoint *spiderpoolv1.SpiderEndpoint) *spiderpoolv1.PodIPAllocation {
	if endpoint == nil {
		return nil
	}
	if endpoint.Status.Current == nil {
		return nil
	}

	if endpoint.Status.Current.UID == uid {
		for _, d := range endpoint.Status.Current.IPs {
			if d.NIC == nic {
				return endpoint.Status.Current
			}
		}
	}

	return nil
}
