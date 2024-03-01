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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/api/v1/controller/server"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/gcmanager"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/kubevirtmanager"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
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
	{"GIT_COMMIT_VERSION", "", false, &controllerContext.Cfg.CommitVersion, nil, nil},
	{"GIT_COMMIT_TIME", "", false, &controllerContext.Cfg.CommitTime, nil, nil},
	{"VERSION", "", false, &controllerContext.Cfg.AppVersion, nil, nil},
	{"GOLANG_ENV_MAXPROCS", "8", false, nil, nil, &controllerContext.Cfg.GoMaxProcs},

	{"SPIDERPOOL_LOG_LEVEL", logutils.LogInfoLevelStr, true, &controllerContext.Cfg.LogLevel, nil, nil},
	{"SPIDERPOOL_ENABLED_METRIC", "false", false, nil, &controllerContext.Cfg.EnableMetric, nil},
	{"SPIDERPOOL_ENABLED_DEBUG_METRIC", "false", false, nil, &controllerContext.Cfg.EnableDebugLevelMetric, nil},
	{"SPIDERPOOL_METRIC_RENEW_PERIOD", "120", true, nil, nil, &controllerContext.Cfg.MetricRenewPeriod},
	{"SPIDERPOOL_HEALTH_PORT", "5720", true, &controllerContext.Cfg.HttpPort, nil, nil},
	{"SPIDERPOOL_METRIC_HTTP_PORT", "5721", true, &controllerContext.Cfg.MetricHttpPort, nil, nil},
	{"SPIDERPOOL_WEBHOOK_PORT", "5722", true, &controllerContext.Cfg.WebhookPort, nil, nil},
	{"SPIDERPOOL_GOPS_LISTEN_PORT", "5724", false, &controllerContext.Cfg.GopsListenPort, nil, nil},
	{"SPIDERPOOL_PYROSCOPE_PUSH_SERVER_ADDRESS", "", false, &controllerContext.Cfg.PyroscopeAddress, nil, nil},

	{"SPIDERPOOL_GC_IP_ENABLED", "true", true, nil, &gcIPConfig.EnableGCIP, nil},
	{"SPIDERPOOL_GC_TERMINATING_POD_IP_ENABLED", "true", true, nil, &gcIPConfig.EnableGCForTerminatingPod, nil},
	{"SPIDERPOOL_GC_IP_WORKER_NUM", "3", true, nil, nil, &gcIPConfig.ReleaseIPWorkerNum},
	{"SPIDERPOOL_GC_CHANNEL_BUFFER", "5000", true, nil, nil, &gcIPConfig.GCIPChannelBuffer},
	{"SPIDERPOOL_GC_MAX_PODENTRY_DB_CAP", "100000", true, nil, nil, &gcIPConfig.MaxPodEntryDatabaseCap},
	{"SPIDERPOOL_GC_DEFAULT_INTERVAL_DURATION", "600", true, nil, nil, &gcIPConfig.DefaultGCIntervalDuration},
	{"SPIDERPOOL_GC_TRACE_POD_GAP_DURATION", "5", true, nil, nil, &gcIPConfig.TracePodGapDuration},
	{"SPIDERPOOL_GC_SIGNAL_TIMEOUT_DURATION", "3", true, nil, nil, &gcIPConfig.GCSignalTimeoutDuration},
	{"SPIDERPOOL_GC_HTTP_REQUEST_TIME_GAP", "1", true, nil, nil, &gcIPConfig.GCSignalGapDuration},
	{"SPIDERPOOL_GC_ADDITIONAL_GRACE_DELAY", "0", true, nil, nil, &gcIPConfig.AdditionalGraceDelay},
	{"SPIDERPOOL_GC_PODENTRY_MAX_RETRIES", "5", true, nil, nil, &gcIPConfig.WorkQueueMaxRetries},
	{"SPIDERPOOL_POD_NAMESPACE", "", true, &controllerContext.Cfg.ControllerPodNamespace, nil, nil},
	{"SPIDERPOOL_POD_NAME", "", true, &controllerContext.Cfg.ControllerPodName, nil, nil},
	{"SPIDERPOOL_LEADER_DURATION", "15", true, nil, nil, &controllerContext.Cfg.LeaseDuration},
	{"SPIDERPOOL_LEADER_RENEW_DEADLINE", "10", true, nil, nil, &controllerContext.Cfg.LeaseRenewDeadline},
	{"SPIDERPOOL_LEADER_RETRY_PERIOD", "2", true, nil, nil, &controllerContext.Cfg.LeaseRetryPeriod},
	{"SPIDERPOOL_LEADER_RETRY_GAP", "1", true, nil, nil, &controllerContext.Cfg.LeaseRetryGap},

	{"SPIDERPOOL_IPPOOL_MAX_ALLOCATED_IPS", "5000", false, nil, nil, &controllerContext.Cfg.IPPoolMaxAllocatedIPs},

	{"SPIDERPOOL_SUBNET_INFORMER_RESYNC_PERIOD", "300", false, nil, nil, &controllerContext.Cfg.SubnetInformerResyncPeriod},
	{"SPIDERPOOL_SUBNET_INFORMER_WORKERS", "5", true, nil, nil, &controllerContext.Cfg.SubnetInformerWorkers},
	{"SPIDERPOOL_SUBNET_INFORMER_MAX_WORKQUEUE_LENGTH", "10000", false, nil, nil, &controllerContext.Cfg.SubnetInformerMaxWorkqueueLength},
	{"SPIDERPOOL_SUBNET_APPLICATION_CONTROLLER_WORKERS", "5", true, nil, nil, &controllerContext.Cfg.SubnetAppControllerWorkers},

	{"SPIDERPOOL_COORDINATOR_ENABLED", "false", false, nil, &controllerContext.Cfg.EnableCoordinator, nil},
	{"SPIDERPOOL_COORDINATOR_DEAFULT_NAME", "default", false, &controllerContext.Cfg.DefaultCoordinatorName, nil, nil},
	{"SPIDERPOOL_COORDINATOR_INFORMER_RESYNC_PERIOD", "60", false, nil, nil, &controllerContext.Cfg.CoordinatorInformerResyncPeriod},
	{"SPIDERPOOL_CNI_CONFIG_DIR", "/etc/cni/net.d", false, &controllerContext.Cfg.DefaultCniConfDir, nil, nil},

	{"SPIDERPOOL_MULTUS_CONFIG_ENABLED", "false", false, nil, &controllerContext.Cfg.EnableMultusConfig, nil},
	{"SPIDERPOOL_MULTUS_CONFIG_INFORMER_RESYNC_PERIOD", "60", false, nil, nil, &controllerContext.Cfg.MultusConfigInformerResyncPeriod},
	{"SPIDERPOOL_CILIUM_CONFIGMAP_NAMESPACE_NAME", "kube-system/cilium-config", false, &controllerContext.Cfg.CiliumConfigName, nil, nil},

	{"SPIDERPOOL_IPPOOL_INFORMER_RESYNC_PERIOD", "300", false, nil, nil, &controllerContext.Cfg.IPPoolInformerResyncPeriod},
	{"SPIDERPOOL_IPPOOL_INFORMER_WORKERS", "3", true, nil, nil, &controllerContext.Cfg.IPPoolInformerWorkers},
	{"SPIDERPOOL_AUTO_IPPOOL_HANDLER_MAX_WORKQUEUE_LENGTH", "10000", true, nil, nil, &controllerContext.Cfg.IPPoolInformerMaxWorkQueueLength},
	{"SPIDERPOOL_WORKQUEUE_MAX_RETRIES", "500", true, nil, nil, &controllerContext.Cfg.WorkQueueMaxRetries},
	{"SPIDERPOOL_WORKQUEUE_RETRY_DELAY_DURATION", "5", true, nil, nil, &controllerContext.Cfg.WorkQueueRequeueDelayDuration},
}

