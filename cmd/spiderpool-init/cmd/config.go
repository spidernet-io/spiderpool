// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"strings"

	"github.com/asaskevich/govalidator"

	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
)

const (
	EnvDefaultIPv4SubnetName   = "SPIDERPOOL_INIT_DEFAULT_IPV4_SUBNET_NAME"
	EnvDefaultIPv4PoolName     = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_NAME"
	EnvDefaultIPv4PoolSubnet   = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_SUBNET"
	EnvDefaultIPv4PoolIPRanges = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_IPRANGES"
	EnvDefaultIPv4PoolGateway  = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_GATEWAY"

	EnvDefaultIPv6SubnetName   = "SPIDERPOOL_INIT_DEFAULT_IPV6_SUBNET_NAME"
	EnvDefaultIPv6PoolName     = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_NAME"
	EnvDefaultIPv6PoolSubnet   = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_SUBNET"
	EnvDefaultIPv6PoolIPRanges = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_IPRANGES"
	EnvDefaultIPv6PoolGateway  = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_GATEWAY"

	EnvNamespace                = "SPIDERPOOL_NAMESPACE"
	EnvSpiderpoolControllerName = "SPIDERPOOL_CONTROLLER_NAME"
)

type _Config struct {
	SubnetV4Name   string
	PoolV4Name     string
	PoolV4Subnet   string
	PoolV4IPRanges []string
	PoolV4Gateway  string

	SubnetV6Name   string
	PoolV6Name     string
	PoolV6Subnet   string
	PoolV6IPRanges []string
	PoolV6Gateway  string
}

var Config = _Config{}

func init() {
	// -------- for ipv4
	Config.SubnetV4Name = os.Getenv(EnvDefaultIPv4SubnetName)
	logger.Sugar().Infof("SubnetV4Name=%s", Config.SubnetV4Name)

	Config.PoolV4Name = os.Getenv(EnvDefaultIPv4PoolName)
	logger.Sugar().Infof("PoolV4Name=%s", Config.PoolV4Name)

	Config.PoolV4Subnet = os.Getenv(EnvDefaultIPv4PoolSubnet)
	logger.Sugar().Infof("PoolV4Subnet=%s", Config.PoolV4Subnet)
	if len(Config.PoolV4Subnet) > 0 {
		if _, e := spiderpoolip.ParseCIDR(4, Config.PoolV4Subnet); e != nil {
			logger.Sugar().Fatalf("PoolV4Subnet '%v' is bad format, error: %v", Config.PoolV4Subnet, e)
		}
	}

	Config.PoolV4Gateway = os.Getenv(EnvDefaultIPv4PoolGateway)
	logger.Sugar().Infof("PoolV4Gateway=%s", Config.PoolV4Gateway)
	if len(Config.PoolV4Gateway) > 0 {
		if !govalidator.IsIPv4(Config.PoolV4Gateway) {
			logger.Sugar().Fatalf("PoolV4Gateway %v is not ipv4 address", Config.PoolV4Gateway)
		}
	}

	tmp := os.Getenv(EnvDefaultIPv4PoolIPRanges)
	logger.Sugar().Infof("PoolV4IPRanges=%s", tmp)
	if len(tmp) > 0 {
		tmp = strings.Replace(tmp, "\"", "", -1)
		tmp = strings.Replace(tmp, "[", "", -1)
		tmp = strings.Replace(tmp, "]", "", -1)
		t := strings.Split(tmp, ",")
		if _, err := spiderpoolip.ParseIPRanges(4, t); nil != err {
			logger.Sugar().Fatalf("PoolV4IPRanges format is wrong,  PoolV4IPRanges='%v', error: %v", t, err)
		}
		Config.PoolV4IPRanges = t
	}

	// ---------- for ipv6
	Config.SubnetV6Name = os.Getenv(EnvDefaultIPv6SubnetName)
	logger.Sugar().Infof("SubnetV6Name=%s", Config.SubnetV6Name)

	Config.PoolV6Name = os.Getenv(EnvDefaultIPv6PoolName)
	logger.Sugar().Infof("PoolV6Name=%s", Config.PoolV6Name)

	Config.PoolV6Subnet = os.Getenv(EnvDefaultIPv6PoolSubnet)
	logger.Sugar().Infof("PoolV6Subnet=%s", Config.PoolV6Subnet)
	if len(Config.PoolV6Subnet) > 0 {
		if _, e := spiderpoolip.ParseCIDR(6, Config.PoolV6Subnet); e != nil {
			logger.Sugar().Fatalf("PoolV6Subnet '%v' is bad format, error: %v", Config.PoolV4Subnet, e)
		}
	}

	Config.PoolV6Gateway = os.Getenv(EnvDefaultIPv6PoolGateway)
	logger.Sugar().Infof("PoolV6Gateway=%s", Config.PoolV6Gateway)
	if len(Config.PoolV6Gateway) > 0 {
		if !govalidator.IsIPv6(Config.PoolV6Gateway) {
			logger.Sugar().Fatalf("PoolV6Gateway %v is not ipv6 address", Config.PoolV6Gateway)
		}
	}

	tmp = os.Getenv(EnvDefaultIPv6PoolIPRanges)
	logger.Sugar().Infof("PoolV6IPRanges=%s", tmp)
	if len(tmp) > 0 {
		tmp = strings.Replace(tmp, "\"", "", -1)
		tmp = strings.Replace(tmp, "[", "", -1)
		tmp = strings.Replace(tmp, "]", "", -1)
		t := strings.Split(tmp, ",")
		if _, err := spiderpoolip.ParseIPRanges(6, t); nil != err {
			logger.Sugar().Fatalf("PoolV6IPRanges format is wrong,  PoolV6IPRanges='%v', error: %v", t, err)
		}
		Config.PoolV6IPRanges = t
	}

	SpiderControllerEndpointNamespace = os.Getenv(EnvNamespace)
	logger.Sugar().Infof("SpiderControllerEndpointNamespace=%s", SpiderControllerEndpointNamespace)
	if len(SpiderControllerEndpointNamespace) == 0 {
		logger.Sugar().Fatalf("SpiderControllerEndpointNamespace is empty")
	}

	SpiderControllerEndpointName = os.Getenv(EnvSpiderpoolControllerName)
	logger.Sugar().Infof("SpiderControllerEndpointName=%s", SpiderControllerEndpointName)
	if len(SpiderControllerEndpointName) == 0 {
		logger.Sugar().Fatalf("SpiderControllerEndpointName is empty")
	}

	// check
	if len(Config.PoolV4Name) != 0 {
		if len(Config.PoolV4Subnet) == 0 {
			logger.Sugar().Fatalf("PoolV4Subnet is empty")
		}
		if len(Config.PoolV4IPRanges) == 0 {
			logger.Sugar().Fatalf("PoolV4IPRanges is empty")
		}
	} else {
		logger.Info("ignore creating IPv4 ippool ")
	}
	if len(Config.PoolV6Name) != 0 {
		if len(Config.PoolV6Subnet) == 0 {
			logger.Sugar().Fatalf("PoolV6Subnet is empty")
		}
		if len(Config.PoolV6IPRanges) == 0 {
			logger.Sugar().Fatalf("PoolV6IPRanges is empty")
		}
	} else {
		logger.Info("ignore creating IPv6 ippool ")
	}

}
