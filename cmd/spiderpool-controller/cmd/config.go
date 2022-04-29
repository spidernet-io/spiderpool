// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spidernet-io/spiderpool/api/v1/controller/server"
)

var controllerContext = new(ControllerContext)

type envConf struct {
	envName      string
	defaultValue string
	required     bool
	associateKey *string
}

// EnvInfo collects the env and relevant agentContext properties.
// 'required' that means if there's no env value and we set 'required' true, we use the default value.
var EnvInfo = []envConf{
	{"SPIDERPOOL_HTTP_PORT", "5720", true, &controllerContext.HttpPort},
}

type ControllerContext struct {
	// flags
	ConfigDir string

	// env
	LogLevel                   string
	MetricHttpPort             string
	HttpPort                   string
	EnabledPprof               bool
	EnabledMetric              bool
	EnabledGCIppool            bool
	EnabledGCTerminatingPodIP  bool
	GCTerminatingPodIPDuration time.Duration
	EnabledGCEvictedPodIP      bool
	GCEvictedPodIPDuration     time.Duration

	// ControllerManagerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	ControllerManagerCtx    context.Context
	ControllerManagerCancel context.CancelFunc

	// handler
	HttpServer *server.Server
}

// BindControllerDaemonFlags bind controller cli daemon flags
func (cc *ControllerContext) BindControllerDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&cc.ConfigDir, "config-dir", "/tmp/spiderpool/configmap", "config file")
}

// RegisterEnv set the env to GlobalConfiguration
func (cc *ControllerContext) RegisterEnv() {
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