type Config struct {
	CommitVersion string
	CommitTime    string
	AppVersion    string
	GoMaxProcs    int

	// flags
	ConfigPath        string
	TlsServerCertPath string
	TlsServerKeyPath  string

	// env
	LogLevel               string
	EnableMetric           bool
	EnableDebugLevelMetric bool
	MetricRenewPeriod      int

	HttpPort          string
	MetricHttpPort    string
	WebhookPort       string
	GopsListenPort    string
	PyroscopeAddress  string
	DefaultCniConfDir string
	// CiliumConfigName is formatted by namespace and name,default is kube-system/cilium-config
	CiliumConfigName string

	ControllerPodNamespace string
	ControllerPodName      string
	DefaultCoordinatorName string
	LeaseDuration          int
	LeaseRenewDeadline     int
	LeaseRetryPeriod       int
	LeaseRetryGap          int

	IPPoolMaxAllocatedIPs int

	SubnetInformerResyncPeriod       int
	SubnetInformerWorkers            int
	SubnetInformerMaxWorkqueueLength int
	SubnetAppControllerWorkers       int

	IPPoolInformerResyncPeriod       int
	IPPoolInformerWorkers            int
	IPPoolInformerMaxWorkQueueLength int
	WorkQueueMaxRetries              int
	WorkQueueRequeueDelayDuration    int

	CoordinatorInformerResyncPeriod int

	EnableMultusConfig               bool
	EnableCoordinator                bool
	MultusConfigInformerResyncPeriod int

	// configmap
	EnableIPv4                        bool `yaml:"enableIPv4"`
	EnableIPv6                        bool `yaml:"enableIPv6"`
	EnableStatefulSet                 bool `yaml:"enableStatefulSet"`
	EnableKubevirtStaticIP            bool `yaml:"enableKubevirtStaticIP"`
	EnableSpiderSubnet                bool `yaml:"enableSpiderSubnet"`
	ClusterSubnetDefaultFlexibleIPNum int  `yaml:"clusterSubnetDefaultFlexibleIPNumber"`
}

type ControllerContext struct {
	Cfg Config

	// InnerCtx is the context that can be used during shutdown.
	// It will be cancelled after receiving an interrupt or termination signal.
	InnerCtx    context.Context
	InnerCancel context.CancelFunc

	// kubernetes Clientset
	ClientSet     *kubernetes.Clientset
	DynamicClient *dynamic.DynamicClient

	// manager
	CRDManager        ctrl.Manager
	SubnetManager     subnetmanager.SubnetManager
	IPPoolManager     ippoolmanager.IPPoolManager
	EndpointManager   workloadendpointmanager.WorkloadEndpointManager
	ReservedIPManager reservedipmanager.ReservedIPManager
	NodeManager       nodemanager.NodeManager
	NSManager         namespacemanager.NamespaceManager
	PodManager        podmanager.PodManager
	GCManager         gcmanager.GCManager
	StsManager        statefulsetmanager.StatefulSetManager
	KubevirtManager   kubevirtmanager.KubevirtManager
	Leader            election.SpiderLeaseElector

	// handler
	HttpServer        *server.Server
	MetricsHttpServer *http.Server

	// webhook http client
	webhookClient *http.Client

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
