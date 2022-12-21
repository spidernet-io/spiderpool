// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/spidernet-io/spiderpool/pkg/types"
)

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
	OwnerNone        string = "None"
	OwnerDeployment  string = "Deployment"
	OwnerStatefulSet string = "StatefulSet"
	OwnerDaemonSet   string = "DaemonSet"
	OwnerUnknown     string = "Unknown"
	OwnerReplicaSet  string = "ReplicaSet"
	OwnerJob         string = "Job"
	OwnerCronJob     string = "CronJob"
)

const (
	PodRunning      types.PodStatus = "Running"
	PodTerminating  types.PodStatus = "Terminating"
	PodGraceTimeout types.PodStatus = "GraceTimeout"
	PodSucceeded    types.PodStatus = "Succeeded"
	PodFailed       types.PodStatus = "Failed"
	PodEvicted      types.PodStatus = "Evicted"
	PodDeleted      types.PodStatus = "Deleted"
	PodUnknown      types.PodStatus = "Unknown"
)

const (
	AnnotationPre = "ipam.spidernet.io"

	AnnoPodIPPool       = AnnotationPre + "/ippool"
	AnnoPodIPPools      = AnnotationPre + "/ippools"
	AnnoPodRoutes       = AnnotationPre + "/routes"
	AnnoPodDNS          = AnnotationPre + "/dns"
	AnnoPodStatus       = AnnotationPre + "/status"
	AnnoNSDefautlV4Pool = AnnotationPre + "/default-ipv4-ippool"
	AnnoNSDefautlV6Pool = AnnotationPre + "/default-ipv6-ippool"

	// subnet manager annotation and labels
	AnnoSpiderSubnet              = AnnotationPre + "/subnet"
	AnnoSpiderSubnets             = AnnotationPre + "/subnets"
	AnnoSpiderSubnetPoolIPNumber  = AnnotationPre + "/ippool-ip-number"
	AnnoSpiderSubnetReclaimIPPool = AnnotationPre + "/ippool-reclaim"

	LabelIPPoolOwnerSpiderSubnet   = AnnotationPre + "/owner-spider-subnet"
	LabelIPPoolOwnerApplication    = AnnotationPre + "/owner-application"
	LabelIPPoolOwnerApplicationUID = AnnotationPre + "/owner-application-uid"
	LabelIPPoolVersion             = AnnotationPre + "/ippool-version"
	LabelIPPoolReclaimIPPool       = AnnoSpiderSubnetReclaimIPPool
	LabelIPPoolInterface           = AnnotationPre + "/interface"
	LabelIPPoolVersionV4           = "IPv4"
	LabelIPPoolVersionV6           = "IPv6"
)

const (
	Spiderpool               = "spiderpool"
	SpiderpoolAgent          = "spiderpool-agent"
	SpiderpoolController     = "spiderpool-controller"
	SpiderpoolAPIGroup       = "spiderpool.spidernet.io"
	SpiderFinalizer          = SpiderpoolAPIGroup
	SpiderpoolAPIVersionV1   = "v1"
	SpiderIPPoolKind         = "SpiderIPPool"
	SpiderEndpointKind       = "SpiderEndpoint"
	SpiderReservedIPKind     = "SpiderReservedIP"
	SpiderSubnetKind         = "SpiderSubnet"
	SpiderIPPoolListKind     = "SpiderIPPoolList"
	SpiderEndpointListKind   = "SpiderEndpointList"
	SpiderReservedIPListKind = "SpiderReservedIPList"
	SpiderSubnetListKind     = "SpiderSubnetList"
)

const (
	SpiderControllerElectorLockName = SpiderpoolController + "-" + resourcelock.LeasesResourceLock
	QualifiedK8sObjNameFmt          = "[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*"
)

const (
	True  = "true"
	False = "false"
)

const (
	EventReasonScaleIPPool  = "ScaleIPPool"
	EventReasonDeleteIPPool = "DeleteIPPool"
	EventReasonResyncSubnet = "ResyncSubnet"
)

const ClusterDefaultInterfaceName = "eth0"
