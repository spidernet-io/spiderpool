// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var ErrorAnnoInput = fmt.Errorf("wrong annotation input")

type PodSubnetAnno struct {
	SubnetManagerV4 string
	SubnetManagerV6 string
	FlexibleIPNum   *int
	AssignIPNum     int
	ReclaimIPPool   bool
}

func (in *PodSubnetAnno) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&PodSubnetAnnotation{`,
		`SpiderSubnetV4:` + fmt.Sprintf("%v", in.SubnetManagerV4) + `,`,
		`SpiderSubnetV6:` + fmt.Sprintf("%v", in.SubnetManagerV6) + `,`,
		`FlexibleIPNumber:` + spiderpoolv1.ValueToStringGenerated(in.FlexibleIPNum) + `,`,
		`AssignIPNumber:` + fmt.Sprintf("%v", in.AssignIPNum) + `,`,
		`ReclaimIPPool:` + fmt.Sprintf("%v", in.ReclaimIPPool) + `,`,
		`}`,
	}, "")
	return s
}

// GetSubnetConfigFromPodAnno generates SpiderSubnet configuration from pod annotation,
// if the pod doesn't have the related subnet annotation it will return nil
func GetSubnetConfigFromPodAnno(podAnnotations map[string]string, appReplicas int) (*PodSubnetAnno, error) {
	// annotation: "spiderpool.spidernet.io/spider-subnet-v4" and "spiderpool.spidernet.io/spider-subnet-v6"
	subnetManagerV4, enableSubnetMgrV4 := podAnnotations[constant.AnnoSubnetManagerV4]
	subnetManagerV6, enableSubnetMgrV6 := podAnnotations[constant.AnnoSubnetManagerV6]

	// standard IPAM mode, do not use subnet manager
	if !enableSubnetMgrV4 && !enableSubnetMgrV6 {
		return nil, nil
	}

	podSubnetConfig := new(PodSubnetAnno)
	podSubnetConfig.SubnetManagerV4 = subnetManagerV4
	podSubnetConfig.SubnetManagerV6 = subnetManagerV6

	// annotation: "spiderpool.spidernet.io/flexible-ip-number"
	flexibleIPNumber, enableFlexibleIPNum := podAnnotations[constant.AnnoSubnetManagerFlexibleIPNumber]
	if enableFlexibleIPNum {
		// invalid case: "spiderpool.spidernet.io/flexible-ip-number":""
		if flexibleIPNumber == "" {
			return nil, fmt.Errorf("%w: subnet manager '%s' value is empty", ErrorAnnoInput, constant.AnnoSubnetManagerFlexibleIPNumber)
		}
		i, err := strconv.Atoi(flexibleIPNumber)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse subnet manager '%s', error: %v", ErrorAnnoInput, constant.AnnoSubnetManagerFlexibleIPNumber, err)
		}

		// invalid case: "spiderpool.spidernet.io/flexible-ip-number":"-1"
		if i < 0 {
			return nil, fmt.Errorf("%w: subnet manager '%s' value must equal or greater than 0", ErrorAnnoInput, constant.AnnoSubnetManagerFlexibleIPNumber)
		}
		podSubnetConfig.FlexibleIPNum = pointer.Int(i)
	} else {
		// annotation: "spiderpool.spidernet.io/assign-ip-number"
		assignIPNum, enableAssignIPNumber := podAnnotations[constant.AnnoSubnetManagerAssignIPNumber]
		if enableAssignIPNumber {
			// invalid case: "spiderpool.spidernet.io/assign-ip-number":""
			if assignIPNum == "" {
				return nil, fmt.Errorf("%w: subnet manager '%s' value is empty", ErrorAnnoInput, constant.AnnoSubnetManagerAssignIPNumber)
			}
			i, err := strconv.Atoi(assignIPNum)
			if nil != err {
				return nil, fmt.Errorf("%w: failed to parse subnet manager '%s', error: %v", ErrorAnnoInput, constant.AnnoSubnetManagerAssignIPNumber, err)
			}

			// invalid case: "spiderpool.spidernet.io/assign-ip-number":"-1"
			if i < 0 {
				return nil, fmt.Errorf("%w: subnet manager '%s' value must equal or greater than 0", ErrorAnnoInput, constant.AnnoSubnetManagerAssignIPNumber)
			}
			podSubnetConfig.AssignIPNum = i
		} else {
			podSubnetConfig.AssignIPNum = appReplicas
		}
	}

	// annotation: "spiderpool.spidernet.io/reclaim-ippool", reclaim IPPool or not (default true)
	reclaimPool, ok := podAnnotations[constant.AnnoSubnetManagerReclaimIPPool]
	if ok {
		parseBool, err := strconv.ParseBool(reclaimPool)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse subnet manager '%s', error: %v", ErrorAnnoInput, constant.AnnoSubnetManagerReclaimIPPool, err)
		}
		podSubnetConfig.ReclaimIPPool = parseBool
	} else {
		podSubnetConfig.ReclaimIPPool = true
	}

	return podSubnetConfig, nil
}

func SubnetPoolName(controllerKind, controllerNS, controllerName string, ipVersion types.IPVersion) string {
	return fmt.Sprintf("auto-%s-%s-%s-v%d-%d",
		strings.ToLower(controllerKind), strings.ToLower(controllerNS), strings.ToLower(controllerName), ipVersion, time.Now().Unix())
}

func AppLabelValue(appKind string, appNS, appName string) string {
	return fmt.Sprintf("%s-%s-%s", strings.ToLower(appKind), strings.ToLower(appNS), strings.ToLower(appName))
}

func GetAppReplicas(replicas *int32) int {
	if replicas == nil {
		return 0
	}

	return int(*replicas)
}

func GenSubnetFreeIPs(subnet *spiderpoolv1.SpiderSubnet) ([]net.IP, error) {
	var used []string
	for _, pool := range subnet.Status.ControlledIPPools {
		used = append(used, pool.IPs...)
	}
	usedIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, used)
	if err != nil {
		return nil, err
	}

	totalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return nil, err
	}
	freeIPs := spiderpoolip.IPsDiffSet(totalIPs, usedIPs)

	return freeIPs, nil
}
