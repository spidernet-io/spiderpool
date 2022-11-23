// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var ErrorAnnoInput = fmt.Errorf("wrong annotation input")

var errInvalidInput = func(str string) error {
	return fmt.Errorf("invalid input '%s'", str)
}

// PodSubnetAnnoConfig is used for the annotation `ipam.spidernet.io/subnet`,
type PodSubnetAnnoConfig struct {
	SubnetName    AnnoSubnetItems
	FlexibleIPNum *int
	AssignIPNum   int
	ReclaimIPPool bool
}

func (in *PodSubnetAnnoConfig) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&PodSubnetAnnoConfig{`,
		`SubnetName:` + strings.Replace(strings.Replace(in.SubnetName.String(), "AnnoSubnetItems", "", 1), `&`, ``, 1) + `,`,
		`FlexibleIPNum:` + spiderpoolv1.ValueToStringGenerated(in.FlexibleIPNum) + `,`,
		`AssignIPNumber:` + fmt.Sprintf("%v", in.AssignIPNum) + `,`,
		`ReclaimIPPool:` + fmt.Sprintf("%v", in.ReclaimIPPool),
		`}`,
	}, "")
	return s
}

// AnnoSubnetItems describes the SpiderSubnet CR names and NIC
type AnnoSubnetItems struct {
	Interface string   `json:"interface,omitempty"`
	IPv4      []string `json:"ipv4,omitempty"`
	IPv6      []string `json:"ipv6,omitempty"`
}

func (in *AnnoSubnetItems) String() string {
	if in == nil {
		return "nil"
	}

	s := strings.Join([]string{`&AnnoSubnetItems{`,
		`Interface:` + fmt.Sprintf("%v", in.Interface) + `,`,
		`IPv4:` + fmt.Sprintf("%v", in.IPv4) + `,`,
		`IPv6:` + fmt.Sprintf("%v", in.IPv6),
		`}`,
	}, "")
	return s
}

// PodSubnetsAnnoConfig is used for the annotation `ipam.spidernet.io/subnets`,
// NOT support in the present version.
type PodSubnetsAnnoConfig struct {
	SubnetName    []AnnoSubnetItems
	FlexibleIPNum *int
	AssignIPNum   int
	ReclaimIPPool bool
}

func (in *PodSubnetsAnnoConfig) String() string {
	if in == nil {
		return "nil"
	}

	repeatedStringForSubnetName := "[]SubnetName{"
	for _, f := range in.SubnetName {
		repeatedStringForSubnetName += strings.Replace(strings.Replace(f.String(), "AnnoSubnetItems", "", 1), `&`, ``, 1) + ","
	}
	repeatedStringForSubnetName += "}"

	s := strings.Join([]string{`&PodSubnetsAnnoConfig`,
		`SubnetName:` + repeatedStringForSubnetName + `,`,
		`FlexibleIPNum:` + spiderpoolv1.ValueToStringGenerated(in.FlexibleIPNum) + `,`,
		`AssignIPNumber:` + fmt.Sprintf("%v", in.AssignIPNum) + `,`,
		`ReclaimIPPool:` + fmt.Sprintf("%v", in.ReclaimIPPool),
		`}`,
	}, "")
	return s
}

func SubnetPoolName(controllerKind, controllerNS, controllerName string, ipVersion types.IPVersion, controllerUID apitypes.UID) string {
	// the format of uuid is "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	// ref: https://github.com/google/uuid/blob/44b5fee7c49cf3bcdf723f106b36d56ef13ccc88/uuid.go#L185
	splits := strings.Split(string(controllerUID), "-")
	lastOne := splits[len(splits)-1]

	return fmt.Sprintf("auto-%s-%s-%s-v%d-%s",
		strings.ToLower(controllerKind), strings.ToLower(controllerNS), strings.ToLower(controllerName), ipVersion, strings.ToLower(lastOne))
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

// GetSubnetAnnoConfig generates SpiderSubnet configuration from pod annotation,
// if the pod doesn't have the related subnet annotation it will return nil
func GetSubnetAnnoConfig(podAnnotations map[string]string) (*PodSubnetAnnoConfig, error) {
	var subnetAnnoConfig PodSubnetAnnoConfig

	// annotation: ipam.spidernet.io/subnets
	subnets, ok := podAnnotations[constant.AnnoSpiderSubnets]
	if ok {
		var subnetsItems []AnnoSubnetItems
		err := json.Unmarshal([]byte(subnets), &subnetsItems)
		if nil != err {
			return nil, fmt.Errorf("failed to parse anntation '%s' value '%s', error: %v", constant.AnnoSpiderSubnets, subnets, err)
		}
		if len(subnetsItems) == 0 {
			return nil, fmt.Errorf("%w: annotation '%s' value requires at least one item", ErrorAnnoInput, constant.AnnoSpiderSubnets)
		}

		// the present version, we just only support to use one network Interface with SpiderSubnet feature
		firstSubnetItem := subnetsItems[0]
		subnetAnnoConfig.SubnetName = firstSubnetItem
	} else {
		// annotation: ipam.spidernet.io/subnet
		subnet, enableSubnet := podAnnotations[constant.AnnoSpiderSubnet]
		if !enableSubnet {
			// default IPAM mode
			return nil, nil
		}
		err := json.Unmarshal([]byte(subnet), &subnetAnnoConfig.SubnetName)
		if nil != err {
			return nil, fmt.Errorf("failed to parse anntation '%s' value '%s', error: %v", constant.AnnoSpiderSubnet, subnet, err)
		}
	}

	// the present version, we just only support one SpiderSubnet object to choose
	if len(subnetAnnoConfig.SubnetName.IPv4) > 1 {
		subnetAnnoConfig.SubnetName.IPv4 = []string{subnetAnnoConfig.SubnetName.IPv4[0]}
	}
	if len(subnetAnnoConfig.SubnetName.IPv6) > 1 {
		subnetAnnoConfig.SubnetName.IPv6 = []string{subnetAnnoConfig.SubnetName.IPv6[0]}
	}

	var isFlexible bool
	var ipNum int
	var err error

	// annotation: ipam.spidernet.io/ippool-ip-number, (default: +0)
	poolIPNum, ok := podAnnotations[constant.AnnoSpiderSubnetPoolIPNumber]
	if ok {
		isFlexible, ipNum, err = getPoolIPNumber(poolIPNum)
		if nil != err {
			return nil, fmt.Errorf("%w: %v", ErrorAnnoInput, err)
		}

		// check out negative number
		if ipNum < 0 {
			return nil, fmt.Errorf("%w: subnet '%s' value must equal or greater than 0", ErrorAnnoInput, constant.AnnoSpiderSubnetPoolIPNumber)
		}

		if isFlexible {
			subnetAnnoConfig.FlexibleIPNum = pointer.Int(ipNum)
		} else {
			subnetAnnoConfig.AssignIPNum = ipNum
		}
	} else {
		// no annotation "ipam.spidernet.io/ippool-ip-number", we just set the pool IP number `+0`
		subnetAnnoConfig.FlexibleIPNum = pointer.Int(0)
	}

	// annotation: "ipam.spidernet.io/reclaim-ippool", reclaim IPPool or not (default true)
	reclaimPool, ok := podAnnotations[constant.AnnoSpiderSubnetReclaimIPPool]
	if ok {
		parseBool, err := strconv.ParseBool(reclaimPool)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse spider subnet '%s', error: %v", ErrorAnnoInput, constant.AnnoSpiderSubnetReclaimIPPool, err)
		}
		subnetAnnoConfig.ReclaimIPPool = parseBool
	} else {
		subnetAnnoConfig.ReclaimIPPool = true
	}

	return &subnetAnnoConfig, nil
}

// getPoolIPNumber judges the given parameter is fixed or flexible
func getPoolIPNumber(str string) (isFlexible bool, ipNum int, err error) {
	tmp := str

	// the '+' sign counts must be '0' or '1'
	plusSignNum := strings.Count(str, "+")
	if plusSignNum == 0 || plusSignNum == 1 {
		_, after, found := strings.Cut(str, "+")
		if found {
			tmp = after
		}

		ipNum, err = strconv.Atoi(tmp)
		if nil != err {
			return false, -1, fmt.Errorf("%w: %v", errInvalidInput(str), err)
		}

		return found, ipNum, nil
	}

	return false, -1, errInvalidInput(str)
}

// CalculateJobPodNum will calculate the job replicas
// once Parallelism and Completions are unset, the API-server will set them to 1
// reference: https://kubernetes.io/docs/concepts/workloads/controllers/job/
func CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions *int32) int {
	switch {
	case jobSpecParallelism != nil && jobSpecCompletions == nil:
		// parallel Jobs with a work queue
		if *jobSpecParallelism == 0 {
			return 1
		}

		// ignore negative integer, cause API-server will refuse the job creation
		return int(*jobSpecParallelism)

	case jobSpecParallelism == nil && jobSpecCompletions != nil:
		// non-parallel Jobs
		if *jobSpecCompletions == 0 {
			return 1
		}

		// ignore negative integer, cause API-server will refuse the job creation
		return int(*jobSpecCompletions)

	case jobSpecParallelism != nil && jobSpecCompletions != nil:
		// parallel Jobs with a fixed completion count
		if *jobSpecCompletions == 0 {
			return 1
		}

		// ignore negative integer, cause API-server will refuse the job creation
		return int(*jobSpecCompletions)
	}

	return 1
}
