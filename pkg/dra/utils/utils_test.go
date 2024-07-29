// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package utils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	drautils "github.com/spidernet-io/spiderpool/pkg/dra/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("test pod webhook", func() {

	var podT *corev1.Pod
	BeforeEach(func() {
		podT = &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		}

		resourceList := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourceName("spidernet.io/rdma_resources"): resource.MustParse("1"),
			},
			Limits: corev1.ResourceList{},
		}

		podT.Spec.Containers = []corev1.Container{
			{
				Name:      "container",
				Image:     "busybox",
				Resources: resourceList,
			},
		}
	})

	It("Test GetStaticNicsFromSpiderClaimParameter", func() {
		fakeClient.Create(context.TODO())
		drautils.GetStaticNicsFromSpiderClaimParameter(context.TODO(), fakeAPIReader)
	})

	It("empty resources", func() {
		podT := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		}

		resourceList := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourceName("spidernet.io/rdma_resources"): resource.MustParse("1"),
			},
			Limits: corev1.ResourceList{},
		}

		podT.Spec.Containers = []corev1.Container{
			{
				Name:      "container",
				Image:     "busybox",
				Resources: resourceList,
			},
		}

		fakeClient.Create(context.TODO())
		podmanager.InjectRdmaResourceToPod(resourceMap, podT)
		Expect(podT.Spec.Containers[0].Resources).To(Equal(resourceList))
	})

	It("resourceMap not to empty, and pod has both limits and requests resources", func() {

		podT := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		}

		resourceList := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourceName("spidernet.io/rdma_resources_a"): resource.MustParse("1"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceName("spidernet.io/rdma_resources_a"): resource.MustParse("1"),
			},
		}

		podT.Spec.Containers = []corev1.Container{
			{
				Name:      "container",
				Image:     "busybox",
				Resources: resourceList,
			},
		}

		resourceMap := make(map[string]bool)
		resourceMap["spidernet.io/rdma_resources_b"] = false
		resourceMap["spidernet.io/rdma_resources_c"] = false
		podmanager.InjectRdmaResourceToPod(resourceMap, podT)

		resourceList.Requests[corev1.ResourceName("spidernet.io/rdma_resources_b")] = resource.MustParse("1")
		resourceList.Requests[corev1.ResourceName("spidernet.io/rdma_resources_c")] = resource.MustParse("1")

		Expect(podT.Spec.Containers[0].Resources).To(Equal(resourceList))
	})

	It("resourceMap not to empty, and pod has already cliamed the resources, should be ignore", func() {
		podT := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		}

		resourceList := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
				corev1.ResourceName("spidernet.io/rdma_resources_a"): resource.MustParse("2"),
				corev1.ResourceName("spidernet.io/rdma_resources_b"): resource.MustParse("2"),
				corev1.ResourceName("spidernet.io/rdma_resources_c"): resource.MustParse("2"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceName("spidernet.io/rdma_resources_a"): resource.MustParse("2"),
			},
		}

		podT.Spec.Containers = []corev1.Container{
			{
				Name:      "container",
				Image:     "busybox",
				Resources: resourceList,
			},
		}

		resourceMap := make(map[string]bool)
		resourceMap["spidernet.io/rdma_resources_b"] = false
		resourceMap["spidernet.io/rdma_resources_c"] = false
		resourceMap["spidernet.io/rdma_resources_d"] = false
		podmanager.InjectRdmaResourceToPod(resourceMap, podT)

		resourceList.Requests[corev1.ResourceName("spidernet.io/rdma_resources_d")] = resource.MustParse("1")

		Expect(podT.Spec.Containers[0].Resources).To(Equal(resourceList))
	})
})
