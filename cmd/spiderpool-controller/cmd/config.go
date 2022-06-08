// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spidernet-io/spiderpool/api/v1/controller/server"
	ctrl "sigs.k8s.io/controller-runtime"
)

var controllerContext = new(ControllerContext)

type envConf struct {
	envName      string
	defaultValue string
	required     bool
	associateKey *string
}

// EnvInfo collects the env and relevant agentContext properties.
var EnvInfo = []envConf{
	{"SPIDERPOOL_HEALTH_PORT", "5720", true, &controllerContext.HttpPort},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5721", true, &controllerContext.MetricHttpPort},
	{"SPIDERPOOL_WEBHOOK_PORT", "5722", true, &controllerContext.WebhookPort},
	{"SPIDERPOOL_CLI_PORT", "5723", true, &controllerContext.CliPort},
}

type ControllerContext struct {
	// flags
	ConfigDir         string
	TlsServerCertPath string
	TlsServerKeyPath  string

	// env
	LogLevel string

	EnabledMetric  bool
	MetricHttpPort string

	HttpPort    string
	WebhookPort string
	CliPort     string

	EnabledPprof bool

	EnabledGCIppool            bool
	EnabledGCTerminatingPodIP  bool
	GCTerminatingPodIPDuration time.Duration
	EnabledGCEvictedPodIP      bool
	GCEvictedPodIPDuration     time.Duration

	// InnerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	InnerCtx    context.Context
	InnerCancel context.CancelFunc

	// handler
	CRDManager ctrl.Manager
	HttpServer *server.Server
}

// BindControllerDaemonFlags bind controller cli daemon flags
func (cc *ControllerContext) BindControllerDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&cc.ConfigDir, "config-dir", "/tmp/spiderpool/configmap", "config file")
	flags.StringVar(&cc.TlsServerCertPath, "tls-server-cert", "", "file path of server cert")
	flags.StringVar(&cc.TlsServerKeyPath, "tls-server-key", "", "file path of server key")
}

// RegisterEnv set the env to GlobalConfiguration
func (cc *ControllerContext) RegisterEnv() {
	for i := range EnvInfo {
		env, ok := os.LookupEnv(EnvInfo[i].envName)
		if ok {
			*(EnvInfo[i].associateKey) = strings.TrimSpace(env)
		} else {
			*(EnvInfo[i].associateKey) = EnvInfo[i].defaultValue
		}
		if EnvInfo[i].required && len(*(EnvInfo[i].associateKey)) == 0 {
			logger.Fatal(fmt.Sprintf("empty value of %s", EnvInfo[i].envName))
		}
	}
}

// verify after retrieve all config
func (cc *ControllerContext) Verify() {
	// TODO(Icarus9913)
	// verify existence and availability for TlsServerCertPath , TlsServerKeyPath , TlsCaPath
}
