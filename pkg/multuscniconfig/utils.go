// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	spiderpoolcmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

const (
	MacVlanType spiderpoolv2beta1.CniType = "macvlan"
	IpVlanType  spiderpoolv2beta1.CniType = "ipvlan"
	SriovType   spiderpoolv2beta1.CniType = "sriov"
	CustomType  spiderpoolv2beta1.CniType = "custom"

	resourceNameAnnot = "k8s.v1.cni.cncf.io/resourceName"

	coordinatorBinName = "coordinattor"
	ifacerBinName      = "ifacer"
)

type MacvlanNetConf struct {
	Type   string                   `json:"type"`
	IPAM   spiderpoolcmd.IPAMConfig `json:"ipam"`
	Master string                   `json:"master"`
	Mode   string                   `json:"mode"`
}

type IPvlanNetConf struct {
	Type   string                   `json:"type"`
	IPAM   spiderpoolcmd.IPAMConfig `json:"ipam"`
	Master string                   `json:"master"`
}

type SRIOVNetConf struct {
	Type         string                   `json:"type"`
	ResourceName string                   `json:"resourceName"` // required
	IPAM         spiderpoolcmd.IPAMConfig `json:"ipam"`
	Vlan         *int                     `json:"vlan,omitempty"`
	DeviceID     string                   `json:"deviceID,omitempty"`
}

type IfacerNetConf struct {
	Type      string   `json:"type"` // Ifacer
	Interface []string `json:"interface"`
	VlanID    int32    `json:"vlanID,omitempty"`
	Bond      *struct {
		Name    string            `json:"name"`
		Mode    int32             `json:"mode"`
		Options map[string]string `json:"options,omitempty"`
	} `json:"bond,omitempty"`
}
