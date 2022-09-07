// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"fmt"
	"strconv"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var ErrorAnnoInput = fmt.Errorf("wrong annotation input")

const (
	AnnoSubnetManagerV4 = "v4-ns-name"
	AnnoSubnetManagerV6 = "v6-ns-name"
	AnnoAssignIPNumber  = "10"
	AnnoReclaimIPPool   = "true"

	OwnedSubnetManager = constant.AnnotationPre + "/OwnedSubnetManager"
	OwnedApplication   = constant.AnnotationPre + "/OwnedApplication"
)

type PodSubnetAnno struct {
	subnetManagerV4 string
	subnetManagerV6 string
	assignIPNum     int
	reclaimIPPool   bool
}

// validating
func getObjSubnetConfig(annotations map[string]string) (PodSubnetAnno, error) {
	podSubnetConfig := PodSubnetAnno{}

	podSubnetConfig.subnetManagerV4 = annotations[AnnoSubnetManagerV4]
	podSubnetConfig.subnetManagerV6 = annotations[AnnoSubnetManagerV6]

	if len(podSubnetConfig.subnetManagerV4) == 0 && len(podSubnetConfig.subnetManagerV6) == 0 {
		return PodSubnetAnno{}, fmt.Errorf("%w: you must specify at least one subnet manager", ErrorAnnoInput)
	}

	numberStr := annotations[AnnoAssignIPNumber]
	if len(numberStr) != 0 {
		i, err := strconv.Atoi(numberStr)
		if nil != err {
			return PodSubnetAnno{}, err
		}

		if i <= 0 {
			return PodSubnetAnno{}, fmt.Errorf("%w: assign IP number must greater than 0", ErrorAnnoInput)
		}
		podSubnetConfig.assignIPNum = i
	}

	reclaimPool := annotations[AnnoReclaimIPPool]
	if len(reclaimPool) != 0 {
		parseBool, err := strconv.ParseBool(reclaimPool)
		if nil != err {
			return PodSubnetAnno{}, err
		}
		podSubnetConfig.reclaimIPPool = parseBool
	}

	return podSubnetConfig, nil
}

func SubnetPoolName(controllerKind, controllerNS, controllerName string, ipVersion types.IPVersion) string {
	return fmt.Sprintf("auto-%s-%s-%s-v%d", controllerKind, controllerNS, controllerName, ipVersion)
}

func AppName(appKind string, appNS, appName string) string {
	return fmt.Sprintf("%s/%s/%s", appKind, appNS, appName)
}
