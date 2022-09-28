// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v3"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/api/v1/agent/server"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ipam"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var agentContext = new(AgentContext)

const (
	defaultNetworkMode = constant.NetworkLegacy
)

type envConf struct {
	envName          string
	defaultValue     string
	required         bool
	associateStrKey  *string
	associateBoolKey *bool
	associateIntKey  *int
}

// EnvInfo collects the env and relevant agentContext properties.
var envInfo = []envConf{
	{"SPIDERPOOL_LOG_LEVEL", constant.LogInfoLevelStr, true, &agentContext.Cfg.LogLevel, nil, nil},
	{"SPIDERPOOL_ENABLED_METRIC", "false", false, nil, &agentContext.Cfg.EnabledMetric, nil},
	{"SPIDERPOOL_HEALTH_PORT", "5710", true, &agentContext.Cfg.HttpPort, nil, nil},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5711", true, &agentContext.Cfg.MetricHttpPort, nil, nil},
	{"SPIDERPOOL_UPDATE_CR_MAX_RETRYS", "3", false, nil, nil, &agentContext.Cfg.UpdateCRMaxRetrys},
	{"SPIDERPOOL_UPDATE_CR_RETRY_UNIT_TIME", "300", false, nil, nil, &agentContext.Cfg.UpdateCRRetryUnitTime},
	{"SPIDERPOOL_WORKLOADENDPOINT_MAX_HISTORY_RECORDS", "100", true, nil, nil, &agentContext.Cfg.WorkloadEndpointMaxHistoryRecords},
	{"SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS", "5000", true, nil, nil, &agentContext.Cfg.IPPoolMaxAllocatedIPs},
	{"SPIDERPOOL_GOPS_LISTEN_PORT", "5712", false, &agentContext.Cfg.GopsListenPort, nil, nil},
	{"SPIDERPOOL_PYROSCOPE_PUSH_SERVER_ADDRESS", "", false, &agentContext.Cfg.PyroscopeAddress, nil, nil},
	{"SPIDERPOOL_LIMITER_MAX_QUEUE_SIZE", "1000", true, nil, nil, &agentContext.Cfg.LimiterMaxQueueSize},
	{"SPIDERPOOL_LIMITER_MAX_WAIT_TIME", "15", true, nil, nil, &agentContext.Cfg.LimiterMaxWaitTime},
	{"SPIDERPOOL_ENABLED_STATEFULSET", "true", true, nil, &agentContext.Cfg.EnableStatefulSet, nil},
	{"SPIDERPOOL_WAIT_SUBNET_POOL_TIME_IN_SECOND", "1", false, nil, nil, &agentContext.Cfg.WaitSubnetPoolTime},
}

type Config struct {
	// flags
	ConfigPath string

	// env
	LogLevel      string
	EnabledMetric bool

	HttpPort         string
	MetricHttpPort   string
	GopsListenPort   string
	PyroscopeAddress string

	UpdateCRMaxRetrys                 int
	UpdateCRRetryUnitTime             int
	WorkloadEndpointMaxHistoryRecords int
	IPPoolMaxAllocatedIPs             int
	WaitSubnetPoolTime                int

	LimiterMaxQueueSize int
	LimiterMaxWaitTime  int

	// configmap
	IpamUnixSocketPath       string   `yaml:"ipamUnixSocketPath"`
	EnableIPv4               bool     `yaml:"enableIPv4"`
	EnableIPv6               bool     `yaml:"enableIPv6"`
	ClusterDefaultIPv4IPPool []string `yaml:"clusterDefaultIPv4IPPool"`
	ClusterDefaultIPv6IPPool []string `yaml:"clusterDefaultIPv6IPPool"`
	NetworkMode              string   `yaml:"networkMode"`
	EnableStatefulSet        bool     `yaml:"enableStatefulSet"`
	EnableSpiderSubnet       bool     `yaml:"enableSpiderSubnet"`
}

type AgentContext struct {
	Cfg Config

	// InnerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	InnerCtx    context.Context
	InnerCancel context.CancelFunc

	// manager
	IPAM          ipam.IPAM
	CRDManager    ctrl.Manager
	IPPoolManager ippoolmanagertypes.IPPoolManager
	WEManager     workloadendpointmanager.WorkloadEndpointManager
	RIPManager    reservedipmanager.ReservedIPManager
	NodeManager   nodemanager.NodeManager
	NSManager     namespacemanager.NamespaceManager
	PodManager    podmanager.PodManager
	StsManager    statefulsetmanager.StatefulSetManager
	SubnetManager subnetmanagertypes.SubnetManager

	// handler
	HttpServer        *server.Server
	UnixServer        *server.Server
	MetricsHttpServer *http.Server

	// probe
	IsStartupProbe atomic.Bool
}

// BindAgentDaemonFlags bind agent cli daemon flags
func (ac *AgentContext) BindAgentDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&ac.Cfg.ConfigPath, "config-path", "/tmp/spiderpool/config-map/conf.yml", "spiderpool-agent configmap file")
}

// ParseConfiguration set the env to AgentConfiguration
func ParseConfiguration() error {
	var result string

	for i := range envInfo {
		env, ok := os.LookupEnv(envInfo[i].envName)
		if ok {
			result = strings.TrimSpace(env)
		} else {
			// if no env and required, set it to default value.
			result = envInfo[i].defaultValue
		}
		if len(result) == 0 {
			if envInfo[i].required {
				logger.Fatal(fmt.Sprintf("empty value of %s", envInfo[i].envName))
			} else {
				// if no env and none-required, just use the empty value.
				continue
			}
		}

		if envInfo[i].associateStrKey != nil {
			*(envInfo[i].associateStrKey) = result
		} else if envInfo[i].associateBoolKey != nil {
			b, err := strconv.ParseBool(result)
			if nil != err {
				return fmt.Errorf("error: %s require a bool value, but get %s", envInfo[i].envName, result)
			}
			*(envInfo[i].associateBoolKey) = b
		} else if envInfo[i].associateIntKey != nil {
			intVal, err := strconv.Atoi(result)
			if nil != err {
				return fmt.Errorf("error: %s require a int value, but get %s", envInfo[i].envName, result)
			}
			*(envInfo[i].associateIntKey) = intVal
		} else {
			return fmt.Errorf("error: %s doesn't match any controller context", envInfo[i].envName)
		}
	}

	return nil
}

// LoadConfigmap reads configmap data from cli flag config-path
func (ac *AgentContext) LoadConfigmap() error {
	configmapBytes, err := os.ReadFile(ac.Cfg.ConfigPath)
	if nil != err {
		return fmt.Errorf("failed to read configmap file, error: %v", err)
	}

	err = yaml.Unmarshal(configmapBytes, &ac.Cfg)
	if nil != err {
		return fmt.Errorf("failed to parse configmap, error: %v", err)
	}

	if ac.Cfg.IpamUnixSocketPath == "" {
		ac.Cfg.IpamUnixSocketPath = constant.DefaultIPAMUnixSocketPath
	}

	if ac.Cfg.NetworkMode == "" {
		ac.Cfg.NetworkMode = defaultNetworkMode
	} else {
		if ac.Cfg.NetworkMode != constant.NetworkLegacy &&
			ac.Cfg.NetworkMode != constant.NetworkStrict &&
			ac.Cfg.NetworkMode != constant.NetworkSDN {
			return fmt.Errorf("unrecognized network mode %s", ac.Cfg.NetworkMode)
		}
	}

	return nil
}
