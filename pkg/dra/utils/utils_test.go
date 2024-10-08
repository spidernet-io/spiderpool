// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package utils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	drautils "github.com/spidernet-io/spiderpool/pkg/dra/utils"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	corev1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"
)

var _ = Describe("test pod webhook", Label("DRA"), func() {

	var podT *corev1.Pod
	var rct *resourcev1alpha2.ResourceClaimTemplate
	var scp *spiderpoolv2beta1.SpiderClaimParameter
	var name string
	BeforeEach(func() {
		rct = &resourcev1alpha2.ResourceClaimTemplate{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ResourceClaimTemplate",
				APIVersion: "resource.k8s.io/v1alpha2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: resourcev1alpha2.ResourceClaimTemplateSpec{
				Spec: resourcev1alpha2.ResourceClaimSpec{
					ParametersRef: &resourcev1alpha2.ResourceClaimParametersReference{
						Kind:     constant.KindSpiderClaimParameter,
						APIGroup: constant.SpiderpoolAPIGroup,
						Name:     name,
					},
				},
			},
		}

		scp = &spiderpoolv2beta1.SpiderClaimParameter{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.KindSpiderClaimParameter,
				APIVersion: spiderpoolv2beta1.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: spiderpoolv2beta1.ClaimParameterSpec{
				DefaultNic: &spiderpoolv2beta1.MultusConfig{
					Namespace:  "default",
					MultusName: "test1",
				},
				SecondaryNics: []spiderpoolv2beta1.MultusConfig{
					{
						Namespace:  "default",
						MultusName: "test2",
					},
				},
			},
		}

		podT = &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				ResourceClaims: []corev1.PodResourceClaim{
					{
						Name: name,
						Source: corev1.ClaimSource{
							ResourceClaimTemplateName: ptr.To("test"),
						},
					},
				},
			},
			Status: corev1.PodStatus{},
		}

		resourceList := corev1.ResourceRequirements{
			// Requests: corev1.ResourceList{
			// 	corev1.ResourceCPU:    resource.MustParse("100m"),
			// 	corev1.ResourceMemory: resource.MustParse("100Mi"),
			// 	corev1.ResourceName("spidernet.io/rdma_resources"): resource.MustParse("1"),
			// },
			Limits: corev1.ResourceList{},
			Claims: []corev1.ResourceClaim{
				{
					Name: name,
				},
			},
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

		err := fakeClient.Create(context.TODO(), rct)
		Expect(err).NotTo(HaveOccurred())

		err = fakeClient.Create(context.TODO(), scp)
		Expect(err).NotTo(HaveOccurred())

		res, err := drautils.GetStaticNicsFromSpiderClaimParameter(context.TODO(), fakeAPIReader, podT)
		Expect(err).NotTo(HaveOccurred())

		expected := []spiderpoolv2beta1.MultusConfig{
			{
				Namespace:  "default",
				MultusName: "test1",
			},
			{
				Namespace:  "default",
				MultusName: "test2",
			},
		}
		Expect(res).To(Equal(expected))
	})

	It("test GetRdmaResourceMapFromStaticNics", func() {
		multusConfigs := []spiderpoolv2beta1.MultusConfig{
			{
				Namespace:  "default",
				MultusName: "test1",
			},
			{
				Namespace:  "default",
				MultusName: "test2",
			},
		}

		smcT := spiderpoolv2beta1.SpiderMultusConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.KindSpiderMultusConfig,
				APIVersion: spiderpoolv2beta1.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To("maclvlan"),
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					EnableRdma:       true,
					RdmaResourceName: "spidernet.io/rdma_resource_a",
					Master:           []string{"eth0"},
				},
			},
		}

		for _, t := range []string{"test1", "test2"} {
			smcT.ObjectMeta.Name = t
			smcT.Spec.MacvlanConfig.RdmaResourceName = "spidernet.io/rdma_resource" + t

			err := fakeClient.Create(context.TODO(), &smcT)
			Expect(err).NotTo(HaveOccurred())
		}

		resMap, err := drautils.GetRdmaResourceMapFromStaticNics(context.TODO(), fakeAPIReader, multusConfigs)
		Expect(err).NotTo(HaveOccurred())

		expected := map[string]bool{
			"spidernet.io/rdma_resource_test1": false,
			"spidernet.io/rdma_resource_test2": false,
		}

		Expect(resMap).To(Equal(expected))
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
				// corev1.ResourceName("spidernet.io/rdma_resources"): resource.MustParse("1"),
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

		resourceMap := map[string]bool{
			"spidernet.io/rdma_resources_a": false,
			"spidernet.io/rdma_resources_b": false,
		}
		drautils.InjectRdmaResourceToPod(resourceMap, podT)

		for k := range resourceMap {
			value, ok := podT.Spec.Containers[0].Resources.Requests[corev1.ResourceName(k)]
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("1"))
		}
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
		drautils.InjectRdmaResourceToPod(resourceMap, podT)

		Expect(len(podT.Spec.Containers[0].Resources.Requests)).To(Equal(5))
		Expect(podT.Spec.Containers[0].Resources.Requests[corev1.ResourceName("spidernet.io/rdma_resources_c")]).To(Equal("1"))

		Expect(podT.Spec.Containers[0].Resources).To(Equal(resourceList))
	})
})
