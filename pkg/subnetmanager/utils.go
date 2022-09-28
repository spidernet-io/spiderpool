// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var ErrorAnnoInput = fmt.Errorf("wrong annotation input")

type PodSubnetAnno struct {
	subnetManagerV4 string
	subnetManagerV6 string
	flexibleIPNum   *int
	assignIPNum     int
	reclaimIPPool   bool
}

func getSubnetConfigFromPodAnno(annotations map[string]string, appReplicas int) (*PodSubnetAnno, error) {
	// annotation: "spiderpool.spidernet.io/spider-subnet-v4" and "spiderpool.spidernet.io/spider-subnet-v6"
	subnetManagerV4, enableSubnetMgrV4 := annotations[constant.AnnoSubnetManagerV4]
	subnetManagerV6, enableSubnetMgrV6 := annotations[constant.AnnoSubnetManagerV6]

	// standard IPAM mode, do not use subnet manager
	if !enableSubnetMgrV4 && !enableSubnetMgrV6 {
		return nil, nil
	}

	podSubnetConfig := new(PodSubnetAnno)
	podSubnetConfig.subnetManagerV4 = subnetManagerV4
	podSubnetConfig.subnetManagerV6 = subnetManagerV6

	// annotation: "spiderpool.spidernet.io/flexible-ip-number"
	flexibleIPNumber, enableFlexibleIPNum := annotations[constant.AnnoSubnetManagerFlexibleIPNumber]
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
		podSubnetConfig.flexibleIPNum = pointer.Int(i)
	} else {
		// annotation: "spiderpool.spidernet.io/assign-ip-number"
		assignIPNum, enableAssignIPNumber := annotations[constant.AnnoSubnetManagerAssignIPNumber]
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
			podSubnetConfig.assignIPNum = i
		} else {
			podSubnetConfig.assignIPNum = appReplicas
		}
	}

	// annotation: "spiderpool.spidernet.io/reclaim-ippool", reclaim IPPool or not (default true)
	reclaimPool, ok := annotations[constant.AnnoSubnetManagerReclaimIPPool]
	if ok {
		parseBool, err := strconv.ParseBool(reclaimPool)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse subnet manager '%s', error: %v", ErrorAnnoInput, constant.AnnoSubnetManagerReclaimIPPool, err)
		}
		podSubnetConfig.reclaimIPPool = parseBool
	} else {
		podSubnetConfig.reclaimIPPool = true
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

func getAppReplicas(replicas *int32) int {
	if replicas == nil {
		return 0
	}

	return int(*replicas)
}
