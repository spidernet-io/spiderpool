// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

// Network configurations
const (
	NetworkLegacy = "legacy"
	NetworkStrict = "strict"
	NetworkSDN    = "sdn"

	// For ipam plugin and spiderpool-agent use
	DefaultIPAMUnixSocketPath = "/var/run/spidernet/spiderpool.sock"
)

// Log level character string
const (
	LogDebugLevelStr = "debug"
	LogInfoLevelStr  = "info"
	LogWarnLevelStr  = "warn"
	LogErrorLevelStr = "error"
	LogFatalLevelStr = "fatal"
	LogPanicLevelStr = "panic"
)

type OwnerType string

const (
	OwnerNone         OwnerType = "None"
	OwnerDeployment   OwnerType = "Deployment"
	OwnerStatefuleSet OwnerType = "StatefulSet"
	OwnerDaemonSet    OwnerType = "DaemonSet"
	OwnerCRD          OwnerType = "Unknown"
)

type PodStatus string

const (
	PodRunning     PodStatus = "Running"
	PodTerminating PodStatus = "Terminating"
	PodSucceeded   PodStatus = "Succeeded"
	PodFailed      PodStatus = "Failed"
	PodEvicted     PodStatus = "Evicted"
)

const (
	AnnotationPre       = "ipam.spidernet.io"
	AnnoPodIPPool       = AnnotationPre + "/ippool"
	AnnoPodIPPools      = AnnotationPre + "/ippools"
	AnnoPodRoutes       = AnnotationPre + "/routes"
	AnnoPodDns          = AnnotationPre + "/dns"
	AnnoPodStatus       = AnnotationPre + "/status"
	AnnoNSDefautlV4Pool = AnnotationPre + "/defaultv4ippool"
	AnnoNSDefautlV6Pool = AnnotationPre + "/defaultv6ippool"
)

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
	DefaultRoute bool     `json:"defaultRoute"`
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
	Vlan     string `json:"vlan"`
}

type AnnoNSDefautlV4PoolValue []string

type AnnoNSDefautlV6PoolValue []string
