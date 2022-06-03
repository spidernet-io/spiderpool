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
	"github.com/spidernet-io/spiderpool/api/v1/agent/server"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"gopkg.in/yaml.v3"
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
	{"SPIDERPOOL_ENABLED_PPROF", "", false, nil, &agentContext.Cfg.EnabledPprof},
	{"SPIDERPOOL_ENABLED_METRIC", "", false, nil, &agentContext.Cfg.EnabledMetric},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5711", true, &agentContext.Cfg.MetricHttpPort, nil},
	{"SPIDERPOOL_HEALTH_PORT", "5710", true, &agentContext.Cfg.HttpPort, nil},
}

type Config struct {
	// flags
	ConfigmapPath string

	// env
	LogLevel       string
	MetricHttpPort string
	HttpPort       string
	EnabledPprof   bool
	EnabledMetric  bool

	// configmap
	IpamUnixSocketPath       string   `yaml:"ipamUnixSocketPath"`
	EnableIPv4               bool     `yaml:"enableIpv4"`
	EnableIPv6               bool     `yaml:"enableIpv6"`
	ClusterDefaultIPv4IPPool []string `yaml:"clusterDefaultIpv4Ippool"`
	ClusterDefaultIPv6IPPool []string `yaml:"clusterDefaultIpv6Ippool"`
	NetworkMode              string   `yaml:"networkMode"`
}

type AgentContext struct {
	Cfg Config

	// ControllerManagerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	ControllerManagerCtx    context.Context
	ControllerManagerCancel context.CancelFunc

	// handler
	HttpServer *server.Server
	UnixServer *server.Server
}

// BindAgentDaemonFlags bind agent cli daemon flags
func (ac *AgentContext) BindAgentDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&ac.Cfg.ConfigmapPath, "config-dir", "/tmp/spiderpool/config-map/conf.yml", "configmap file")
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
	configmapBytes, err := ioutil.ReadFile(ac.Cfg.ConfigmapPath)
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
