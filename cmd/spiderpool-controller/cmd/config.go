// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/pflag"
	"os"
	"time"
)

var ControllerConfig ControllerConfiguration

type ControllerConfiguration struct {
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

	// handler
}

// BindControllerDaemonFlags bind controller cli daemon flags
func (cc *ControllerConfiguration) BindControllerDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&cc.ConfigDir, "config-dir", "/tmp/spiderpool/configmap", "config file")
}

// RegisterEnv set the env to GlobalConfiguration
func (cc *ControllerConfiguration) RegisterEnv() {
	controllerPort := os.Getenv("SPIDERPOOL_HTTP_PORT")
	if controllerPort == "" {
		controllerPort = "5720"
	}
	cc.HttpPort = controllerPort

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
