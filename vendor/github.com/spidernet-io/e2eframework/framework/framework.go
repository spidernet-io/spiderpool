// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"strings"
	"time"

	"github.com/mohae/deepcopy"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"
	"os"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	// multus
	MultusDefaultCni    string
	MultusAdditionalCni string
}

var ClusterInformation = &ClusterInfo{}

type envconfig struct {
	EnvName  string
	DestStr  *string
	DestBool *bool
	Default  string
	Required bool
	BoolType bool
}

const (
	E2E_CLUSTER_NAME            = "E2E_CLUSTER_NAME"
	E2E_KUBECONFIG_PATH         = "E2E_KUBECONFIG_PATH"
	E2E_IPV4_ENABLED            = "E2E_IPV4_ENABLED"
	E2E_IPV6_ENABLED            = "E2E_IPV6_ENABLED"
	E2E_MULTUS_CNI_ENABLED      = "E2E_MULTUS_CNI_ENABLED"
	E2E_SPIDERPOOL_IPAM_ENABLED = "E2E_SPIDERPOOL_IPAM_ENABLED"
	E2E_WHEREABOUT_IPAM_ENABLED = "E2E_WHEREABOUT_IPAM_ENABLED"
	E2E_KIND_CLUSTER_NODE_LIST  = "E2E_KIND_CLUSTER_NODE_LIST"
	E2E_Multus_DefaultCni       = "E2E_Multus_DefaultCni"
	E2E_Multus_AdditionalCni    = "E2E_Multus_AdditionalCni"
)

var envConfigList = []envconfig{
	// --- multus field
	{EnvName: E2E_Multus_DefaultCni, DestStr: &ClusterInformation.MultusDefaultCni, Default: "", Required: false},
	{EnvName: E2E_Multus_AdditionalCni, DestStr: &ClusterInformation.MultusAdditionalCni, Default: "", Required: false},
	// --- require field
	{EnvName: E2E_CLUSTER_NAME, DestStr: &ClusterInformation.ClusterName, Default: "", Required: true},
	{EnvName: E2E_KUBECONFIG_PATH, DestStr: &ClusterInformation.KubeConfigPath, Default: "", Required: true},
	// ---- optional field
	{EnvName: E2E_IPV4_ENABLED, DestBool: &ClusterInformation.IpV4Enabled, Default: "true", Required: false},
	{EnvName: E2E_IPV6_ENABLED, DestBool: &ClusterInformation.IpV6Enabled, Default: "true", Required: false},
	{EnvName: E2E_MULTUS_CNI_ENABLED, DestBool: &ClusterInformation.MultusEnabled, Default: "false", Required: false},
	{EnvName: E2E_SPIDERPOOL_IPAM_ENABLED, DestBool: &ClusterInformation.SpiderIPAMEnabled, Default: "false", Required: false},
	{EnvName: E2E_WHEREABOUT_IPAM_ENABLED, DestBool: &ClusterInformation.WhereaboutIPAMEnabled, Default: "false", Required: false},
	// ---- kind field
	{EnvName: E2E_KIND_CLUSTER_NODE_LIST, DestStr: &ClusterInformation.KindNodeListRaw, Default: "false", Required: false},
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

	t         TestingT
	Config    FConfig
	EnableLog bool
}

// -------------------------------------------
type TestingT interface {
	Logf(format string, args ...interface{})
}

var (
	Default_k8sClient_QPS   float32 = 200
	Default_k8sClient_Burst int     = 300

	Default_k8sClient_ApiOperateTimeout     = 15 * time.Second
	Default_k8sClient_ResourceDeleteTimeout = 60 * time.Second
)

