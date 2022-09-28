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
	PodGraceTimeOut types.PodStatus = "GraceTimeOut"
	PodSucceeded    types.PodStatus = "Succeeded"
	PodFailed       types.PodStatus = "Failed"
	PodEvicted      types.PodStatus = "Evicted"
	PodDeleted      types.PodStatus = "Deleted"
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

const (
	SpiderpoolAgent        = "spiderpool-agent"
	SpiderpoolController   = "spiderpool-controller"
	SpiderpoolAPIGroup     = "spiderpool.spidernet.io"
	SpiderFinalizer        = SpiderpoolAPIGroup
	SpiderpoolAPIVersionV1 = "v1"
	SpiderIPPoolKind       = "SpiderIPPool"
	SpiderEndpointKind     = "SpiderEndpoint"
	SpiderReservedIPKind   = "SpiderReservedIP"
	SpiderSubnetKind       = "SpiderSubnet"
)

const (
	SpiderControllerElectorLockName = SpiderpoolController + "-" + resourcelock.LeasesResourceLock
	QualifiedK8sObjNameFmt          = "[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*"
)

// subnet manager annotation and labels
const (
	AnnoSubnetManagerPrefix = SpiderpoolAPIGroup

	AnnoSubnetManagerV4               = AnnoSubnetManagerPrefix + "/spider-subnet-v4"
	AnnoSubnetManagerV6               = AnnoSubnetManagerPrefix + "/spider-subnet-v6"
	AnnoSubnetManagerAssignIPNumber   = AnnoSubnetManagerPrefix + "/assign-ip-number"
	AnnoSubnetManagerFlexibleIPNumber = AnnoSubnetManagerPrefix + "/flexible-ip-number"
	AnnoSubnetManagerReclaimIPPool    = AnnoSubnetManagerPrefix + "/reclaim-ippool"

	LabelIPPoolOwnerSpiderSubnet   = "owner-spider-subnet"
	LabelIPPoolOwnerApplication    = "owner-application"
	LabelIPPoolOwnerApplicationUID = "owner-application-uid"
	LabelIPPoolVersion             = "ippool-version"
	LabelIPPoolVersionV4           = "v4"
	LabelIPPoolVersionV6           = "v6"
)
