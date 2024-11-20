// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/libcni"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	coordinatorcmd "github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
)

const (
	ENVNamespace                = "SPIDERPOOL_NAMESPACE"
	ENVSpiderpoolControllerName = "SPIDERPOOL_CONTROLLER_NAME"
	ENVSpiderpoolAgentName      = "SPIDERPOOL_AGENT_NAME"

	ENVDefaultCoordinatorName             = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_NAME"
	ENVDefaultCoordinatorTuneMode         = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_MODE"
	ENVDefaultCoordinatorPodCIDRType      = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_POD_CIDR_TYPE"
	ENVDefaultCoordinatorDetectGateway    = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_DETECT_GATEWAY"
	ENVDefaultCoordinatorDetectIPConflict = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_DETECT_IP_CONFLICT"
	ENVDefaultCoordinatorTunePodRoutes    = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_TUNE_POD_ROUTES"
	ENVDefaultCoordiantorHijackCIDR       = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_HIJACK_CIDR"

	ENVDefaultIPv4SubnetName = "SPIDERPOOL_INIT_DEFAULT_IPV4_SUBNET_NAME"
	ENVDefaultIPv4IPPoolName = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_NAME"
	ENVDefaultIPv4CIDR       = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_SUBNET"
	ENVDefaultIPv4IPRanges   = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_IPRANGES"
	ENVDefaultIPv4Gateway    = "SPIDERPOOL_INIT_DEFAULT_IPV4_IPPOOL_GATEWAY"

	ENVDefaultIPv6SubnetName = "SPIDERPOOL_INIT_DEFAULT_IPV6_SUBNET_NAME"
	ENVDefaultIPv6IPPoolName = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_NAME"
	ENVDefaultIPv6CIDR       = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_SUBNET"
	ENVDefaultIPv6IPRanges   = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_IPRANGES"
	ENVDefaultIPv6Gateway    = "SPIDERPOOL_INIT_DEFAULT_IPV6_IPPOOL_GATEWAY"

	ENVEnableMultusConfig                = "SPIDERPOOL_INIT_ENABLE_MULTUS_CONFIG"
	ENVInstallMultusCNI                  = "SPIDERPOOL_INIT_INSTALL_MULTUS"
	ENVDefaultCNIDir                     = "SPIDERPOOL_INIT_DEFAULT_CNI_DIR"
	ENVDefaultCNIName                    = "SPIDERPOOL_INIT_DEFAULT_CNI_NAME"
	ENVDefaultCNINamespace               = "SPIDERPOOL_INIT_DEFAULT_CNI_NAMESPACE"
	ENVDefaultMultusConfigMap            = "SPIDERPOOL_INIT_MULTUS_CONFIGMAP"
	ENVDefaultReadinessFile              = "SPIDERPOOL_INIT_READINESS_FILE"
	ENVDefaultCoordinatorVethLinkAddress = "SPIDERPOOL_INIT_DEFAULT_COORDINATOR_VETH_LINK_ADDRESS"
)

var (
	legacyCalicoCniName = "k8s-pod-network"
	calicoCniName       = "calico"
	readinessFileName   = "/etc/spiderpool/ready"
)

type InitDefaultConfig struct {
	Namespace      string
	ControllerName string
	AgentName      string

	CoordinatorName               string
	CoordinatorMode               string
	CoordinatorPodCIDRType        string
	CoordinatorPodDefaultRouteNic string
	CoordinatorPodMACPrefix       string
	CoordinatorVethLinkAddress    string
	CoordinatorDetectGateway      bool
	CoordinatorDetectIPConflict   bool
	CoordinatorTunePodRoutes      bool
	CoordinatorHijackCIDR         []string

	V4SubnetName string
	V4IPPoolName string
	V4CIDR       string
	V4IPRanges   []string
	V4Gateway    string

	V6SubnetName string
	V6IPPoolName string
	V6CIDR       string
	V6IPRanges   []string
	V6Gateway    string

	// multuscniconfig
	enableMultusConfig  bool
	DefaultCNIDir       string
	DefaultCNIName      string
	DefaultCNINamespace string
	MultusConfigMap     string

	// readiness
	ReadinessFile string
}

