// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"net"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

func genResIPConfig(allocateIP net.IP, poolSpec *spiderpoolv1.IPPoolSpec, nic, poolName string) (*models.IPConfig, error) {
	ipNet, err := spiderpoolip.ParseIP(*poolSpec.IPVersion, poolSpec.Subnet, true)
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
