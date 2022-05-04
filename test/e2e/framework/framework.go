// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"flag"
	"fmt"
	"github.com/mohae/deepcopy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
)

type Framework struct {
	// clienset
	kClient client.WithWatch
	kConfig *rest.Config

	// cluster info
	C ClusterInfo

	t GinkgoTInterface

	ApiOperateTimeout     time.Duration
	ResourceDeleteTimeout time.Duration
}

type ClusterInfo struct {
	IpV4Enabled       bool
	IpV6Enabled       bool
	MultusEnabled     bool
	SpiderIPAMEnabled bool
	ClusterName       string
	KubeConfigPath    string
	// docker container name for kind cluster
	KindNodeList []string
}

var clusterInfo = &ClusterInfo{}

// NewFramework init Framework struct
func NewFramework() *Framework {
	f := &Framework{}
	var err error
	var ok bool

	f.t = GinkgoT()
	defer GinkgoRecover()

	v := deepcopy.Copy(*clusterInfo)
	f.C, ok = v.(ClusterInfo)
	Expect(ok).To(BeTrue())

	if f.C.KubeConfigPath == "" {
		Fail("miss KubeConfigPath")
	}
	f.kConfig, err = clientcmd.BuildConfigFromFlags("", f.C.KubeConfigPath)
	Expect(err).NotTo(HaveOccurred())

	f.kConfig.QPS = 100
	f.kConfig.Burst = 200

	scheme := runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = apiextensions_v1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// f.Client, err = client.New(f.kConfig, client.Options{Scheme: scheme})
	f.kClient, err = client.NewWithWatch(f.kConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	f.ApiOperateTimeout = 15 * time.Second
	f.ResourceDeleteTimeout = 60 * time.Second

	// generate a uniq namespace pod for test
	// namespace must be: lower case alphanumeric characters or '-', and must start and end with an alphanumeric character

	f.t = GinkgoT()
	GinkgoWriter.Printf("Framework: %+v \n", f)
	return f
}

// T exposes a GinkgoTInterface which exposes many of the same methods
// as a *testing.T, for use in tests that previously required a *testing.T.
func (f *Framework) T() GinkgoTInterface {
	return f.t
}

func (f *Framework) RandomName() string {
	m := time.Now()
	return fmt.Sprintf("%v%v-%v", m.Minute(), m.Second(), m.Nanosecond())
}

func (f *Framework) By(format string, arg ...interface{}) {
	t := fmt.Sprintf(format, arg...)
	By(t)
}

func (f *Framework) Fail(format string, arg ...interface{}) {
	t := fmt.Sprintf(format, arg...)
	Fail(t)
}

// ------------- basic operate
func (f *Framework) CreateResource(obj client.Object, opts ...client.CreateOption) error {
	ctx1, cancel1 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel1()
	return f.kClient.Create(ctx1, obj, opts...)
}

func (f *Framework) DeleteResource(obj client.Object, opts ...client.DeleteOption) error {
	ctx2, cancel2 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel2()
	return f.kClient.Delete(ctx2, obj, opts...)
}

func (f *Framework) GetResource(key client.ObjectKey, obj client.Object) error {
	ctx3, cancel3 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel3()
	return f.kClient.Get(ctx3, key, obj)
}

// ------------- for pod

func (f *Framework) CreatePod(pod *corev1.Pod, opts ...client.CreateOption) error {

	// try to wait for finish last deleting
	fake := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.ObjectMeta.Namespace,
			Name:      pod.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &corev1.Pod{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		f.Fail("failed to create , a same pod %v/%v exists", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	} else {
		Eventually(func(g Gomega) bool {
			defer GinkgoRecover()
			existing := &corev1.Pod{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				f.By("waiting for a same pod %v/%v to finish deleting \n", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
			}
			return b
		}).WithTimeout(f.ResourceDeleteTimeout).WithPolling(2 * time.Second).Should(BeTrue())
	}

	return f.CreateResource(pod, opts...)
}

func (f *Framework) DeletePod(name, namespace string, opts ...client.DeleteOption) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(pod, opts...)
}

func (f *Framework) GetPod(name, namespace string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(pod)
	existing := &corev1.Pod{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) WaitPodStarted(name, namespace string, ctx context.Context) (*corev1.Pod, error) {

	// refer to https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/client/watch_test.go
	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.kClient.Watch(ctx, &corev1.PodList{}, l)
	Expect(err).NotTo(HaveOccurred())
	Expect(watchInterface).NotTo(BeNil())
	defer watchInterface.Stop()

	for {
		select {
		// if pod not exist , got no event
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return nil, fmt.Errorf("channel is closed ")
			} else {
				f.By("pod %v/%v %v event \n", namespace, name, event.Type)

				// Added    EventType = "ADDED"
				// Modified EventType = "MODIFIED"
				// Deleted  EventType = "DELETED"
				// Bookmark EventType = "BOOKMARK"
				// Error    EventType = "ERROR"
				if event.Type == watch.Error {
					return nil, fmt.Errorf("received error event: %+v", event)
				} else if event.Type == watch.Deleted {
					return nil, fmt.Errorf("resource is deleted")
				} else {
					pod, ok := event.Object.(*corev1.Pod)
					// metaObject, ok := event.Object.(metav1.Object)
					if !ok {
						Fail("failed to get metaObject")
					}
					f.By("pod %v/%v status=%+v\n", namespace, name, pod.Status.Phase)
					if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
						break
					} else {
						return pod, nil
					}
				}
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx timeout ")
		}
	}
}

// ------------- for replicaset , to do

// ------------- for deployment , to do

// ------------- for statefulset , to do

// ------------- for job , to do

// ------------- for daemonset , to do

// ------------- for namespace

func (f *Framework) CreateNamespace(nsName string, opts ...client.CreateOption) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nsName,
			Labels: map[string]string{"spiderpool-e2e-ns": "true"},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
	}

	key := client.ObjectKeyFromObject(ns)
	existing := &corev1.Namespace{}
	e := f.GetResource(key, existing)

	if e == nil && existing.Status.Phase == corev1.NamespaceTerminating {
		Eventually(func(g Gomega) {
			defer GinkgoRecover()
			existing := &corev1.Namespace{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				f.By("waiting for a same namespace %v to finish deleting \n", nsName)
			}
			Expect(b).To(BeTrue())
		}).WithTimeout(f.ResourceDeleteTimeout).WithPolling(2 * time.Second).Should(BeTrue())
	}
	return f.CreateResource(ns, opts...)
}

