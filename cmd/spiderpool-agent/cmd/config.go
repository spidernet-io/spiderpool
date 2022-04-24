// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/pflag"
	"os"
)

var AgentConfig AgentConfiguration

type AgentConfiguration struct {
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

}

// BindAgentDaemonFlags bind agent cli daemon flags
func (ac *AgentConfiguration) BindAgentDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&ac.ConfigDir, "config-dir", "/tmp/spiderpool/configmap", "config file")
	flags.StringVar(&ac.IpamConfigDir, "ipam-config-dir", "", "config file for ipam plugin")
}

// RegisterEnv set the env to AgentConfiguration
func (ac *AgentConfiguration) RegisterEnv() {
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
