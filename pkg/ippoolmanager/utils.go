// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"net"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

func genResIPConfig(allocateIP net.IP, nic string, ipPool *spiderpoolv1.SpiderIPPool) *models.IPConfig {
	ipNet, _ := spiderpoolip.ParseIP(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet, true)
	ipNet.IP = allocateIP
	address := ipNet.String()

	var gateway string
	if ipPool.Spec.Gateway != nil {
		gateway = *ipPool.Spec.Gateway
	}

	return &models.IPConfig{
		Address: &address,
		Gateway: gateway,
		IPPool:  ipPool.Name,
		Nic:     &nic,
		Version: ipPool.Spec.IPVersion,
		Vlan:    *ipPool.Spec.Vlan,
	}
}

func ShouldScaleIPPool(pool *spiderpoolv1.SpiderIPPool) bool {
	ips, _ := spiderpoolip.AssembleTotalIPs(*pool.Spec.IPVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)

	if pool.Status.AutoDesiredIPCount != nil {
		if int64(len(ips)) != *pool.Status.AutoDesiredIPCount {
			return true
		}
	}

	return false
}

func IsAutoCreatedIPPool(pool *spiderpoolv1.SpiderIPPool) bool {
	// only the auto-created IPPool owns the label "ipam.spidernet.io/owner-application"
	poolLabels := pool.GetLabels()
	_, ok := poolLabels[constant.LabelIPPoolOwnerApplication]
	return ok
}
