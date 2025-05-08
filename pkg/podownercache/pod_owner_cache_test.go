// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podownercache

import (
	"context"
	"fmt"
	"testing"
	"time"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfakecli "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// Label(K00002)

func TestPodOwnerCache(t *testing.T) {
	fakeCli := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeCli, 0*time.Second)
	informer := factory.Core().V1().Pods().Informer()

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-rs",
				},
			},
		},
		Status: corev1.PodStatus{
			PodIPs: []corev1.PodIP{
				{
					IP: "10.6.1.20",
				},
			},
		},
	}

	noOwnerPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			PodIPs: []corev1.PodIP{
				{
					IP: "10.6.1.21",
				},
			},
		},
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	scheme := kruntime.NewScheme()
	err := corev1.AddToScheme(scheme)
	if err != nil {
		t.Fatal(err)
	}
	err = appv1.AddToScheme(scheme)
	if err != nil {
		t.Fatal(err)
	}

	objs := getMockObjs()
	cli := k8sfakecli.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	cache, err := New(context.Background(), informer, cli)
	if err != nil {
		t.Fatal(err)
	}

	// case add
	_, err = fakeCli.CoreV1().Pods("test-ns").Create(context.Background(), pod, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = fakeCli.CoreV1().Pods("test-ns").Create(context.Background(), noOwnerPod, v1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second * 6)

	res := cache.GetPodByIP("10.6.1.20")
	if res == nil {
		t.Fatal("res is nil")
	}
	if res.OwnerInfo.Namespace != "test-ns" && res.OwnerInfo.Name != "test-deployment" {
		t.Fatal(fmt.Println("res is not equal to test-ns and test-deployment"))
	}

	res = cache.GetPodByIP("10.6.1.21")
	if res == nil {
		t.Fatal("res is nil")
	}

	// case update
	_, err = fakeCli.CoreV1().Pods("test-ns").Update(context.Background(), pod, v1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second * 6)
	res = cache.GetPodByIP("10.6.1.20")
	if res == nil {
		t.Fatal("res is nil")
	}
	if res.OwnerInfo.Namespace != "test-ns" && res.OwnerInfo.Name != "test-deployment" {
		t.Fatal(fmt.Println("res is not equal to test-ns and test-deployment"))
	}

	// case delete
	err = fakeCli.CoreV1().Pods("test-ns").Delete(context.Background(), "test-pod", v1.DeleteOptions{})
	if err != nil {
		t.Fatal("res is nil")
	}
	time.Sleep(time.Second * 6)
	res = cache.GetPodByIP("10.6.1.20")
	if res != nil {
		t.Fatal("res is not nil")
	}

	// case for not exist ip
	res = cache.GetPodByIP("10.6.1.29")
	if res != nil {
		t.Fatal("res is not nil")
	}
}

func getMockObjs() []client.Object {
	return []client.Object{
		&appv1.ReplicaSet{
			TypeMeta: v1.TypeMeta{
				Kind:       "ReplicaSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-rs",
				Namespace: "test-ns",
				OwnerReferences: []v1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						UID:        "test-uid",
					},
				},
			},
		},

		&appv1.Deployment{
			TypeMeta: v1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "test-ns",
			},
		},
	}
}

// MockForbiddenClient is a custom mock implementation of the client.Reader interface
type MockForbiddenClient struct{}

func (m *MockForbiddenClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return errors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "replicasets"}, "test-rs", nil)
}

func (m *MockForbiddenClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

// TestGetFinalOwnerForbidden tests the getFinalOwner method when the Get call returns a forbidden error.
func TestGetFinalOwnerForbidden(t *testing.T) {
	logger = logutils.Logger.Named("PodOwnerCache")

	mockClient := &MockForbiddenClient{}
	cache := &PodOwnerCache{
		ctx:       context.Background(),
		apiReader: mockClient,
	}

	// Create a mock pod object
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-rs",
				},
			},
		},
	}

	ownerInfo, err := cache.getFinalOwner(pod)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ownerInfo != nil {
		t.Fatalf("expected ownerInfo to be nil, got %v", ownerInfo)
	}
}

// MockNotFoundClient is a custom mock implementation of the client.Reader interface
type MockNotFoundClient struct{}

func (m *MockNotFoundClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return errors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "replicasets"}, key.Name)
}

func (m *MockNotFoundClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

// TestGetFinalOwnerNotFound tests the getFinalOwner method when the Get call returns a not found error.
func TestGetFinalOwnerNotFound(t *testing.T) {
	logger = logutils.Logger.Named("PodOwnerCache")

	mockClient := &MockNotFoundClient{}
	cache := &PodOwnerCache{
		ctx:       context.Background(),
		apiReader: mockClient,
	}

	// Create a mock pod object
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-rs",
				},
			},
		},
	}

	ownerInfo, err := cache.getFinalOwner(pod)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ownerInfo != nil {
		t.Fatalf("expected ownerInfo to be nil, got %v", ownerInfo)
	}
}
