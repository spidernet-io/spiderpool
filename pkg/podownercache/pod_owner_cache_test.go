// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podownercache

import (
	"context"
	"fmt"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfakecli "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

// Label(K00002)

func TestPodOwnerCache(t *testing.T) {
	fakeCli := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeCli, 0*time.Second)
	informer := factory.Core().V1().Pods().Informer()
	//indexer := informer.GetIndexer()

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

	//err := indexer.Add()
	//if err != nil {
	//	t.Fatal(err)
	//}

	stopCh := make(chan struct{})
	defer close(stopCh)
	go factory.Start(stopCh)

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

	time.Sleep(time.Second * 6)

	res := cache.GetPodByIP("10.6.1.20")
	if res == nil {
		t.Fatal("res is nil")
	}
	if res.OwnerInfo.Namespace != "test-ns" && res.OwnerInfo.Name != "test-deployment" {
		t.Fatal(fmt.Println("res is not equal to test-ns and test-deployment"))
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
