// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server"
)

var agentContext = new(AgentContext)

type envConf struct {
	envName      string
	defaultValue string
	required     bool
	associateKey *string
}

// EnvInfo collects the env and relevant agentContext properties.
// 'required' that means if there's no env value and we set 'required' true, we use the default value.
var EnvInfo = []envConf{
	{"SPIDERPOOL_HTTP_PORT", "5710", true, &agentContext.HttpPort},
	{"SPIDER_AGENT_SOCKET_PATH", "/var/tmp/spiderpool.sock", true, &agentContext.SocketPath},
}

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
	SocketPath     string

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
	flags.StringVar(&ac.ConfigDir, "config-dir", "/tmp/spiderpool/configmap", "config file")
	flags.StringVar(&ac.IpamConfigDir, "ipam-config-dir", "", "config file for ipam plugin")
}

// RegisterEnv set the env to AgentConfiguration
func (ac *AgentContext) RegisterEnv() {
	for i := range EnvInfo {
		env, ok := os.LookupEnv(EnvInfo[i].envName)
		if ok {
			*(EnvInfo[i].associateKey) = strings.TrimSpace(env)
		}

		// if no env and required, set it to default value.
		// if no env and none-required, just use the empty value.
		if !ok && EnvInfo[i].required {
			*(EnvInfo[i].associateKey) = EnvInfo[i].defaultValue
		}
	}
}
