// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"encoding/json"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func ConvertIPDetailsToIPConfigsAndAllRoutes(details []spiderpoolv2beta1.IPAllocationDetail) ([]*models.IPConfig, []*models.Route) {
	var ips []*models.IPConfig
	var routes []*models.Route
	for _, d := range details {
		nic := d.NIC
		if d.IPv4 != nil {
			version := constant.IPv4
			var ipv4Gateway string
			if d.IPv4Gateway != nil {
				ipv4Gateway = *d.IPv4Gateway
				// don't set default route if we have cleangateway
				if !(d.CleanGateway != nil && *d.CleanGateway) {
					routes = append(routes, genDefaultRoute(nic, ipv4Gateway))
				}
			}
			ips = append(ips, &models.IPConfig{
				Address: d.IPv4,
				Gateway: ipv4Gateway,
				IPPool:  *d.IPv4Pool,
				Nic:     &nic,
				Version: &version,
				Vlan:    *d.Vlan,
			})
		}

		if d.IPv6 != nil {
			version := constant.IPv6
			var ipv6Gateway string
			if d.IPv6Gateway != nil {
				ipv6Gateway = *d.IPv6Gateway
				// don't set default route if we have cleangateway
				if !(d.CleanGateway != nil && *d.CleanGateway) {
					routes = append(routes, genDefaultRoute(nic, ipv6Gateway))
				}
			}
			ips = append(ips, &models.IPConfig{
				Address: d.IPv6,
				Gateway: ipv6Gateway,
				IPPool:  *d.IPv6Pool,
				Nic:     &nic,
				Version: &version,
				Vlan:    *d.Vlan,
			})
		}

		routes = append(routes, ConvertSpecRoutesToOAIRoutes(d.NIC, d.Routes)...)
	}

	return ips, routes
}

func ConvertResultsToIPConfigsAndAllRoutes(results []*types.AllocationResult) ([]*models.IPConfig, []*models.Route) {
	var ips []*models.IPConfig
	var routes []*models.Route
	for _, r := range results {
		ips = append(ips, r.IP)
		routes = append(routes, r.Routes...)

		if r.CleanGateway {
			continue
		}

		if r.IP.Gateway != "" {
			routes = append(routes, genDefaultRoute(*r.IP.Nic, r.IP.Gateway))
		}
	}

	return ips, routes
}

func genDefaultRoute(nic, gateway string) *models.Route {
	var route *models.Route
	if govalidator.IsIPv4(gateway) {
		dst := "0.0.0.0/0"
		route = &models.Route{
			IfName: &nic,
			Dst:    &dst,
			Gw:     &gateway,
		}
	}

	if govalidator.IsIPv6(gateway) {
		dst := "::/0"
		route = &models.Route{
			IfName: &nic,
			Dst:    &dst,
			Gw:     &gateway,
		}
	}

	return route
}

func ConvertResultsToIPDetails(results []*types.AllocationResult, isMultipleNicWithNoName bool) []spiderpoolv2beta1.IPAllocationDetail {
	nicToDetail := map[string]*spiderpoolv2beta1.IPAllocationDetail{}
	for _, r := range results {
		var gateway *string
		var cleanGateway *bool
		if r.IP.Gateway != "" {
			gateway = ptr.To(r.IP.Gateway)
			cleanGateway = ptr.To(r.CleanGateway)
		}

		address := *r.IP.Address
		pool := r.IP.IPPool
		vlan := r.IP.Vlan
		routes := ConvertOAIRoutesToSpecRoutes(r.Routes)

		if d, ok := nicToDetail[*r.IP.Nic]; ok {
			if *r.IP.Version == constant.IPv4 {
				(*d).IPv4 = &address
				(*d).IPv4Pool = &pool
				(*d).IPv4Gateway = gateway
				(*d).Routes = append(d.Routes, routes...)
			} else {
				(*d).IPv6 = r.IP.Address
				(*d).IPv6Pool = &r.IP.IPPool
				(*d).IPv6Gateway = gateway
				(*d).Routes = append(d.Routes, routes...)
			}
			continue
		}

		if *r.IP.Version == constant.IPv4 {
			nicToDetail[*r.IP.Nic] = &spiderpoolv2beta1.IPAllocationDetail{
				NIC:          *r.IP.Nic,
				IPv4:         &address,
				IPv4Pool:     &pool,
				Vlan:         &vlan,
				IPv4Gateway:  gateway,
				CleanGateway: cleanGateway,
				Routes:       routes,
			}
		} else {
			nicToDetail[*r.IP.Nic] = &spiderpoolv2beta1.IPAllocationDetail{
				NIC:          *r.IP.Nic,
				IPv6:         &address,
				IPv6Pool:     &pool,
				Vlan:         &vlan,
				IPv6Gateway:  gateway,
				CleanGateway: cleanGateway,
				Routes:       routes,
			}
		}
	}

	details := []spiderpoolv2beta1.IPAllocationDetail{}
	for _, d := range nicToDetail {
		details = append(details, *d)
	}

	// If no NIC name, sort the results with NIC and specify the first NIC name with "eth0", the others with empty.
	// For the other NIC allocation, we'll retrieve the results from the Endpoint resource and update the Endpoint resource with real NIC name for the no NIC name set.
	if isMultipleNicWithNoName {
		sort.Slice(details, func(i, j int) bool {
			pre, err := strconv.Atoi(details[i].NIC)
			if nil != err {
				return false
			}
			latter, err := strconv.Atoi(details[j].NIC)
			if nil != err {
				return false
			}
			return pre < latter
		})

		for index := range details {
			if index == 0 {
				details[index].NIC = constant.ClusterDefaultInterfaceName
			} else {
				details[index].NIC = ""
			}
		}
	}

	return details
}