func NewInitDefaultConfig() InitDefaultConfig {
	return parseENVAsDefault()
}

func parseENVAsDefault() InitDefaultConfig {
	config := InitDefaultConfig{}
	config.Namespace = strings.ReplaceAll(os.Getenv(ENVNamespace), "\"", "")
	if len(config.Namespace) == 0 {
		logger.Sugar().Fatalf("ENV %s %w", ENVNamespace, constant.ErrMissingRequiredParam)
	}
	config.ControllerName = strings.ReplaceAll(os.Getenv(ENVSpiderpoolControllerName), "\"", "")
	if len(config.ControllerName) == 0 {
		logger.Sugar().Fatalf("ENV %s %v", ENVSpiderpoolControllerName, constant.ErrMissingRequiredParam)
	}
	config.AgentName = strings.ReplaceAll(os.Getenv(ENVSpiderpoolAgentName), "\"", "")
	if len(config.AgentName) == 0 {
		logger.Sugar().Fatalf("ENV %s %v", ENVSpiderpoolAgentName, constant.ErrMissingRequiredParam)
	}

	// Coordinator
	config.CoordinatorName = strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorName), "\"", "")
	if len(config.CoordinatorName) != 0 {
		config.CoordinatorMode = strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorTuneMode), "\"", "")
		if config.CoordinatorMode == "" {
			config.CoordinatorMode = string(coordinatorcmd.ModeAuto)
		}
		config.CoordinatorPodCIDRType = strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorPodCIDRType), "\"", "")

		edg := strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorDetectGateway), "\"", "")
		dg, err := strconv.ParseBool(edg)
		if err != nil {
			logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultCoordinatorDetectGateway, edg, err)
		}
		config.CoordinatorDetectGateway = dg

		edic := strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorDetectIPConflict), "\"", "")
		dic, err := strconv.ParseBool(edic)
		if err != nil {
			logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultCoordinatorDetectIPConflict, edic, err)
		}
		config.CoordinatorDetectIPConflict = dic

		etpr := strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorTunePodRoutes), "\"", "")
		tpr, err := strconv.ParseBool(etpr)
		if err != nil {
			logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultCoordinatorTunePodRoutes, etpr, err)
		}
		config.CoordinatorTunePodRoutes = tpr
		config.CoordinatorPodDefaultRouteNic = ""
		config.CoordinatorPodMACPrefix = ""
		v := os.Getenv(ENVDefaultCoordiantorHijackCIDR)
		if len(v) > 0 {
			v = strings.ReplaceAll(v, "\"", "")
			v = strings.ReplaceAll(v, "\\", "")
			v = strings.ReplaceAll(v, "[", "")
			v = strings.ReplaceAll(v, "]", "")
			v = strings.ReplaceAll(v, ",", " ")
			subnets := strings.Fields(v)

			for idx, r := range subnets {
				nPrefix, err := spiderpoolip.ParseIPOrCIDR(r)
				if err != nil {
					logger.Sugar().Fatalf("ENV %s invalid: %v", ENVDefaultCoordiantorHijackCIDR, err)
				}
				subnets[idx] = nPrefix.String()
			}
			config.CoordinatorHijackCIDR = subnets
		} else {
			config.CoordinatorHijackCIDR = []string{}
		}

		config.CoordinatorVethLinkAddress = strings.ReplaceAll(os.Getenv(ENVDefaultCoordinatorVethLinkAddress), "\"", "")
	} else {
		logger.Info("Ignore creating default Coordinator")
	}

	// IPv4
	config.V4SubnetName = strings.ReplaceAll(os.Getenv(ENVDefaultIPv4SubnetName), "\"", "")
	config.V4IPPoolName = strings.ReplaceAll(os.Getenv(ENVDefaultIPv4IPPoolName), "\"", "")
	if len(config.V4SubnetName) != 0 || len(config.V4IPPoolName) != 0 {
		config.V4CIDR = strings.ReplaceAll(os.Getenv(ENVDefaultIPv4CIDR), "\"", "")
		if len(config.V4CIDR) == 0 {
			logger.Sugar().Fatalf("ENV %s %w, if you need to create a default IPv4 Subnet or IPPool", ENVDefaultIPv4CIDR, constant.ErrMissingRequiredParam)
		}
		if err := spiderpoolip.IsCIDR(constant.IPv4, config.V4CIDR); err != nil {
			logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultIPv4CIDR, config.V4CIDR, err)
		}

		config.V4Gateway = strings.ReplaceAll(os.Getenv(ENVDefaultIPv4Gateway), "\"", "")
		if len(config.V4Gateway) > 0 {
			if err := spiderpoolip.IsIP(constant.IPv4, config.V4Gateway); err != nil {
				logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultIPv4Gateway, config.V4Gateway, err)
			}
		}

		v := os.Getenv(ENVDefaultIPv4IPRanges)
		if len(v) > 0 {
			v = strings.ReplaceAll(v, "\"", "")
			v = strings.ReplaceAll(v, "\\", "")
			v = strings.ReplaceAll(v, "[", "")
			v = strings.ReplaceAll(v, "]", "")
			v = strings.ReplaceAll(v, ",", " ")
			ranges := strings.Fields(v)

			for _, r := range ranges {
				if err := spiderpoolip.IsIPRange(constant.IPv4, r); err != nil {
					logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultIPv4IPRanges, ranges, err)
				}
			}
			config.V4IPRanges = ranges
		}
	} else {
		logger.Info("Ignore creating default IPv4 Subnet or IPPool")
	}

	// IPv6
	config.V6SubnetName = strings.ReplaceAll(os.Getenv(ENVDefaultIPv6SubnetName), "\"", "")
	config.V6IPPoolName = strings.ReplaceAll(os.Getenv(ENVDefaultIPv6IPPoolName), "\"", "")
	if len(config.V6SubnetName) != 0 || len(config.V6IPPoolName) != 0 {
		config.V6CIDR = strings.ReplaceAll(os.Getenv(ENVDefaultIPv6CIDR), "\"", "")
		if len(config.V6CIDR) == 0 {
			logger.Sugar().Fatalf("ENV %s %w, if you need to create a default IPv6 Subnet or IPPool", ENVDefaultIPv6CIDR, constant.ErrMissingRequiredParam)
		}
		if err := spiderpoolip.IsCIDR(constant.IPv6, config.V6CIDR); err != nil {
			logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultIPv6CIDR, config.V6CIDR, err)
		}

		config.V6Gateway = strings.ReplaceAll(os.Getenv(ENVDefaultIPv6Gateway), "\"", "")
		if len(config.V6Gateway) > 0 {
			if err := spiderpoolip.IsIP(constant.IPv6, config.V6Gateway); err != nil {
				logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultIPv6Gateway, config.V6Gateway, err)
			}
		}

		v := os.Getenv(ENVDefaultIPv6IPRanges)
		if len(v) > 0 {
			v = strings.ReplaceAll(v, "\"", "")
			v = strings.ReplaceAll(v, "\\", "")
			v = strings.ReplaceAll(v, "[", "")
			v = strings.ReplaceAll(v, "]", "")
			v = strings.ReplaceAll(v, ",", " ")
			ranges := strings.Fields(v)

			for _, r := range ranges {
				if err := spiderpoolip.IsIPRange(constant.IPv6, r); err != nil {
					logger.Sugar().Fatalf("ENV %s %s: %v", ENVDefaultIPv6IPRanges, ranges, err)
				}
			}
			config.V6IPRanges = ranges
		}
	} else {
		logger.Info("Ignore creating default IPv6 Subnet or IPPool")
	}

	if config.V4SubnetName == config.V6SubnetName && len(config.V4SubnetName) != 0 {
		logger.Sugar().Fatalf(
			"ENV %s %s\nENV %s %s\nDefault IPv4 Subnet name cannot be the same as IPv6 one",
			ENVDefaultIPv4SubnetName,
			config.V4SubnetName,
			ENVDefaultIPv6SubnetName,
			config.V6SubnetName,
		)
	}
	if config.V4IPPoolName == config.V6IPPoolName && len(config.V4IPPoolName) != 0 {
		logger.Sugar().Fatalf(
			"ENV %s %s\nENV %s %s\nDefault IPv4 IPPool name cannot be the same as IPv6 one",
			ENVDefaultIPv4IPPoolName,
			config.V4IPPoolName,
			ENVDefaultIPv6IPPoolName,
			config.V6IPPoolName,
		)
	}

	var err error
	enableMultusConfig := strings.ReplaceAll(os.Getenv(ENVEnableMultusConfig), "\"", "")
	config.enableMultusConfig, err = strconv.ParseBool(enableMultusConfig)
	if err != nil {
		logger.Sugar().Fatalf("ENV %s: %s invalid: %v", ENVEnableMultusConfig, enableMultusConfig, err)
	}

	config.DefaultCNIDir = strings.ReplaceAll(os.Getenv(ENVDefaultCNIDir), "\"", "")
	if config.DefaultCNIDir != "" {
		_, err = os.ReadDir(config.DefaultCNIDir)
		if err != nil {
			logger.Sugar().Fatalf("ENV %s:%s invalid: %v", ENVDefaultCNIDir, config.DefaultCNIDir, err)
		}
	}

	config.DefaultCNIName = strings.ReplaceAll(os.Getenv(ENVDefaultCNIName), "\"", "")
	config.DefaultCNINamespace = strings.ReplaceAll(os.Getenv(ENVDefaultCNINamespace), "\"", "")
	config.MultusConfigMap = strings.ReplaceAll(os.Getenv(ENVDefaultMultusConfigMap), "\"", "")
	config.ReadinessFile = strings.ReplaceAll(os.Getenv(ENVDefaultReadinessFile), "\"", "")
	if config.ReadinessFile == "" {
		config.ReadinessFile = readinessFileName
	}

	logger.Sugar().Infof("Init default config: %+v", config)

	return config
}