func (f *Framework) DeleteNamespace(nsName string, opts ...client.DeleteOption) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
	return f.DeleteResource(ns, opts...)
}

// ------------- shutdown node , to do

// ------------- docker exec command to kind node

// ---------------------------

func init() {
	var nodeList string
	testing.Init()
	flag.BoolVar(&(clusterInfo.IpV4Enabled), "IpV4Enabled", true, "IpV4 Enabled of cluster")
	flag.BoolVar(&(clusterInfo.IpV6Enabled), "IpV6Enabled", true, "IpV6 Enabled of cluster")
	flag.StringVar(&(clusterInfo.KubeConfigPath), "KubeConfigPath", "", "the path to kubeConfig")
	flag.BoolVar(&(clusterInfo.MultusEnabled), "MultusEnabled", true, "Multus Enabled")
	flag.BoolVar(&(clusterInfo.SpiderIPAMEnabled), "SpiderIPAMEnabled", true, "SpiderIPAM Enabled")
	flag.StringVar(&(clusterInfo.ClusterName), "ClusterName", "", "Cluster Name")
	flag.StringVar(&nodeList, "ClusterNodeList", "", "Cluster kind node list")

	flag.Parse()

	clusterInfo.KindNodeList = strings.Split(nodeList, ",")
	if len(clusterInfo.KindNodeList) == 0 {
		Fail("error, fail to get kind nodes")
	}

}
