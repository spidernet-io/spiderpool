// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"fmt"
	"net"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func AssembleReservedIPs(version types.IPVersion, rIPList *spiderpoolv1.SpiderReservedIPList) ([]net.IP, error) {
	if err := spiderpoolip.IsIPVersion(version); err != nil {
		return nil, err
	}

	if rIPList == nil {
		return nil, fmt.Errorf("reserved IP list %w", constant.ErrMissingRequiredParam)
	}

	var ips []net.IP
	for _, r := range rIPList.Items {
		if r.DeletionTimestamp == nil && *r.Spec.IPVersion == version {
			rIPs, err := spiderpoolip.ParseIPRanges(version, r.Spec.IPs)
			if err != nil {
				return nil, err
			}
			ips = append(ips, rIPs...)
		}
	}

	return ips, nil
}