// parseCNIFromConfig parse cni's name and type from given cni config path
func parseCNIFromConfig(cniConfigPath string) (string, string, error) {
	var cniName, cniType string
	if cniConfigPath == "" {
		logger.Sugar().Infof("No network found in %s, create default multuscniconfig", cniConfigPath)
		return "default", constant.CustomCNI, nil
	}

	logger.Sugar().Infof("the first cni config file is %s in /etc/cni/net.d", cniConfigPath)
	if strings.HasSuffix(cniConfigPath, ".conflist") {
		confList, err := libcni.ConfListFromFile(cniConfigPath)
		if err != nil {
			return "", "", fmt.Errorf("error loading CNI conflist file %s: %v", cniConfigPath, err)
		}
		cniName = confList.Name
		cniType = confList.Plugins[0].Network.Type

	} else {
		conf, err := libcni.ConfFromFile(cniConfigPath)
		if err != nil {
			return "", "", fmt.Errorf("error loading CNI config file %s: %v", cniConfigPath, err)
		}
		cniName = conf.Network.Name
		cniType = conf.Network.Type
	}

	switch cniType {
	case constant.MacvlanCNI:
		cniType = constant.MacvlanCNI
	case constant.IPVlanCNI:
		cniType = constant.IPVlanCNI
	case constant.SriovCNI:
		cniType = constant.SriovCNI
	case constant.IBSriovCNI:
		cniType = constant.IBSriovCNI
	case constant.IPoIBCNI:
		cniType = constant.IPoIBCNI
	default:
		cniType = constant.CustomCNI
	}

	return cniName, cniType, nil
}

func getMultusCniConfig(cniName, cniType string, ns string) *spiderpoolv2beta1.SpiderMultusConfig {
	annotations := make(map[string]string)
	// change calico cni name from k8s-pod-network to calico
	// more datails see:
	// https://github.com/projectcalico/calico/issues/7837
	if cniName == legacyCalicoCniName {
		cniName = calicoCniName
		annotations[constant.AnnoNetAttachConfName] = legacyCalicoCniName
	}
	return &spiderpoolv2beta1.SpiderMultusConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cniName,
			Namespace:   ns,
			Annotations: annotations,
		},
		Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
			CniType:           ptr.To(cniType),
			EnableCoordinator: ptr.To(false),
		},
	}
}
