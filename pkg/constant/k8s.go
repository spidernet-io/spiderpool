// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

const (
	KindPod         = "Pod"
	KindDeployment  = "Deployment"
	KindStatefulSet = "StatefulSet"
	KindDaemonSet   = "DaemonSet"
	KindUnknown     = "Unknown"
	KindReplicaSet  = "ReplicaSet"
	KindJob         = "Job"
	KindCronJob     = "CronJob"
)

var K8sKinds = []string{KindPod, KindDeployment, KindReplicaSet, KindDaemonSet, KindStatefulSet, KindJob, KindCronJob}
var K8sAPIVersions = []string{corev1.SchemeGroupVersion.String(), appsv1.SchemeGroupVersion.String(), batchv1.SchemeGroupVersion.String()}

const (
	PodRunning     types.PodStatus = "Running"
	PodTerminating types.PodStatus = "Terminating"
	PodSucceeded   types.PodStatus = "Succeeded"
	PodFailed      types.PodStatus = "Failed"
	PodEvicted     types.PodStatus = "Evicted"
	PodDeleted     types.PodStatus = "Deleted"
	PodUnknown     types.PodStatus = "Unknown"
)

const (
	AnnotationPre = "ipam.spidernet.io"

	AnnoPodIPPool       = AnnotationPre + "/ippool"
	AnnoPodIPPools      = AnnotationPre + "/ippools"
	AnnoPodRoutes       = AnnotationPre + "/routes"
	AnnoPodDNS          = AnnotationPre + "/dns"
	AnnoNSDefautlV4Pool = AnnotationPre + "/default-ipv4-ippool"
	AnnoNSDefautlV6Pool = AnnotationPre + "/default-ipv6-ippool"

	// subnet manager annotation and labels
	AnnoSpiderSubnet              = AnnotationPre + "/subnet"
	AnnoSpiderSubnets             = AnnotationPre + "/subnets"
	AnnoSpiderSubnetPoolIPNumber  = AnnotationPre + "/ippool-ip-number"
	AnnoSpiderSubnetReclaimIPPool = AnnotationPre + "/ippool-reclaim"
	AnnoSpiderSubnetPoolApp       = AnnotationPre + "/application"

	LabelIPPoolOwnerSpiderSubnet   = AnnotationPre + "/owner-spider-subnet"
	LabelIPPoolOwnerApplicationUID = AnnotationPre + "/owner-application-uid"
	LabelIPPoolReclaimIPPool       = AnnoSpiderSubnetReclaimIPPool

	LabelSubnetCIDR = AnnotationPre + "/subnet-cidr"
	LabelIPPoolCIDR = AnnotationPre + "/ippool-cidr"
)

const (
	Spiderpool           = "spiderpool"
	SpiderpoolAgent      = "spiderpool-agent"
	SpiderpoolController = "spiderpool-controller"
)

const (
	SpiderFinalizer      = SpiderpoolAPIGroup
	SpiderpoolAPIGroup   = "spiderpool.spidernet.io"
	SpiderpoolAPIVersion = "v2beta1"
	KindSpiderSubnet     = "SpiderSubnet"
	KindSpiderIPPool     = "SpiderIPPool"
	KindSpiderEndpoint   = "SpiderEndpoint"
	KindSpiderReservedIP = "SpiderReservedIP"
)

const (
	UseCache    = true
	IgnoreCache = false
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
