// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"net"
	"strings"

	"github.com/spidernet-io/spiderpool/pkg/types"
)

func AssembleTotalIPs(ipVersion types.IPVersion, ipRanges, excludedIPRanges []string) ([]net.IP, error) {
	ips, err := ParseIPRanges(ipVersion, ipRanges)
	if nil != err {
		return nil, err
	}
	excludeIPs, err := ParseIPRanges(ipVersion, excludedIPRanges)
	if nil != err {
		return nil, err
	}
	totalIPs := IPsDiffSet(ips, excludeIPs, false)

	return totalIPs, nil
}

func CIDRToLabelValue(ipVersion types.IPVersion, subnet string) (string, error) {
	if err := IsCIDR(ipVersion, subnet); err != nil {
		return "", err
	}

	value := strings.Replace(subnet, ".", "-", 3)
	value = strings.Replace(value, ":", "-", 7)
	value = strings.Replace(value, "/", "-", 1)

	return value, nil
}