func ConvertAnnoPodRoutesToOAIRoutes(annoPodRoutes types.AnnoPodRoutesValue) []*models.Route {
	var routes []*models.Route
	for _, r := range annoPodRoutes {
		dst := r.Dst
		gw := r.Gw
		routes = append(routes, &models.Route{
			IfName: new(string),
			Dst:    &dst,
			Gw:     &gw,
		})
	}

	return routes
}

func ConvertSpecRoutesToOAIRoutes(nic string, specRoutes []spiderpoolv2beta1.Route) []*models.Route {
	var routes []*models.Route
	for _, r := range specRoutes {
		dst := r.Dst
		gw := r.Gw
		routes = append(routes, &models.Route{
			IfName: &nic,
			Dst:    &dst,
			Gw:     &gw,
		})
	}

	return routes
}

func ConvertOAIRoutesToSpecRoutes(oaiRoutes []*models.Route) []spiderpoolv2beta1.Route {
	var routes []spiderpoolv2beta1.Route
	for _, r := range oaiRoutes {
		routes = append(routes, spiderpoolv2beta1.Route{
			Dst: *r.Dst,
			Gw:  *r.Gw,
		})
	}

	return routes
}

func GroupIPAllocationDetails(uid string, details []spiderpoolv2beta1.IPAllocationDetail) types.PoolNameToIPAndUIDs {
	pius := types.PoolNameToIPAndUIDs{}
	for _, d := range details {
		if d.IPv4 != nil {
			pius[*d.IPv4Pool] = append(pius[*d.IPv4Pool], types.IPAndUID{
				IP:  strings.Split(*d.IPv4, "/")[0],
				UID: uid,
			})
		}
		if d.IPv6 != nil {
			pius[*d.IPv6Pool] = append(pius[*d.IPv6Pool], types.IPAndUID{
				IP:  strings.Split(*d.IPv6, "/")[0],
				UID: uid,
			})
		}
	}

	return pius
}

func GenIPConfigResult(allocateIP net.IP, nic string, ipPool *spiderpoolv2beta1.SpiderIPPool) *models.IPConfig {
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
	}
}

func UnmarshalIPPoolAllocatedIPs(data *string) (spiderpoolv2beta1.PoolIPAllocations, error) {
	if data == nil {
		return nil, nil
	}

	var records spiderpoolv2beta1.PoolIPAllocations
	if err := json.Unmarshal([]byte(*data), &records); err != nil {
		return nil, err
	}

	return records, nil
}

func MarshalIPPoolAllocatedIPs(records spiderpoolv2beta1.PoolIPAllocations) (*string, error) {
	if len(records) == 0 {
		return nil, nil
	}

	v, err := json.Marshal(records)
	if err != nil {
		return nil, err
	}
	data := string(v)

	return &data, nil
}

func UnmarshalSubnetAllocatedIPPools(data *string) (spiderpoolv2beta1.PoolIPPreAllocations, error) {
	if data == nil {
		return nil, nil
	}

	var subnetStatusAllocatedIPPool spiderpoolv2beta1.PoolIPPreAllocations
	err := json.Unmarshal([]byte(*data), &subnetStatusAllocatedIPPool)
	if nil != err {
		return nil, err
	}

	return subnetStatusAllocatedIPPool, nil
}

func MarshalSubnetAllocatedIPPools(preAllocations spiderpoolv2beta1.PoolIPPreAllocations) (*string, error) {
	if len(preAllocations) == 0 {
		return nil, nil
	}

	v, err := json.Marshal(preAllocations)
	if err != nil {
		return nil, err
	}
	data := string(v)

	return &data, nil
}
