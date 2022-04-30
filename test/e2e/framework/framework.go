// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Spiderpool

package framework

import (
	"context"
	"flag"
	"github.com/mohae/deepcopy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type Framework struct {
	// clienset
	KubeClientSet client.Client
	KubeConfig    *rest.Config

	// cluster info
	C *ClusterInfo

	// todo , need docker cli to shutdown node

	// todo, need ssh to node to check something

	t GinkgoTInterface

	ApiOperateTimeout time.Duration
}

type ClusterInfo struct {
	IpV4Enabled       bool
	IpV6Enabled       bool
	MultusEnabled     bool
	SpiderIPAMEnabled bool
	ClusterName       string
	KubeConfigPath    string
}

var clusterInfo = &ClusterInfo{}

// NewFramework init Framework struct
func NewFramework() *Framework {
	f := &Framework{}
	var err error

	f.t = GinkgoT()
	defer GinkgoRecover()

	v := deepcopy.Copy(*clusterInfo)
	t, ok := v.(ClusterInfo)
	Expect(ok).To(BeTrue())
	f.C = &t

	if t.KubeConfigPath == "" {
		Fail("miss KubeConfigPath")
	}
	f.KubeConfig, err = clientcmd.BuildConfigFromFlags("", t.KubeConfigPath)
	Expect(err).NotTo(HaveOccurred())

	f.KubeConfig.QPS = 1000
	f.KubeConfig.Burst = 2000

	scheme := runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = apiextensions_v1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	f.KubeClientSet, err = client.New(f.KubeConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	f.ApiOperateTimeout = 15 * time.Second

	GinkgoWriter.Printf("Framework: %+v \n", f)
	return f
}

func (f *Framework) CreateResource(obj client.Object) {
	ctx1, cancel1 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel1()
	e := f.KubeClientSet.Create(ctx1, obj)
	Expect(e).NotTo(HaveOccurred())
}

func (f *Framework) DeleteResource(obj client.Object) {
	ctx1, cancel1 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel1()
	e := f.KubeClientSet.Create(ctx1, obj)
	Expect(e).NotTo(HaveOccurred())
}

/*
// T exposes a GinkgoTInterface which exposes many of the same methods
// as a *testing.T, for use in tests that previously required a *testing.T.
func (f *Framework) T() GinkgoTInterface {
	return f.t
}

// CreateNamespace creates a namespace with the given name in the
// Kubernetes API or fails the test if it encounters an error.
func (f *Framework) CreateNamespace(namespaceName string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespaceName,
			Labels: map[string]string{"spiderpool-e2e-ns": "true"},
		},
	}
	key := client.ObjectKeyFromObject(ns)

	existing := &corev1.Namespace{}
	err := f.KubeClientSet.Get(context.Background(), key, existing)
	if err == nil && existing.Status.Phase == corev1.NamespaceTerminating {
		// Got an existing namespace and it's terminating: give it a chance to go
		// away.
		Consistently(func(g Gomega) {
			defer GinkgoRecover()
			b := api_errors.IsNotFound(f.KubeClientSet.Get(context.TODO(), key, existing))
			Expect(b).To(BeTrue())
		}, "30s").Should(Succeed())
	}

	Eventually(func(g Gomega) {
		defer GinkgoRecover()
		err := f.KubeClientSet.Create(context.TODO(), ns)
		g.Expect(err).NotTo(HaveOccurred())
	}, "30s").Should(Succeed())

}

type NamespacedTestBody func(string)

func (f *Framework) NamespacedTest(namespace string, body NamespacedTestBody) {
	ginkgo.Context("with namespace: "+namespace, func() {
		ginkgo.BeforeEach(func() {
			f.CreateNamespace(namespace)
		})
		ginkgo.AfterEach(func() {
			f.DeleteNamespace(namespace, false)
		})

		body(namespace)
	})
}
*/

func init() {
	testing.Init()
	flag.BoolVar(&(clusterInfo.IpV4Enabled), "IpV4Enabled", true, "IpV4 Enabled of cluster")
	flag.BoolVar(&(clusterInfo.IpV6Enabled), "IpV6Enabled", true, "IpV6 Enabled of cluster")
	flag.StringVar(&(clusterInfo.KubeConfigPath), "KubeConfigPath", "", "the path to kubeconfig")
	flag.BoolVar(&(clusterInfo.MultusEnabled), "MultusEnabled", true, "Multus Enabled")
	flag.BoolVar(&(clusterInfo.SpiderIPAMEnabled), "SpiderIPAMEnabled", true, "SpiderIPAM Enabled")
	flag.StringVar(&(clusterInfo.ClusterName), "ClusterName", "", "Cluster Name")

	flag.Parse()

}
