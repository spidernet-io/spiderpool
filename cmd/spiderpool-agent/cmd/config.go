// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/api/v1/agent/server"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ipam"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
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
}

// EnvInfo collects the env and relevant agentContext properties.
var envInfo = []envConf{
	{"SPIDERPOOL_LOG_LEVEL", constant.LogInfoLevelStr, true, &agentContext.Cfg.LogLevel, nil},
	{"SPIDERPOOL_ENABLED_METRIC", "false", false, nil, &agentContext.Cfg.EnabledMetric},
	{"SPIDERPOOL_HEALTH_PORT", "5710", true, &agentContext.Cfg.HttpPort, nil},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5711", true, &agentContext.Cfg.MetricHttpPort, nil},
	{"SPIDERPOOL_UPDATE_CR_MAX_RETRYS", "3", false, &agentContext.Cfg.UpdateCRMaxRetrys, nil},
	{"SPIDERPOOL_WORKLOADENDPOINT_MAX_HISTORY_RECORDS", "100", false, &agentContext.Cfg.WorkloadEndpointMaxHistoryRecords, nil},
	{"SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS", "5000", false, &agentContext.Cfg.IPPoolMaxAllocatedIPs, nil},
	{"SPIDERPOOL_GOPS_LISTEN_PORT", "5712", false, &agentContext.Cfg.GopsListenPort, nil},
	{"SPIDERPOOL_PYROSCOPE_PUSH_SERVER_ADDRESS", "", false, &agentContext.Cfg.PyroscopeAddress, nil},
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

	UpdateCRMaxRetrys                 string
	WorkloadEndpointMaxHistoryRecords string
	IPPoolMaxAllocatedIPs             string

	// configmap
	IpamUnixSocketPath       string   `yaml:"ipamUnixSocketPath"`
	EnableIPv4               bool     `yaml:"enableIPv4"`
	EnableIPv6               bool     `yaml:"enableIPv6"`
	ClusterDefaultIPv4IPPool []string `yaml:"clusterDefaultIPv4IPPool"`
	ClusterDefaultIPv6IPPool []string `yaml:"clusterDefaultIPv6IPPool"`
	NetworkMode              string   `yaml:"networkMode"`
}

type AgentContext struct {
	Cfg Config

	// InnerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	InnerCtx    context.Context
	InnerCancel context.CancelFunc

	// manager
	IPAM          ipam.IPAM
	IPPoolManager ippoolmanager.IPPoolManager
	WEManager     workloadendpointmanager.WorkloadEndpointManager
	RIPManager    reservedipmanager.ReservedIPManager
	NodeManager   nodemanager.NodeManager
	NSManager     namespacemanager.NamespaceManager
	PodManager    podmanager.PodManager

	// handler
	CRDManager ctrl.Manager
	HttpServer *server.Server
	UnixServer *server.Server
}

// BindAgentDaemonFlags bind agent cli daemon flags
func (ac *AgentContext) BindAgentDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&ac.Cfg.ConfigPath, "config-path", "/tmp/spiderpool/config-map/conf.yml", "spiderpool-agent configmap file")
}

// RegisterEnv set the env to AgentConfiguration
func (ac *AgentContext) RegisterEnv() error {
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

		if nil != envInfo[i].associateStrKey {
			*(envInfo[i].associateStrKey) = result
		} else {
			b, err := strconv.ParseBool(result)
			if nil != err {
				return fmt.Errorf("Error: %s require a bool value, but get %s", envInfo[i].envName, result)
			}
			*(envInfo[i].associateBoolKey) = b
		}
	}

	return nil
}

// LoadConfigmap reads configmap data from cli flag config-path
func (ac *AgentContext) LoadConfigmap() error {
	configmapBytes, err := ioutil.ReadFile(ac.Cfg.ConfigPath)
	if nil != err {
		return fmt.Errorf("Read configmap file failed: %v", err)
	}

	err = yaml.Unmarshal(configmapBytes, &ac.Cfg)
	if nil != err {
		return fmt.Errorf("Parse configmap failed: %v", err)
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
			return fmt.Errorf("Error: Unrecognized network mode %s", ac.Cfg.NetworkMode)
		}
	}

	return nil
}
