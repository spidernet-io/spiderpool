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
	"k8s.io/client-go/kubernetes"

	// "k8s.io/apimachinery/pkg/types"
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
	Client        client.WithWatch
	KubeClientSet kubernetes.Interface
	KubeConfig    *rest.Config

	// cluster info
	C ClusterInfo

	// todo , need docker cli to shutdown node

	// todo, need ssh to node to check something

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
	f.KubeConfig, err = clientcmd.BuildConfigFromFlags("", f.C.KubeConfigPath)
	Expect(err).NotTo(HaveOccurred())

	f.KubeConfig.QPS = 1000
	f.KubeConfig.Burst = 2000

	scheme := runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = apiextensions_v1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// f.Client, err = client.New(f.KubeConfig, client.Options{Scheme: scheme})
	f.Client, err = client.NewWithWatch(f.KubeConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	f.ApiOperateTimeout = 15 * time.Second
	f.ResourceDeleteTimeout = 60 * time.Second

	f.KubeClientSet, err = kubernetes.NewForConfig(f.KubeConfig)
	Expect(err).NotTo(HaveOccurred())

	GinkgoWriter.Printf("Framework: %+v \n", f)
	return f
}

// ------------- basic operate
func (f *Framework) CreateResource(obj client.Object, opts ...client.CreateOption) error {
	ctx1, cancel1 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel1()
	return f.Client.Create(ctx1, obj, opts...)
}

func (f *Framework) DeleteResource(obj client.Object, opts ...client.DeleteOption) error {
	ctx2, cancel2 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel2()
	return f.Client.Delete(ctx2, obj, opts...)
}

func (f *Framework) GetResource(key client.ObjectKey, obj client.Object) error {
	ctx3, cancel3 := context.WithTimeout(context.Background(), f.ApiOperateTimeout)
	defer cancel3()
	return f.Client.Get(ctx3, key, obj)
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
		s := fmt.Sprintf("failed to create , a same pod %v/%v exists", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
		Fail(s)
	} else {
		Eventually(func(g Gomega) bool {
			defer GinkgoRecover()
			existing := &corev1.Pod{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if b == false {
				GinkgoWriter.Printf("waiting for a same pod %v/%v to finish deleting \n", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
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
	watchInterface, err := f.Client.Watch(ctx, &corev1.PodList{}, l)
	Expect(err).NotTo(HaveOccurred())
	Expect(watchInterface).NotTo(BeNil())
	defer watchInterface.Stop()

	for {
		select {
		// if pod not exist , got no event
		case event, ok := <-watchInterface.ResultChan():
			if ok == false {
				return nil, fmt.Errorf("channel is closed ")
			} else {
				// GinkgoWriter.Printf("receive event: %+v\n", event)
				GinkgoWriter.Printf("pod %v/%v %v event \n", namespace, name, event.Type)

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
					if ok == false {
						Fail("failed to get metaObject")
					}
					GinkgoWriter.Printf("pod %v/%v status=%+v\n", namespace, name, pod.Status.Phase)
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
			if b == false {
				GinkgoWriter.Printf("waiting for a same namespace %v to finish deleting \n", nsName)
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

// ---------------------------

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
