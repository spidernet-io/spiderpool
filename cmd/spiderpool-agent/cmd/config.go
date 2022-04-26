// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"

	"github.com/spf13/pflag"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server"
)

var agentContext = new(AgentContext)

type AgentContext struct {
	// flags
	ConfigDir     string
	IpamConfigDir string

	// env
	LogLevel       string
	MetricHttpPort string
	HttpPort       string
	EnabledPprof   bool
	EnabledMetric  bool

	// handler
	HttpServer *server.Server
}

// BindAgentDaemonFlags bind agent cli daemon flags
func (ac *AgentContext) BindAgentDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&ac.ConfigDir, "config-dir", "/tmp/spiderpool/configmap", "config file")
	flags.StringVar(&ac.IpamConfigDir, "ipam-config-dir", "", "config file for ipam plugin")
}

// RegisterEnv set the env to AgentConfiguration
func (ac *AgentContext) RegisterEnv() {
	agentPort := os.Getenv("SPIDERPOOL_HTTP_PORT")
	if agentPort == "" {
		agentPort = "5710"
	}
	ac.HttpPort = agentPort

	// TODO
	//os.Getenv("SPIDERPOOL_METRIC_HTTP_PORT")
	//os.Getenv("SPIDERPOOL_LOG_LEVEL")
	//os.Getenv("SPIDERPOOL_ENABLED_PPROF")
	//os.Getenv("SPIDERPOOL_ENABLED_METRIC")
	//os.Getenv("SPIDERPOOL_GC_IPPOOL_ENABLED")
	//os.Getenv("SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED")
	//os.Getenv("SPIDERPOOL_GC_TERMINATING_POD_IP_DELAY")
	//os.Getenv("SPIDERPOOL_GC_EVICTED_POD_IP_ENABLED")
	//os.Getenv("SPIDERPOOL_GC_EVICTED_POD_IP_DELAY")
}
