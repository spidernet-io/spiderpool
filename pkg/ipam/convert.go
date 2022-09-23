// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"encoding/json"
	"strings"

	"github.com/asaskevich/govalidator"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func convertIPDetailsToIPConfigsAndAllRoutes(details []spiderpoolv1.IPAllocationDetail) ([]*models.IPConfig, []*models.Route) {
	var ips []*models.IPConfig
	var routes []*models.Route
	for _, d := range details {
		nic := d.NIC

		if d.IPv4 != nil {
			version := constant.IPv4
			var ipv4Gateway string
			if d.IPv4Gateway != nil {
				ipv4Gateway = *d.IPv4Gateway
				routes = append(routes, genDefaultRoute(nic, ipv4Gateway))
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
				routes = append(routes, genDefaultRoute(nic, ipv6Gateway))
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

		routes = append(routes, convertSpecRoutesToOAIRoutes(d.NIC, d.Routes)...)
	}

	return ips, routes
}

func convertResultsToIPConfigsAndAllRoutes(results []*AllocationResult) ([]*models.IPConfig, []*models.Route) {
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

func convertResultsToIPDetails(results []*AllocationResult) []spiderpoolv1.IPAllocationDetail {
	nicToDetail := map[string]*spiderpoolv1.IPAllocationDetail{}
	var cleanGateway *bool
	for _, r := range results {
		var gateway *string
		if r.IP.Gateway != "" {
			gateway = new(string)
			*gateway = r.IP.Gateway
			if cleanGateway == nil {
				cleanGateway = new(bool)
				*cleanGateway = r.CleanGateway
			}
		}
		routes := convertOAIRoutesToSpecRoutes(r.Routes)
		if d, ok := nicToDetail[*r.IP.Nic]; ok {
			if *r.IP.Version == constant.IPv4 {
				d.IPv4 = r.IP.Address
				d.IPv4Pool = &r.IP.IPPool
				d.IPv4Gateway = gateway
				d.CleanGateway = cleanGateway
				d.Routes = append(d.Routes, routes...)
			} else {
				d.IPv6 = r.IP.Address
				d.IPv6Pool = &r.IP.IPPool
				d.IPv6Gateway = gateway
				d.CleanGateway = cleanGateway
				d.Routes = append(d.Routes, routes...)
			}
			continue
		}

		if *r.IP.Version == constant.IPv4 {
			nicToDetail[*r.IP.Nic] = &spiderpoolv1.IPAllocationDetail{
				NIC:          *r.IP.Nic,
				IPv4:         r.IP.Address,
				IPv4Pool:     &r.IP.IPPool,
				Vlan:         &r.IP.Vlan,
				IPv4Gateway:  gateway,
				CleanGateway: cleanGateway,
				Routes:       routes,
			}
		} else {
			nicToDetail[*r.IP.Nic] = &spiderpoolv1.IPAllocationDetail{
				NIC:          *r.IP.Nic,
				IPv6:         r.IP.Address,
				IPv6Pool:     &r.IP.IPPool,
				Vlan:         &r.IP.Vlan,
				IPv6Gateway:  gateway,
				CleanGateway: cleanGateway,
				Routes:       routes,
			}
		}
	}

	details := []spiderpoolv1.IPAllocationDetail{}
	for _, d := range nicToDetail {
		details = append(details, *d)
	}

	return details
}

func convertAnnoPodRoutesToOAIRoutes(annoPodRoutes types.AnnoPodRoutesValue) []*models.Route {
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

func convertSpecRoutesToOAIRoutes(nic string, specRoutes []spiderpoolv1.Route) []*models.Route {
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

func convertOAIRoutesToSpecRoutes(oaiRoutes []*models.Route) []spiderpoolv1.Route {
	var routes []spiderpoolv1.Route
	for _, r := range oaiRoutes {
		routes = append(routes, spiderpoolv1.Route{
			Dst: *r.Dst,
			Gw:  *r.Gw,
		})
	}

	return routes
}

func genIPAssignmentAnnotation(ips []*models.IPConfig) (map[string]string, error) {
	nicToValue := map[string]*types.AnnoPodAssignedEthxValue{}
	for _, ip := range ips {
		if v, ok := nicToValue[*ip.Nic]; ok {
			if *ip.Version == constant.IPv4 {
				v.IPv4 = *ip.Address
				v.IPv4Pool = ip.IPPool
				v.Vlan = ip.Vlan
			} else {
				v.IPv6 = *ip.Address
				v.IPv6Pool = ip.IPPool
				v.Vlan = ip.Vlan
			}
			continue
		}

		if *ip.Version == constant.IPv4 {
			nicToValue[*ip.Nic] = &types.AnnoPodAssignedEthxValue{
				NIC:      *ip.Nic,
				IPv4:     *ip.Address,
				IPv4Pool: ip.IPPool,
				Vlan:     ip.Vlan,
			}
		} else {
			nicToValue[*ip.Nic] = &types.AnnoPodAssignedEthxValue{
				NIC:      *ip.Nic,
				IPv6:     *ip.Address,
				IPv6Pool: ip.IPPool,
				Vlan:     ip.Vlan,
			}
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

func GroupIPDetails(containerID string, details []spiderpoolv1.IPAllocationDetail) map[string][]types.IPAndCID {
	poolToIPAndCIDs := map[string][]types.IPAndCID{}
	for _, d := range details {
		if d.IPv4 != nil {
			poolToIPAndCIDs[*d.IPv4Pool] = append(poolToIPAndCIDs[*d.IPv4Pool], types.IPAndCID{
				IP:          strings.Split(*d.IPv4, "/")[0],
				ContainerID: containerID,
			})
		}
		if d.IPv6 != nil {
			poolToIPAndCIDs[*d.IPv6Pool] = append(poolToIPAndCIDs[*d.IPv6Pool], types.IPAndCID{
				IP:          strings.Split(*d.IPv6, "/")[0],
				ContainerID: containerID,
			})
		}
	}

	return poolToIPAndCIDs
}
