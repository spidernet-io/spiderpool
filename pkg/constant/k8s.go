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
	KindKubevirtVM  = "VirtualMachine"
	KindKubevirtVMI = "VirtualMachineInstance"
	KindServiceCIDR = "ServiceCIDR"
)

var K8sKinds = []string{KindPod, KindDeployment, KindReplicaSet, KindDaemonSet, KindStatefulSet, KindJob, KindCronJob}
var K8sAPIVersions = []string{corev1.SchemeGroupVersion.String(), appsv1.SchemeGroupVersion.String(), batchv1.SchemeGroupVersion.String()}
var AutoPoolPodAffinities = []string{AutoPoolPodAffinityAppAPIGroup, AutoPoolPodAffinityAppAPIVersion, AutoPoolPodAffinityAppKind, AutoPoolPodAffinityAppNS, AutoPoolPodAffinityAppName}

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
	// DEPRETED, Maintain backward compatibility, don't remove it.
	// and all new annotations use spidernet.io
	AnnotationPre    = "ipam.spidernet.io"
	CNIAnnotationPre = "cni.spidernet.io"

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

	LabelIPPoolReclaimIPPool             = AnnoSpiderSubnetReclaimIPPool
	LabelIPPoolOwnerSpiderSubnet         = AnnotationPre + "/owner-spider-subnet"
	LabelIPPoolOwnerApplicationGV        = AnnotationPre + "/owner-application-gv"
	LabelIPPoolOwnerApplicationKind      = AnnotationPre + "/owner-application-kind"
	LabelIPPoolOwnerApplicationNamespace = AnnotationPre + "/owner-application-namespace"
	LabelIPPoolOwnerApplicationName      = AnnotationPre + "/owner-application-name"
	LabelIPPoolOwnerApplicationUID       = AnnotationPre + "/owner-application-uid"
	LabelIPPoolInterface                 = AnnotationPre + "/interface"
	LabelIPPoolIPVersion                 = AnnotationPre + "/ip-version"
	LabelValueIPVersionV4                = "IPv4"
	LabelValueIPVersionV6                = "IPv6"

	LabelSubnetCIDR = AnnotationPre + "/subnet-cidr"
	LabelIPPoolCIDR = AnnotationPre + "/ippool-cidr"

	// auto pool special pod affinity matchLabels key
	AutoPoolPodAffinityAppPrefix     = AnnotationPre
	AutoPoolPodAffinityAppAPIGroup   = AutoPoolPodAffinityAppPrefix + "/app-api-group"
	AutoPoolPodAffinityAppAPIVersion = AutoPoolPodAffinityAppPrefix + "/app-api-version"
	AutoPoolPodAffinityAppKind       = AutoPoolPodAffinityAppPrefix + "/app-kind"
	AutoPoolPodAffinityAppNS         = AutoPoolPodAffinityAppPrefix + "/app-namespace"
	AutoPoolPodAffinityAppName       = AutoPoolPodAffinityAppPrefix + "/app-name"

	// SpiderMultusConfig
	MultusConfAnnoPre          = "multus.spidernet.io"
	AnnoNetAttachConfName      = MultusConfAnnoPre + "/cr-name"
	AnnoMultusConfigCNIVersion = MultusConfAnnoPre + "/cni-version"

	// Coordinator
	AnnoDefaultRouteInterface = AnnotationPre + "/default-route-nic"

	//dra
	DraAnnotationPre  = "dra.spidernet.io"
	AnnoDraCdiVersion = AnnotationPre + "/cdi-version"

	// webhook
	PodMutatingWebhookName    = "pods.spiderpool.spidernet.io"
	AnnoPodResourceInject     = CNIAnnotationPre + "/rdma-resource-inject"
	AnnoNetworkResourceInject = CNIAnnotationPre + "/network-resource-inject"
	AnnoCNIConfigName         = CNIAnnotationPre + "/cniconfig-name"
)

const (
	Spiderpool           = "spiderpool"
	SpiderpoolAgent      = "spiderpool-agent"
	SpiderpoolController = "spiderpool-controller"
	Coordinator          = "coordinator"
	Ifacer               = "ifacer"
)

const (
	SpiderFinalizer          = SpiderpoolAPIGroup
	SpiderpoolAPIGroup       = "spiderpool.spidernet.io"
	SpiderpoolAPIVersion     = "v2beta1"
	KindSpiderSubnet         = "SpiderSubnet"
	KindSpiderIPPool         = "SpiderIPPool"
	KindSpiderEndpoint       = "SpiderEndpoint"
	KindSpiderReservedIP     = "SpiderReservedIP"
	KindSpiderCoordinator    = "SpiderCoordinator"
	KindSpiderMultusConfig   = "SpiderMultusConfig"
	KindSpiderClaimParameter = "SpiderClaimParameter"
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

// multus-cni annotation
const (
	MultusDefaultNetAnnot        = "v1.multus-cni.io/default-network"
	MultusNetworkAttachmentAnnot = "k8s.v1.cni.cncf.io/networks"
	MultusNetworkStatus          = "k8s.v1.cni.cncf.io/network-status"
	ResourceNameAnnot            = "k8s.v1.cni.cncf.io/resourceName"
	ResourceNameOvsCniValue      = "ovs-cni.network.kubevirt.io"
)

const (
	MacvlanCNI = "macvlan"
	IPVlanCNI  = "ipvlan"
	SriovCNI   = "sriov"
	IBSriovCNI = "ib-sriov"
	IPoIBCNI   = "ipoib"
	OvsCNI     = "ovs"
	CustomCNI  = "custom"
	TuningCNI  = "tuning"
)

const WebhookMutateRoute = "/webhook-health-check"

// CRD field
const (
	SpecIPVersionField = "spec.ipVersion"
	SpecDefaultField   = "spec.default"
)

const (
	Str4 = "4"
	Str6 = "6"
)

// dra-related
const (
	DRACDIVendor              = "k8s." + DRADriverName
	DRACDIClass               = "claim"
	DRACDIKind                = DRACDIVendor + "/" + DRACDIClass
	DRADriverName             = "dra.spidernet.io"
	DRAPluginRegistrationPath = "/var/lib/kubelet/plugins_registry/" + DRADriverName + ".sock"
	DRADriverPluginPath       = "/var/lib/kubelet/plugins/" + DRADriverName
	DRADriverPluginSocketPath = DRADriverPluginPath + "/plugin.sock"
)

// env

const (
	ENV_SPIDERPOOL_NODENAME = "SPIDERPOOL_NODENAME"
)

// spiderpool cleaning sriov
const (
	SriovNetworkOperatorAPIGroup                 = "sriovnetwork.openshift.io"
	SriovOperatorWebhookConfigMutatingOrValidate = "sriov-operator-webhook-config"
	SriovNetworkResourcesInjectorMutating        = "network-resources-injector-config"
	SriovNetworkOperatorConfigs                  = "sriovoperatorconfigs.sriovnetwork.openshift.io"
)

// webhook Mutating or Validating Configuration
const (
	MutatingWebhookConfiguration   = "MutatingWebhookConfiguration"
	ValidatingWebhookConfiguration = "ValidatingWebhookConfiguration"
)
