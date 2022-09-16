// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"net"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

// AssembleTotalIP will calculate an IPPool CR object usable IPs number,
// it summaries the IPPool IPs then subtracts ExcludeIPs.
// notice: this method would not filter ReservedIP CR object data!
func assembleTotalIPs(ipPool *spiderpoolv1.SpiderIPPool) ([]net.IP, error) {
	ips, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.IPs)
	if nil != err {
		return nil, err
	}
	excludeIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.ExcludeIPs)
	if nil != err {
		return nil, err
	}
	totalIPs := spiderpoolip.IPsDiffSet(ips, excludeIPs)

	return totalIPs, nil
}

func genResIPConfig(allocateIP net.IP, poolSpec *spiderpoolv1.IPPoolSpec, nic, poolName string) (*models.IPConfig, error) {
	ipNet, err := spiderpoolip.ParseIP(*poolSpec.IPVersion, poolSpec.Subnet)
	if err != nil {
		return nil, err
	}
	ipNet.IP = allocateIP
	address := ipNet.String()

	var gateway string
	if poolSpec.Gateway != nil {
		gateway = *poolSpec.Gateway
	}

	return &models.IPConfig{
		Address: &address,
		Gateway: gateway,
		IPPool:  poolName,
		Nic:     &nic,
		Version: poolSpec.IPVersion,
		Vlan:    *poolSpec.Vlan,
	}, nil
}
