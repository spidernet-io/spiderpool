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
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/api/v1/controller/server"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/gcmanager"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var controllerContext = new(ControllerContext)

var gcIPConfig = new(gcmanager.GarbageCollectionConfig)

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
	{"SPIDERPOOL_LOG_LEVEL", constant.LogInfoLevelStr, true, &controllerContext.Cfg.LogLevel, nil, nil},
	{"SPIDERPOOL_ENABLED_METRIC", "false", false, nil, &controllerContext.Cfg.EnabledMetric, nil},
	{"SPIDERPOOL_HEALTH_PORT", "5720", true, &controllerContext.Cfg.HttpPort, nil, nil},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5721", true, &controllerContext.Cfg.MetricHttpPort, nil, nil},
	{"SPIDERPOOL_WEBHOOK_PORT", "5722", true, &controllerContext.Cfg.WebhookPort, nil, nil},
	{"SPIDERPOOL_GOPS_LISTEN_PORT", "5724", false, &controllerContext.Cfg.GopsListenPort, nil, nil},
	{"SPIDERPOOL_PYROSCOPE_PUSH_SERVER_ADDRESS", "", false, &controllerContext.Cfg.PyroscopeAddress, nil, nil},
	{"SPIDERPOOL_WORKLOADENDPOINT_MAX_HISTORY_RECORDS", "100", false, nil, nil, &controllerContext.Cfg.WorkloadEndpointMaxHistoryRecords},
	{"SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS", "5000", false, nil, nil, &controllerContext.Cfg.IPPoolMaxAllocatedIPs},
	{"SPIDERPOOL_UPDATE_CR_MAX_RETRIES", "3", false, nil, nil, &controllerContext.Cfg.UpdateCRMaxRetries},
	{"SPIDERPOOL_UPDATE_CR_RETRY_UNIT_TIME", "500", false, nil, nil, &controllerContext.Cfg.UpdateCRRetryUnitTime},
	{"SPIDERPOOL_GC_IP_ENABLED", "true", true, nil, &gcIPConfig.EnableGCIP, nil},
	{"SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED", "true", true, nil, &gcIPConfig.EnableGCForTerminatingPod, nil},
	{"SPIDERPOOL_GC_IP_WORKER_NUM", "3", true, nil, nil, &gcIPConfig.ReleaseIPWorkerNum},
	{"SPIDERPOOL_GC_CHANNEL_BUFFER", "5000", true, nil, nil, &gcIPConfig.GCIPChannelBuffer},
	{"SPIDERPOOL_GC_MAX_PODENTRY_DB_CAP", "100000", true, nil, nil, &gcIPConfig.MaxPodEntryDatabaseCap},
	{"SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION", "600", true, nil, nil, &gcIPConfig.DefaultGCIntervalDuration},
	{"SPIDERPOOL_GC_TRACE_POD_GAP_DURATION", "5", true, nil, nil, &gcIPConfig.TracePodGapDuration},
	{"SPIDERPOOL_GC_SIGNAL_TIMEOUT_DURATION", "3", true, nil, nil, &gcIPConfig.GCSignalTimeoutDuration},
	{"SPIDERPOOL_GC_HTTP_REQUEST_TIME_GAP", "1", true, nil, nil, &gcIPConfig.GCSignalGapDuration},
	{"SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY", "5", true, nil, nil, &gcIPConfig.AdditionalGraceDelay},
	{"SPIDERPOOL_POD_NAMESPACE", "", true, &controllerContext.Cfg.ControllerPodNamespace, nil, nil},
	{"SPIDERPOOL_POD_NAME", "", true, &controllerContext.Cfg.ControllerPodName, nil, nil},
	{"SPIDERPOOL_GC_LEADER_DURATION", "15", true, nil, nil, &controllerContext.Cfg.LeaseDuration},
	{"SPIDERPOOL_GC_LEADER_RENEW_DEADLINE", "10", true, nil, nil, &controllerContext.Cfg.LeaseRenewDeadline},
	{"SPIDERPOOL_GC_LEADER_RETRY_PERIOD", "2", true, nil, nil, &controllerContext.Cfg.LeaseRetryPeriod},
	{"SPIDERPOOL_GC_LEADER_RETRY_GAP", "1", true, nil, nil, &controllerContext.Cfg.LeaseRetryGap},
}

type Config struct {
	ControllerPodNamespace string
	ControllerPodName      string

	// flags
	ConfigPath        string
	TlsServerCertPath string
	TlsServerKeyPath  string

	// env
	LogLevel      string
	EnabledMetric bool

	HttpPort       string
	MetricHttpPort string
	WebhookPort    string

	GopsListenPort   string
	PyroscopeAddress string

	UpdateCRMaxRetries                int
	UpdateCRRetryUnitTime             int
	WorkloadEndpointMaxHistoryRecords int
	IPPoolMaxAllocatedIPs             int

	LeaseDuration      int
	LeaseRenewDeadline int
	LeaseRetryPeriod   int
	LeaseRetryGap      int

	// configmap
	EnableIPv4         bool `yaml:"enableIPv4"`
	EnableIPv6         bool `yaml:"enableIPv6"`
	EnableStatefulSet  bool `yaml:"enableStatefulSet"`
	EnableSpiderSubnet bool `yaml:"enableSpiderSubnet"`
}

type ControllerContext struct {
	Cfg Config

	// InnerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	InnerCtx    context.Context
	InnerCancel context.CancelFunc

	// kubernetes Clientset
	ClientSet *kubernetes.Clientset

	// K8s event recorder.
	Recorder record.EventRecorder

	// manager
	CRDManager    ctrl.Manager
	SubnetManager subnetmanagertypes.SubnetManager
	IPPoolManager ippoolmanagertypes.IPPoolManager
	WEPManager    workloadendpointmanager.WorkloadEndpointManager
	RIPManager    reservedipmanager.ReservedIPManager
	NodeManager   nodemanager.NodeManager
	NSManager     namespacemanager.NamespaceManager
	PodManager    podmanager.PodManager
	GCManager     gcmanager.GCManager
	StsManager    statefulsetmanager.StatefulSetManager
	Leader        election.SpiderLeaseElector

	// handler
	HttpServer        *server.Server
	MetricsHttpServer *http.Server

	// probe
	IsStartupProbe atomic.Bool
}

// BindControllerDaemonFlags bind controller cli daemon flags
func (cc *ControllerContext) BindControllerDaemonFlags(flags *pflag.FlagSet) {
	flags.StringVar(&cc.Cfg.ConfigPath, "config-path", "/tmp/spiderpool/config-map/conf.yml", "spiderpool-controller configmap file")
	flags.StringVar(&cc.Cfg.TlsServerCertPath, "tls-server-cert", "", "file path of server cert")
	flags.StringVar(&cc.Cfg.TlsServerKeyPath, "tls-server-key", "", "file path of server key")
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

// verify after retrieve all config
func (cc *ControllerContext) Verify() {
	// TODO(Icarus9913)
	// verify existence and availability for TlsServerCertPath , TlsServerKeyPath , TlsCaPath
}

// LoadConfigmap reads configmap data from cli flag config-path
func (cc *ControllerContext) LoadConfigmap() error {
	configmapBytes, err := os.ReadFile(cc.Cfg.ConfigPath)
	if nil != err {
		return fmt.Errorf("failed to read configmap file, error: %v", err)
	}

	err = yaml.Unmarshal(configmapBytes, &cc.Cfg)
	if nil != err {
		return fmt.Errorf("failed to parse configmap, error: %v", err)
	}

	return nil
}