// NewFramework init Framework struct
// fakeClient for unitest
func NewFramework(t TestingT, schemeRegisterList []func(*runtime.Scheme) error, fakeClient ...client.WithWatch) (*Framework, error) {

	if t == nil {
		return nil, fmt.Errorf("miss TestingT")
	}

	var err error
	var ok bool

	// defer GinkgoRecover()
	if len(ClusterInformation.ClusterName) == 0 {
		if e := initClusterInfo(); e != nil {
			return nil, e
		}
	}

	f := &Framework{}
	f.t = t
	f.EnableLog = true

	v := deepcopy.Copy(*ClusterInformation)
	f.Info, ok = v.(ClusterInfo)
	if !ok {
		return nil, fmt.Errorf("internal error, failed to deepcopy")
	}

	if fakeClient != nil {
		f.KClient = fakeClient[0]
	} else {
		if f.Info.KubeConfigPath == "" {
			return nil, fmt.Errorf("miss KubeConfig Path")
		}
		f.KConfig, err = clientcmd.BuildConfigFromFlags("", f.Info.KubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("BuildConfigFromFlags failed % v", err)
		}
		f.KConfig.QPS = Default_k8sClient_QPS
		f.KConfig.Burst = Default_k8sClient_Burst

		scheme := runtime.NewScheme()
		err = corev1.AddToScheme(scheme)
		if err != nil {
			return nil, fmt.Errorf("failed to add runtime Scheme : %v", err)
		}

		err = appsv1.AddToScheme(scheme)
		if err != nil {
			return nil, fmt.Errorf("failed to add appsv1 Scheme : %v", err)
		}

		err = batchv1.AddToScheme(scheme)
		if err != nil {
			return nil, fmt.Errorf("failed to add batchv1 Scheme")
		}

		err = apiextensions_v1.AddToScheme(scheme)
		if err != nil {
			return nil, fmt.Errorf("failed to add apiextensions_v1 Scheme : %v", err)
		}
		// f.Client, err = client.New(f.kConfig, client.Options{Scheme: scheme})

		for n, v := range schemeRegisterList {
			if err := v(scheme); err != nil {
				return nil, fmt.Errorf("failed to add schemeRegisterList[%v], reason=%v ", n, err)
			}
		}

		f.KClient, err = client.NewWithWatch(f.KConfig, client.Options{Scheme: scheme})
		if err != nil {
			return nil, fmt.Errorf("failed to new clientset: %v", err)
		}
	}

	f.Config.ApiOperateTimeout = Default_k8sClient_ApiOperateTimeout
	f.Config.ResourceDeleteTimeout = Default_k8sClient_ResourceDeleteTimeout

	f.t.Logf("Framework ClusterInfo: %+v \n", f.Info)
	f.t.Logf("Framework Config: %+v \n", f.Config)

	return f, nil
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

func (f *Framework) ListResource(list client.ObjectList, opts ...client.ListOption) error {
	ctx4, cancel4 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel4()
	return f.KClient.List(ctx4, list, opts...)
}

func (f *Framework) UpdateResource(obj client.Object, opts ...client.UpdateOption) error {
	ctx5, cancel5 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel5()
	return f.KClient.Update(ctx5, obj, opts...)
}

func (f *Framework) UpdateResourceStatus(obj client.Object, opts ...client.UpdateOption) error {
	ctx6, cancel6 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel6()
	return f.KClient.Status().Update(ctx6, obj, opts...)
}

func (f *Framework) PatchResource(obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	ctx7, cancel7 := context.WithTimeout(context.Background(), f.Config.ApiOperateTimeout)
	defer cancel7()
	return f.KClient.Patch(ctx7, obj, patch, opts...)
}

func initClusterInfo() error {

	for _, v := range envConfigList {
		t := os.Getenv(v.EnvName)
		if len(t) == 0 && v.Required {
			return fmt.Errorf("error, failed to get ENV %s", v.EnvName)
		}
		r := v.Default
		if len(t) > 0 {
			r = t
		}
		if v.DestStr != nil {
			*(v.DestStr) = r
		} else {
			if s, err := strconv.ParseBool(r); err != nil {
				return fmt.Errorf("error, %v require a bool value, but get %v", v.EnvName, r)
			} else {
				*(v.DestBool) = s
			}
		}
	}

	if len(ClusterInformation.KindNodeListRaw) > 0 {
		ClusterInformation.KindNodeList = strings.Split(ClusterInformation.KindNodeListRaw, ",")
	}
	return nil

}

func (f *Framework) Log(format string, args ...interface{}) {
	if f.EnableLog {
		f.t.Logf(format, args...)
	}
}
