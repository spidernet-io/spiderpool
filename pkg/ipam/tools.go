// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"encoding/json"
	"strings"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func convertToIPConfigs(details []spiderpoolv1.IPAllocationDetail) []*models.IPConfig {
	var ipConfigs []*models.IPConfig
	for _, d := range details {
		var version int64
		if d.IPv4 != nil {
			version = 4
			ipConfigs = append(ipConfigs, &models.IPConfig{
				Address: d.IPv4,
				Nic:     &d.NIC,
				Version: &version,
				Vlan:    int64(*d.Vlan),
				Gateway: *d.IPv4Gateway,
			})
		}

		if d.IPv6 != nil {
			version = 6
			ipConfigs = append(ipConfigs, &models.IPConfig{
				Address: d.IPv6,
				Nic:     &d.NIC,
				Version: &version,
				Vlan:    int64(*d.Vlan),
				Gateway: *d.IPv6Gateway,
			})
		}
	}

	return ipConfigs
}

func convertToIPDetails(ipConfigs []*models.IPConfig) []spiderpoolv1.IPAllocationDetail {
	nicToDetail := map[string]*spiderpoolv1.IPAllocationDetail{}
	for _, c := range ipConfigs {
		if d, ok := nicToDetail[*c.Nic]; ok {
			if *c.Version == 4 {
				d.IPv4 = c.Address
				d.IPv4Pool = &c.IPPool
				d.IPv4Gateway = &c.Gateway
			} else {
				d.IPv6 = c.Address
				d.IPv6Pool = &c.IPPool
				d.IPv6Gateway = &c.Gateway
			}
			continue
		}

		vlan := spiderpoolv1.Vlan(c.Vlan)
		if *c.Version == 4 {
			nicToDetail[*c.Nic] = &spiderpoolv1.IPAllocationDetail{
				NIC:         *c.Nic,
				IPv4:        c.Address,
				IPv4Pool:    &c.IPPool,
				Vlan:        &vlan,
				IPv4Gateway: &c.Gateway,
			}
		} else {
			nicToDetail[*c.Nic] = &spiderpoolv1.IPAllocationDetail{
				NIC:         *c.Nic,
				IPv6:        c.Address,
				IPv6Pool:    &c.IPPool,
				Vlan:        &vlan,
				IPv6Gateway: &c.Gateway,
			}
		}
	}

	details := []spiderpoolv1.IPAllocationDetail{}
	for _, d := range nicToDetail {
		details = append(details, *d)
	}

	return details
}

func groupIPDetails(containerID string, details []spiderpoolv1.IPAllocationDetail) map[string][]ippoolmanager.IPAndCID {
	poolToIPAndCIDs := map[string][]ippoolmanager.IPAndCID{}
	for _, d := range details {
		if d.IPv4 != nil {
			poolToIPAndCIDs[*d.IPv4Pool] = append(poolToIPAndCIDs[*d.IPv4Pool], ippoolmanager.IPAndCID{
				IP:          strings.Split(*d.IPv4, "/")[0],
				ContainerID: containerID,
			})
		}
		if d.IPv6 != nil {
			poolToIPAndCIDs[*d.IPv6Pool] = append(poolToIPAndCIDs[*d.IPv6Pool], ippoolmanager.IPAndCID{
				IP:          strings.Split(*d.IPv6, "/")[0],
				ContainerID: containerID,
			})
		}
	}

	return poolToIPAndCIDs
}

func genIPAssignmentAnnotation(ipConfigs []*models.IPConfig) (map[string]string, error) {
	nicToValue := map[string]types.AnnoPodAssignedEthxValue{}
	for _, c := range ipConfigs {
		if v, ok := nicToValue[*c.Nic]; ok {
			if *c.Version == 4 {
				v.IPv4 = *c.Address
				v.IPv4Pool = c.IPPool
				v.Vlan = int(c.Vlan)
			} else {
				v.IPv6 = *c.Address
				v.IPv6Pool = c.IPPool
				v.Vlan = int(c.Vlan)
			}
			continue
		}

		nicToValue[*c.Nic] = types.AnnoPodAssignedEthxValue{
			NIC: *c.Nic,
		}
	}

	podAnnotations := map[string]string{}
	for nic, anno := range nicToValue {
		b, err := json.Marshal(anno)
		if err != nil {
			return nil, err
		}
		podAnnotations[constant.AnnotationPre+"/assigned-"+nic] = string(b)
	}

	return podAnnotations, nil
}
