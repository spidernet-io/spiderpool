// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"

	stringutil "github.com/spidernet-io/spiderpool/pkg/utils/string"
)

type PodStatus string

type AppNamespacedName struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

type PodTopController struct {
	AppNamespacedName
	UID apitypes.UID
	APP metav1.Object
}

type AnnoPodIPPoolValue struct {
	IPv4Pools []string `json:"ipv4,omitempty"`
	IPv6Pools []string `json:"ipv6,omitempty"`
}

type AnnoPodIPPoolsValue []AnnoIPPoolItem

type AnnoIPPoolItem struct {
	NIC          string   `json:"interface,omitempty"`
	IPv4Pools    []string `json:"ipv4,omitempty"`
	IPv6Pools    []string `json:"ipv6,omitempty"`
	CleanGateway bool     `json:"cleangateway"`
}

type AnnoPodRoutesValue []AnnoRouteItem

type AnnoRouteItem struct {
	Dst string `json:"dst"`
	Gw  string `json:"gw"`
}

type AnnoNSDefautlV4PoolValue []string

type AnnoNSDefautlV6PoolValue []string

type PodSubnetAnnoConfig struct {
	MultipleSubnets []AnnoSubnetItem
	SingleSubnet    *AnnoSubnetItem
	FlexibleIPNum   *int
	AssignIPNum     int
	ReclaimIPPool   bool
}

func (in *PodSubnetAnnoConfig) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&PodSubnetAnnoConfig{`,
		`MultipleSubnets` + fmt.Sprintf("%v", in.MultipleSubnets),
		`SingleSubnet:` + strings.Replace(strings.Replace(in.SingleSubnet.String(), "AnnoSubnetItem", "", 1), `&`, ``, 1) + `,`,
		`FlexibleIPNum:` + stringutil.ValueToStringGenerated(in.FlexibleIPNum) + `,`,
		`AssignIPNumber:` + fmt.Sprintf("%v", in.AssignIPNum) + `,`,
		`ReclaimIPPool:` + fmt.Sprintf("%v", in.ReclaimIPPool),
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

	s := strings.Join([]string{`&AnnoSubnetItem{`,
		`Interface:` + fmt.Sprintf("%v", in.Interface) + `,`,
		`IPv4:` + fmt.Sprintf("%v", in.IPv4) + `,`,
		`IPv6:` + fmt.Sprintf("%v", in.IPv6),
		`}`,
	}, "")
	return s
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
