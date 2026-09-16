// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package types

import (
	"fmt"
	"strings"

	stringutil "github.com/spidernet-io/spiderpool/pkg/utils/string"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

type (
	PodStatus         string
	AppNamespacedName struct {
		APIVersion string
		Kind       string
		Namespace  string
		Name       string
	}
)

type PodTopController struct {
	AppNamespacedName
	UID apitypes.UID
	APP metav1.Object
}
type AnnoPodIPPoolValue struct {
	IPv4Pools []string `json:"ipv4,omitempty"`
	IPv6Pools []string `json:"ipv6,omitempty"`
}
type (
	AnnoPodIPPoolsValue []AnnoIPPoolItem
	AnnoIPPoolItem      struct {
		NIC          string   `json:"interface,omitempty"`
		IPv4Pools    []string `json:"ipv4,omitempty"`
		IPv6Pools    []string `json:"ipv6,omitempty"`
		CleanGateway bool     `json:"cleangateway"`
	}
)

type (
	AnnoPodRoutesValue []AnnoRouteItem
	AnnoRouteItem      struct {
		Dst string `json:"dst"`
		Gw  string `json:"gw"`
	}
)

type (
	AnnoNSDefautlV4PoolValue []string
	AnnoNSDefautlV6PoolValue []string
	PodSubnetAnnoConfig      struct {
		MultipleSubnets []AnnoSubnetItem
		SingleSubnet    *AnnoSubnetItem
		FlexibleIPNum   *int
		AssignIPNum     int
		ReclaimIPPool   bool
	}
)

func (in *PodSubnetAnnoConfig) String() string {
	if in == nil {
		return "nil"
	}
	s := strings.Join([]string{
		`&PodSubnetAnnoConfig{`,
		`MultipleSubnets` + fmt.Sprintf("%v", in.MultipleSubnets) + `,`,
		`SingleSubnet:` + strings.Replace(strings.Replace(in.SingleSubnet.String(), "AnnoSubnetItem", "", 1), `&`, ``, 1) + `,`,
		`FlexibleIPNum:` + stringutil.ValueToStringGenerated(in.FlexibleIPNum) + `,`,
		`AssignIPNumber:` + fmt.Sprintf("%v", in.AssignIPNum) + `,`,
		`ReclaimIPPool:` + fmt.Sprintf("%v", in.ReclaimIPPool) + `,`,
		`}`,
	}, "")
	return s
}

// AnnoSubnetItem describes the SpiderSubnet CR names and NIC
type AnnoSubnetItem struct {
	Interface string   `json:"interface,omitempty"`
	IPv4      []string `json:"ipv4,omitempty"`
	IPv6      []string `json:"ipv6,omitempty"`
}

func (in *AnnoSubnetItem) String() string {
	if in == nil {
		return "nil"
	}
	return fmt.Sprintf(
		"&AnnoSubnetItem{Interface:%v,IPv4:%v,IPv6:%v}",
		in.Interface,
		in.IPv4,
		in.IPv6,
	)
}

// AutoPoolProperty describes Auto-created IPPool's properties
type AutoPoolProperty struct {
	DesiredIPNumber int
	IPVersion       IPVersion
	IsReclaimIPPool bool
	IfName          string
	// AnnoPoolIPNumberVal serves for AutoPool annotation to explain whether it is IP number flexible or fixed.
	AnnoPoolIPNumberVal string
}
type SpiderpoolConfigmapConfig struct {
	EnableIPv4                                    bool                    `yaml:"enableIPv4"`
	EnableIPv6                                    bool                    `yaml:"enableIPv6"`
	TuneSysctlConfig                              bool                    `yaml:"tuneSysctlConfig"`
	EnableStatefulSet                             bool                    `yaml:"enableStatefulSet"`
	EnableKubevirtStaticIP                        bool                    `yaml:"enableKubevirtStaticIP"`
	EnableSpiderSubnet                            bool                    `yaml:"enableSpiderSubnet"`
	EnableAutoPoolForApplication                  bool                    `yaml:"enableAutoPoolForApplication"`
	EnableCleanOutdatedEndpoint                   bool                    `yaml:"enableCleanOutdatedEndpoint"`
	EnableIPConflictDetection                     bool                    `yaml:"enableIPConflictDetection"`
	EnableGatewayDetection                        bool                    `yaml:"enableGatewayDetection"`
	ClusterSubnetAutoPoolDefaultRedundantIPNumber int                     `yaml:"clusterSubnetAutoPoolDefaultRedundantIPNumber"`
	EnableValidatingResourcesDeletedWebhook       bool                    `yaml:"enableValidatingResourcesDeletedWebhook"`
	IpamUnixSocketPath                            string                  `yaml:"ipamUnixSocketPath"`
	PodResourceInjectConfig                       PodResourceInjectConfig `yaml:"podResourceInject"`
	IaaSProviderConfig                            IaaSProviderConfig      `yaml:"iaasNetworkProvider,omitempty"`
	AgentConfig                                   AgentConfig             `yaml:"agent,omitempty"`
}
type PodResourceInjectConfig struct {
	Enabled           bool     `yaml:"enabled"`
	NamespacesExclude []string `yaml:"namespacesExclude"`
	NamespacesInclude []string `yaml:"namespacesInclude"`
}
type IaaSProviderConfig struct {
	// Service locates the IaaS provider Kubernetes Service. If Service.Name
	// is empty, IaaS integration is disabled.
	Service IaaSProviderService `yaml:"service,omitempty"`
	// TLS configures how spiderpool verifies the provider serving
	// certificate (one-way TLS, server auth only).
	TLS                IaaSProviderTLS `yaml:"tls,omitempty"`
	HTTPRequestTimeout string          `yaml:"httpRequestTimeout,omitempty"`
	// ExcludeReportNics lists local physical NIC names (e.g. management or
	// storage NICs) that spiderpool-agent must not report to the Node
	// annotation ipam.spidernet.io/parent-nics.
	ExcludeReportNics []string `yaml:"excludeReportNics,omitempty"`
}

type IaaSProviderService struct {
	Name      string `yaml:"name,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`
	Port      int    `yaml:"port,omitempty"`
}

type IaaSProviderTLS struct {
	// CaFile is the path of the PEM CA bundle (may contain multiple CA
	// certificates) used to verify the provider serving certificate. The
	// file is re-read on every new connection so a refreshed mounted
	// Secret takes effect without restart. If empty, certificate
	// verification is skipped (InsecureSkipVerify) to ease gradual
	// rollout; a warning is logged.
	CaFile string `yaml:"caFile,omitempty"`
}

// Enabled reports whether IaaS provider integration is enabled.
func (c *IaaSProviderConfig) Enabled() bool {
	return c.Service.Name != ""
}

type AgentConfig struct {
	NetworkResourcePlugin NetworkResourcePluginConfig `yaml:"networkResourcePlugin,omitempty"`
}

type NetworkResourcePluginConfig struct {
	Enabled               bool                  `yaml:"enabled"`
	KubeletRootDir        string                `yaml:"kubeletRootDir,omitempty"`
	ResourceAdvertisement ResourceAdvertisement `yaml:"resourceAdvertisement,omitempty"`
}

type ResourceAdvertisement struct {
	SubENI    SubENIAdvertisement    `yaml:"subENI,omitempty"`
	MasterNIC MasterNICAdvertisement `yaml:"masterNIC,omitempty"`
}

type SubENIAdvertisement struct {
	Rules []SubENIRule `yaml:"rules,omitempty"`
}

type SubENIRule struct {
	ResourceName    string        `yaml:"resourceName,omitempty"`
	DefaultMaxCount int           `yaml:"defaultMaxCount,omitempty"`
	NodeSelector    LabelSelector `yaml:"nodeSelector,omitempty"`
}

type MasterNICAdvertisement struct {
	Rules []MasterNICRule `yaml:"rules,omitempty"`
}

type MasterNICRule struct {
	NodeSelector      LabelSelector `yaml:"nodeSelector,omitempty"`
	DefaultMaxCount   int           `yaml:"defaultMaxCount,omitempty"`
	IncludeInterfaces []string      `yaml:"includeInterfaces,omitempty"`
	ExcludeInterfaces []string      `yaml:"excludeInterfaces,omitempty"`
}

// LabelSelector mirrors metav1.LabelSelector with yaml tags so that
// gopkg.in/yaml.v3 (used to parse the Spiderpool ConfigMap) decodes the
// camelCase keys correctly. metav1.LabelSelector only carries json tags,
// which yaml.v3 ignores, silently producing an empty match-everything
// selector.
type LabelSelector struct {
	MatchLabels      map[string]string          `yaml:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `yaml:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement mirrors metav1.LabelSelectorRequirement with yaml tags.
type LabelSelectorRequirement struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values,omitempty"`
}

// ToMetav1 converts the yaml-decoded selector to a metav1.LabelSelector.
func (s LabelSelector) ToMetav1() metav1.LabelSelector {
	result := metav1.LabelSelector{}
	if len(s.MatchLabels) > 0 {
		result.MatchLabels = make(map[string]string, len(s.MatchLabels))
		for k, v := range s.MatchLabels {
			result.MatchLabels[k] = v
		}
	}
	for _, req := range s.MatchExpressions {
		result.MatchExpressions = append(result.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      req.Key,
			Operator: metav1.LabelSelectorOperator(req.Operator),
			Values:   append([]string(nil), req.Values...),
		})
	}
	return result
}
