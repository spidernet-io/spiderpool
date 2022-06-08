// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import "github.com/spidernet-io/spiderpool/pkg/types"

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

const (
	OwnerNone         types.OwnerType = "None"
	OwnerDeployment   types.OwnerType = "Deployment"
	OwnerStatefuleSet types.OwnerType = "StatefulSet"
	OwnerDaemonSet    types.OwnerType = "DaemonSet"
	OwnerCRD          types.OwnerType = "Unknown"
)

const (
	PodRunning     types.PodStatus = "Running"
	PodTerminating types.PodStatus = "Terminating"
	PodSucceeded   types.PodStatus = "Succeeded"
	PodFailed      types.PodStatus = "Failed"
	PodEvicted     types.PodStatus = "Evicted"
)

const (
	AnnotationPre       = "ipam.spidernet.io"
	AnnoPodIPPool       = AnnotationPre + "/ippool"
	AnnoPodIPPools      = AnnotationPre + "/ippools"
	AnnoPodRoutes       = AnnotationPre + "/routes"
	AnnoPodDNS          = AnnotationPre + "/dns"
	AnnoPodStatus       = AnnotationPre + "/status"
	AnnoNSDefautlV4Pool = AnnotationPre + "/defaultv4ippool"
	AnnoNSDefautlV6Pool = AnnotationPre + "/defaultv6ippool"
)
