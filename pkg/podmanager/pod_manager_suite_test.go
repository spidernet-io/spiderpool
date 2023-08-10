// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kruiseapi "github.com/openkruise/kruise-api"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var tracker k8stesting.ObjectTracker
var fakeAPIReader client.Reader
var fakeDynamicClient *dynamicfake.FakeDynamicClient
var podManager podmanager.PodManager

const (
	testNamespace = "testns"
	testName      = "testname"
)

var (
	cloneSet = &kruisev1.CloneSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CloneSet",
			APIVersion: kruisev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	}

	orphanReplicaSet = &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: pointer.Int32(2),
		},
	}

	replicaSet, deployment = func() (*appsv1.ReplicaSet, *appsv1.Deployment) {
		tmpReplicaSet := &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-1", testName),
				Namespace: testNamespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         appsv1.SchemeGroupVersion.String(),
					Kind:               constant.KindDeployment,
					Name:               testName,
					BlockOwnerDeletion: pointer.Bool(true),
					Controller:         pointer.Bool(true),
				}},
			},
			Spec: appsv1.ReplicaSetSpec{
				Replicas: pointer.Int32(2),
			},
		}
		tmpDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32(2),
			},
		}
		return tmpReplicaSet, tmpDeployment
	}()

	statefulSet = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(2),
		},
	}

	daemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	}

	orphanJob = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
	}

	job, cronJob = func() (*batchv1.Job, *batchv1.CronJob) {
		tmpJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-1", testName),
				Namespace: testNamespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         batchv1.SchemeGroupVersion.String(),
					Kind:               constant.KindCronJob,
					Name:               testName,
					BlockOwnerDeletion: pointer.Bool(true),
					Controller:         pointer.Bool(true),
				}},
			},
		}
		tmpCronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNamespace,
			},
		}
		return tmpJob, tmpCronJob
	}()
)

func TestPodManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodManager Suite", Label("podmanager", "unitest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = kruiseapi.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&corev1.Pod{}, metav1.ObjectNameField, func(raw client.Object) []string {
			pod := raw.(*corev1.Pod)
			return []string{pod.GetObjectMeta().GetName()}
		}).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&corev1.Pod{}, metav1.ObjectNameField, func(raw client.Object) []string {
			pod := raw.(*corev1.Pod)
			return []string{pod.GetObjectMeta().GetName()}
		}).
		Build()

	fakeDynamicClient = dynamicfake.NewSimpleDynamicClient(scheme,
		cloneSet, orphanReplicaSet, replicaSet, deployment, statefulSet, daemonSet,
		orphanJob, job, cronJob,
	)

	podManager, err = podmanager.NewPodManager(
		fakeClient,
		fakeAPIReader,
		fakeDynamicClient,
	)
	Expect(err).NotTo(HaveOccurred())
})
