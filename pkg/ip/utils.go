// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"net"

	"github.com/spidernet-io/spiderpool/pkg/types"
)

func AssembleTotalIPs(ipVersion types.IPVersion, ipRanges, excludeIPRanges []string) ([]net.IP, error) {
	ips, err := ParseIPRanges(ipVersion, ipRanges)
	if nil != err {
		return nil, err
	}
	excludeIPs, err := ParseIPRanges(ipVersion, excludeIPRanges)
	if nil != err {
		return nil, err
	}
	totalIPs := IPsDiffSet(ips, excludeIPs)

	return totalIPs, nil
}
