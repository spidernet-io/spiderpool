// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"fmt"
	"reflect"
	"strings"
)

func valueToStringGenerated(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}

// String serves for SpiderIPPool
func (in *SpiderIPPool) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&SpiderIPPool{`,
		`ObjectMeta:` + strings.Replace(fmt.Sprintf("%v", in.ObjectMeta), `&`, ``, 1) + `,`,
		`Spec:` + strings.Replace(strings.Replace(in.Spec.String(), "IPPoolSpec", "IPPoolSpec", 1), `&`, ``, 1) + `,`,
		`Status:` + strings.Replace(strings.Replace(in.Status.String(), "IPPoolStatus", "IPPoolStatus", 1), `&`, ``, 1) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderIPPool Spec IPPoolSpec
func (in *IPPoolSpec) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&IPPoolSpec{`,
		`IPVersion:` + valueToStringGenerated(in.IPVersion) + `,`,
		`Subnet:` + fmt.Sprintf("%v", in.Subnet) + `,`,
		`IPs:` + fmt.Sprintf("%v", in.IPs) + `,`,
		`Disable:` + valueToStringGenerated(in.Disable) + `,`,
		`ExcludeIPs:` + fmt.Sprintf("%v", in.ExcludeIPs) + `,`,
		`Gateway:` + valueToStringGenerated(in.Gateway) + `,`,
		`Vlan:` + valueToStringGenerated(in.Vlan) + `,`,
		`Routes:` + fmt.Sprintf("%+v", in.Routes) + `,`,
		`PodAffinity:` + fmt.Sprintf("%v", in.PodAffinity) + `,`,
		`NamespaceAffinity:` + fmt.Sprintf("%v", in.NamespaceAffinity) + `,`,
		`NodeAffinity:` + fmt.Sprintf("%v", in.NodeAffinity) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderIPPool Status IPPoolStatus
func (in *IPPoolStatus) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&IPPoolStatus{`,
		`AllocatedIPs:` + fmt.Sprintf("%+v", in.AllocatedIPs) + `,`,
		`TotalIPCount:` + valueToStringGenerated(in.TotalIPCount) + `,`,
		`AllocatedIPCount:` + valueToStringGenerated(in.AllocatedIPCount) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderEndpoint
func (in *SpiderEndpoint) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&SpiderEndpoint{`,
		`ObjectMeta:` + strings.Replace(fmt.Sprintf("%v", in.ObjectMeta), `&`, ``, 1) + `,`,
		`Status:` + in.Status.String() + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderEndpoint Status WorkloadEndpointStatus
func (in *WorkloadEndpointStatus) String() string {
	if in == nil {
		return "nil"
	}

	repeatedStringForHistory := "[]History{"
	for _, f := range in.History {
		repeatedStringForHistory += strings.Replace(strings.Replace(f.String(), "History", "History", 1), `&`, ``, 1) + ","
	}
	repeatedStringForHistory += "}"

	s := strings.Join([]string{`&WorkloadEndpointStatus{`,
		`Current:` + fmt.Sprintf("%v", in.Current) + `,`,
		`History:` + repeatedStringForHistory + `,`,
		`OwnerControllerType:` + fmt.Sprintf("%v", in.OwnerControllerType) + `,`,
		`OwnerControllerName` + fmt.Sprintf("%v", in.OwnerControllerName) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderEndpoint Status PodIPAllocation
func (in *PodIPAllocation) String() string {
	if in == nil {
		return "nil"
	}

	repeatedStringForIPs := "[]IPs{"
	for _, f := range in.IPs {
		repeatedStringForIPs += strings.Replace(strings.Replace(f.String(), "IPs", "IPs", 1), `&`, ``, 1) + ","
	}
	repeatedStringForIPs += "}"

	s := strings.Join([]string{`&PodIPAllocation{`,
		`ContainerID:` + fmt.Sprintf("%+v", in.ContainerID) + `,`,
		`Node:` + valueToStringGenerated(in.Node) + `,`,
		`IPs:` + repeatedStringForIPs + `,`,
		`CreationTime:` + fmt.Sprintf("%v", in.CreationTime) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderEndpoint Status
func (in *IPAllocationDetail) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&IPAllocationDetail{`,
		`NIC:` + fmt.Sprintf("%v", in.NIC) + `,`,
		`IPv4:` + valueToStringGenerated(in.IPv4) + `,`,
		`IPv6:` + valueToStringGenerated(in.IPv6) + `,`,
		`IPv4Pool:` + valueToStringGenerated(in.IPv4Pool) + `,`,
		`IPv6Pool:` + valueToStringGenerated(in.IPv6Pool) + `,`,
		`Vlan:` + valueToStringGenerated(in.Vlan) + `,`,
		`IPv4Gateway:` + valueToStringGenerated(in.IPv4Gateway) + `,`,
		`IPv6Gateway:` + valueToStringGenerated(in.IPv6Gateway) + `,`,
		`CleanGateway:` + valueToStringGenerated(in.CleanGateway) + `,`,
		`Routes:` + fmt.Sprintf("%+v", in.Routes) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderReservedIP
func (in *SpiderReservedIP) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&SpiderReservedIP{`,
		`ObjectMeta:` + strings.Replace(fmt.Sprintf("%v", in.ObjectMeta), `&`, ``, 1) + `,`,
		`Spec:` + strings.Replace(strings.Replace(in.Spec.String(), "ReservedIPSpec", "ReservedIPSpec", 1), `&`, ``, 1) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderReservedIP Spec
func (in *ReservedIPSpec) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&ReservedIPSpec{`,
		`IPVersion:` + valueToStringGenerated(in.IPVersion) + `,`,
		`IPs:` + fmt.Sprintf("%v", in.IPs) + `,`,
		`}`,
	}, "")
	return s
}
