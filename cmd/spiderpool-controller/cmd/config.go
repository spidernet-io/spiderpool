// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spidernet-io/spiderpool/api/v1/controller/server"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	ctrl "sigs.k8s.io/controller-runtime"
)

var controllerContext = new(ControllerContext)

type envConf struct {
	envName          string
	defaultValue     string
	required         bool
	associateStrKey  *string
	associateBoolKey *bool
}

// EnvInfo collects the env and relevant agentContext properties.
var envInfo = []envConf{
	{"SPIDERPOOL_LOG_LEVEL", constant.LogInfoLevelStr, true, &controllerContext.Cfg.LogLevel, nil},
	{"SPIDERPOOL_ENABLED_PPROF", "false", false, nil, &controllerContext.Cfg.EnabledPprof},
	{"SPIDERPOOL_ENABLED_METRIC", "false", false, nil, &controllerContext.Cfg.EnabledMetric},
	{"SPIDERPOOL_HEALTH_PORT", "5720", true, &controllerContext.Cfg.HttpPort, nil},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5721", true, &controllerContext.Cfg.MetricHttpPort, nil},
	{"SPIDERPOOL_WEBHOOK_PORT", "5722", true, &controllerContext.Cfg.WebhookPort, nil},
	{"SPIDERPOOL_CLI_PORT", "5723", true, &controllerContext.Cfg.CliPort, nil},
}

type Config struct {
	// flags
	ConfigPath        string
	TlsServerCertPath string
	TlsServerKeyPath  string

	// env
	LogLevel      string
	EnabledPprof  bool
	EnabledMetric bool

	HttpPort       string
	MetricHttpPort string
	WebhookPort    string
	CliPort        string

	// TODO(iiiceoo): Configmap
	EnabledGCIppool            bool
	EnabledGCTerminatingPodIP  bool
	GCTerminatingPodIPDuration time.Duration
	EnabledGCEvictedPodIP      bool
	GCEvictedPodIPDuration     time.Duration
}

type ControllerContext struct {
	Cfg Config

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
	flags.StringVar(&cc.Cfg.ConfigPath, "config-path", "/tmp/spiderpool/config-map/conf.yml", "spiderpool-controller configmap file")
	flags.StringVar(&cc.Cfg.TlsServerCertPath, "tls-server-cert", "", "file path of server cert")
	flags.StringVar(&cc.Cfg.TlsServerKeyPath, "tls-server-key", "", "file path of server key")
}

// RegisterEnv set the env to AgentConfiguration
func (cc *ControllerContext) RegisterEnv() error {
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

// verify after retrieve all config
func (cc *ControllerContext) Verify() {
	// TODO(Icarus9913)
	// verify existence and availability for TlsServerCertPath , TlsServerKeyPath , TlsCaPath
}
