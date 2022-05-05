// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package e2eframework

import (
	"context"
	"github.com/mohae/deepcopy"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"os"
	"strconv"
)

// -----------------------------

type ClusterInfo struct {
	IpV4Enabled           bool
	IpV6Enabled           bool
	MultusEnabled         bool
	SpiderIPAMEnabled     bool
	WhereaboutIPAMEnabled bool
	ClusterName           string
	KubeConfigPath        string
	// docker container name for kind cluster
	KindNodeList    []string
	KindNodeListRaw string
}

var clusterInfo = &ClusterInfo{}

type envconfig struct {
	EnvName  string
	DestStr  *string
	DestBool *bool
	Default  string
	Required bool
	BoolType bool
}

var envConfigList = []envconfig{
	// --- require field
	envconfig{EnvName: "E2E_CLUSTER_NAME", DestStr: &clusterInfo.ClusterName, Default: "", Required: true},
	envconfig{EnvName: "E2E_KUBECONFIG_PATH", DestStr: &clusterInfo.KubeConfigPath, Default: "", Required: true},
	// ---- optional field
	envconfig{EnvName: "E2E_IPV4_ENABLED", DestBool: &clusterInfo.IpV4Enabled, Default: "true", Required: false},
	envconfig{EnvName: "E2E_IPV6_ENABLED", DestBool: &clusterInfo.IpV6Enabled, Default: "true", Required: false},
	envconfig{EnvName: "E2E_MULTUS_CNI_ENABLED", DestBool: &clusterInfo.MultusEnabled, Default: "false", Required: false},
	envconfig{EnvName: "E2E_SPIDERPOOL_IPAM_ENABLED", DestBool: &clusterInfo.SpiderIPAMEnabled, Default: "false", Required: false},
	envconfig{EnvName: "E2E_WHEREABOUT_IPAM_ENABLED", DestBool: &clusterInfo.WhereaboutIPAMEnabled, Default: "false", Required: false},
	// ---- kind field
	envconfig{EnvName: "E2E_KIND_CLUSTER_NODE_LIST", DestStr: &clusterInfo.KindNodeListRaw, Default: "false", Required: false},
	// ---- vagrant field
}

// -------------------------------------------

type FConfig struct {
	ApiOperateTimeout     time.Duration
	ResourceDeleteTimeout time.Duration
}

type Framework struct {
	// clienset
	KClient client.WithWatch
	KConfig *rest.Config

	// cluster info
	Info ClusterInfo

	t      TestingT
	Config FConfig
}

// -------------------------------------------
type TestingT interface {
	Fatalf(format string, args ...interface{})
	Logf(format string, args ...interface{})
}

var (
	Default_k8sClient_QPS   float32 = 200
	Default_k8sClient_Burst int     = 300

	Default_k8sClient_ApiOperateTimeout     = 15 * time.Second
	Default_k8sClient_ResourceDeleteTimeout = 60 * time.Second
)

// NewFramework init Framework struct
func NewFramework(t TestingT) *Framework {

	if t == nil {
		return nil
	}

	var err error
	var ok bool

	// defer GinkgoRecover()
	if len(clusterInfo.ClusterName) == 0 {
		initClusterInfo(t)
	}

	f := &Framework{}
	f.t = t

	v := deepcopy.Copy(*clusterInfo)
	f.Info, ok = v.(ClusterInfo)
	if ok == false {
		f.t.Fatalf("internal error, failed to deepcopy")
	}

	if f.Info.KubeConfigPath == "" {
		f.t.Fatalf("miss KubeConfig Path")
	}
	f.KConfig, err = clientcmd.BuildConfigFromFlags("", f.Info.KubeConfigPath)
	if err != nil {
		f.t.Fatalf("BuildConfigFromFlags failed % v", err)
	}

	f.KConfig.QPS = Default_k8sClient_QPS
	f.KConfig.Burst = Default_k8sClient_Burst

	scheme := runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	if err != nil {
		f.t.Fatalf("failed to add runtime Scheme : %v", err)
	}
	err = apiextensions_v1.AddToScheme(scheme)
	if err != nil {
		f.t.Fatalf("failed to add apiextensions_v1 Scheme : %v", err)
	}

	// f.Client, err = client.New(f.kConfig, client.Options{Scheme: scheme})
	f.KClient, err = client.NewWithWatch(f.KConfig, client.Options{Scheme: scheme})
	if err != nil {
		f.t.Fatalf("failed to new clientset: %v", err)
	}

	f.Config.ApiOperateTimeout = Default_k8sClient_ApiOperateTimeout
	f.Config.ResourceDeleteTimeout = Default_k8sClient_ResourceDeleteTimeout

	f.t.Logf("Framework ClusterInfo: %+v \n", f.Info)
	f.t.Logf("Framework Config: %+v \n", f.Config)

	return f
}

// ------------- basic operate

func (f *Framework) CreateResource(obj client.Object, opts ...client.CreateOption) error {
	ctx1, cancel1 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel1()
	return f.KClient.Create(ctx1, obj, opts...)
}

func (f *Framework) DeleteResource(obj client.Object, opts ...client.DeleteOption) error {
	ctx2, cancel2 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel2()
	return f.KClient.Delete(ctx2, obj, opts...)
}

func (f *Framework) GetResource(key client.ObjectKey, obj client.Object) error {
	ctx3, cancel3 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel3()
	return f.KClient.Get(ctx3, key, obj)
}

// ------------- for replicaset , to do

// ------------- for deployment , to do

// ------------- for statefulset , to do

// ------------- for job , to do

// ------------- for daemonset , to do

// ------------- for namespace

// ------------- shutdown node , to do

// ------------- docker exec command to kind node

func initClusterInfo(q TestingT) {

	for _, v := range envConfigList {
		t := os.Getenv(v.EnvName)
		if len(t) == 0 && v.Required == true {
			q.Fatalf("error, failed to get ENV %s", v.EnvName)
		}
		r := v.Default
		if len(t) > 0 {
			r = t
		}
		if v.DestStr != nil {
			*(v.DestStr) = r
		} else {
			if s, err := strconv.ParseBool(r); err != nil {
				q.Fatalf("error, %v require a bool value, but get %v", v.EnvName, r)
			} else {
				*(v.DestBool) = s
			}
		}
	}

	if len(clusterInfo.KindNodeListRaw) > 0 {
		clusterInfo.KindNodeList = strings.Split(clusterInfo.KindNodeListRaw, ",")
	}

}
