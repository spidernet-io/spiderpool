// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

type PodStatus string

type AnnoPodIPPoolValue struct {
	NIC       *string  `json:"interface,omitempty"`
	IPv4Pools []string `json:"ipv4pools,omitempty"`
	IPv6Pools []string `json:"ipv6pools,omitempty"`
}

type AnnoPodIPPoolsValue []AnnoIPPoolItem

type AnnoIPPoolItem struct {
	NIC          string   `json:"interface"`
	IPv4Pools    []string `json:"ipv4pools,omitempty"`
	IPv6Pools    []string `json:"ipv6pools,omitempty"`
	CleanGateway bool     `json:"cleanGateway"`
}

type AnnoPodRoutesValue []AnnoRouteItem

type AnnoRouteItem struct {
	Dst string `json:"dst"`
	Gw  string `json:"gw"`
}

type AnnoPodAssignedEthxValue struct {
	NIC      string `json:"interface"`
	IPv4Pool string `json:"ipv4pool"`
	IPv6Pool string `json:"ipv6pool"`
	IPv4     string `json:"ipv4"`
	IPv6     string `json:"ipv6"`
	Vlan     int64  `json:"vlan"`
}

type AnnoNSDefautlV4PoolValue []string

type AnnoNSDefautlV6PoolValue []string
