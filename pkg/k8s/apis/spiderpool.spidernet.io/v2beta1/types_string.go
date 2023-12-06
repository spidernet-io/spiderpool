// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package v2beta1

import (
	"fmt"
	"strings"

	stringutil "github.com/spidernet-io/spiderpool/pkg/utils/string"
)

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
		`IPVersion:` + stringutil.ValueToStringGenerated(in.IPVersion) + `,`,
		`Subnet:` + fmt.Sprintf("%v", in.Subnet) + `,`,
		`IPs:` + fmt.Sprintf("%v", in.IPs) + `,`,
		`ExcludeIPs:` + fmt.Sprintf("%v", in.ExcludeIPs) + `,`,
		`Gateway:` + stringutil.ValueToStringGenerated(in.Gateway) + `,`,
		`Vlan:` + stringutil.ValueToStringGenerated(in.Vlan) + `,`,
		`Routes:` + fmt.Sprintf("%+v", in.Routes) + `,`,
		`PodAffinity:` + fmt.Sprintf("%v", in.PodAffinity.String()) + `,`,
		`NamespaceAffinity:` + fmt.Sprintf("%v", in.NamespaceAffinity.String()) + `,`,
		`NamespaceName:` + fmt.Sprintf("%v", in.NamespaceName) + `,`,
		`NodeAffinity:` + fmt.Sprintf("%v", in.NodeAffinity.String()) + `,`,
		`NodeName:` + fmt.Sprintf("%v", in.NodeName) + `,`,
		`MultusName:` + fmt.Sprintf("%v", in.MultusName) + `,`,
		`Default:` + stringutil.ValueToStringGenerated(in.Default) + `,`,
		`Disable:` + stringutil.ValueToStringGenerated(in.Disable) + `,`,
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
		`AllocatedIPs:` + stringutil.ValueToStringGenerated(in.AllocatedIPs) + `,`,
		`TotalIPCount:` + stringutil.ValueToStringGenerated(in.TotalIPCount) + `,`,
		`AllocatedIPCount:` + stringutil.ValueToStringGenerated(in.AllocatedIPCount) + `,`,
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

	s := strings.Join([]string{`&WorkloadEndpointStatus{`,
		`Current:` + fmt.Sprintf("%v", in.Current.String()) + `,`,
		`OwnerControllerType:` + fmt.Sprintf("%v", in.OwnerControllerType) + `,`,
		`OwnerControllerName:` + fmt.Sprintf("%v", in.OwnerControllerName) + `,`,
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
		`UID:` + fmt.Sprintf("%+v", in.UID) + `,`,
		`Node:` + fmt.Sprintf("%+v", in.Node) + `,`,
		`IPs:` + repeatedStringForIPs + `,`,
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
		`IPv4:` + stringutil.ValueToStringGenerated(in.IPv4) + `,`,
		`IPv6:` + stringutil.ValueToStringGenerated(in.IPv6) + `,`,
		`IPv4Pool:` + stringutil.ValueToStringGenerated(in.IPv4Pool) + `,`,
		`IPv6Pool:` + stringutil.ValueToStringGenerated(in.IPv6Pool) + `,`,
		`Vlan:` + stringutil.ValueToStringGenerated(in.Vlan) + `,`,
		`IPv4Gateway:` + stringutil.ValueToStringGenerated(in.IPv4Gateway) + `,`,
		`IPv6Gateway:` + stringutil.ValueToStringGenerated(in.IPv6Gateway) + `,`,
		`CleanGateway:` + stringutil.ValueToStringGenerated(in.CleanGateway) + `,`,
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
		`IPVersion:` + stringutil.ValueToStringGenerated(in.IPVersion) + `,`,
		`IPs:` + fmt.Sprintf("%v", in.IPs) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderSubnet
func (in *SpiderSubnet) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&SpiderSubnet{`,
		`ObjectMeta:` + strings.Replace(fmt.Sprintf("%v", in.ObjectMeta), `&`, ``, 1) + `,`,
		`Spec:` + strings.Replace(strings.Replace(in.Spec.String(), "SubnetSpec", "SubnetSpec", 1), `&`, ``, 1) + `,`,
		`Status:` + strings.Replace(strings.Replace(in.Status.String(), "SubnetStatus", "SubnetStatus", 1), `&`, ``, 1) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderSubnet Spec
func (in *SubnetSpec) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&SubnetSpec{`,
		`IPVersion:` + stringutil.ValueToStringGenerated(in.IPVersion) + `,`,
		`Subnet:` + fmt.Sprintf("%v", in.Subnet) + `,`,
		`IPs:` + fmt.Sprintf("%v", in.IPs) + `,`,
		`ExcludeIPs:` + fmt.Sprintf("%v", in.ExcludeIPs) + `,`,
		`Gateway:` + stringutil.ValueToStringGenerated(in.Gateway) + `,`,
		`Vlan:` + stringutil.ValueToStringGenerated(in.Vlan) + `,`,
		`Routes:` + fmt.Sprintf("%+v", in.Routes) + `,`,
		`}`,
	}, "")
	return s
}

// String serves for SpiderSubnet Status
func (in *SubnetStatus) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`SubnetStatus{`,
		`ControlledIPPools:` + stringutil.ValueToStringGenerated(in.ControlledIPPools) + `,`,
		`TotalIPCount:` + stringutil.ValueToStringGenerated(in.TotalIPCount) + `,`,
		`AllocatedIPCount:` + stringutil.ValueToStringGenerated(in.AllocatedIPCount) + `,`,
		`}`,
	}, "")
	return s
}
