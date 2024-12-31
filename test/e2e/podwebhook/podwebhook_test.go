// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podwebhook_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Podwebhook", func() {
	var namespace string

	BeforeEach(func() {
		// create namespace
		namespace = "ns-" + common.GenerateString(10, true)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v")
		})
	})

	Context("Test inject pod network resources", func() {
		It("Test inject pod network resources", Label("H00001"), func() {
			// Define multus cni NetworkAttachmentDefinition and create
			createNad := func(name string) *v2beta1.SpiderMultusConfig {
				return &v2beta1.SpiderMultusConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Annotations: map[string]string{
							constant.AnnoPodResourceInject: "macvlan-rdma",
						},
					},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.MacvlanCNI),
						MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
							Master:           []string{common.NIC1},
							RdmaResourceName: "spidernet.io/rdma_resource" + "_" + name,
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test-ipv4-pool"},
							},
						},
					},
				}
			}

			By("Create spiderMultusConfig: nad1 for testing")
			Expect(frame.CreateSpiderMultusInstance(createNad("nad1"))).NotTo(HaveOccurred())
			By("Create spiderMultusConfig: nad2 for testing")
			Expect(frame.CreateSpiderMultusInstance(createNad("nad2"))).NotTo(HaveOccurred())

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: namespace,
					Annotations: map[string]string{
						constant.AnnoPodResourceInject: "macvlan-rdma",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "while true; do echo 'HTTP/1.1 200 OK Hello, World!' | nc -l -p 80; done"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "samplepod",
									ContainerPort: 80,
								},
							},
						},
					},
				},
			}

			By("Create Pod for testing network resources inject")
			err := frame.CreatePod(pod)
			Expect(err).NotTo(HaveOccurred())

			By("Check pod network annotations and resources")
			p, err := frame.GetPod(pod.Name, pod.Namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to get pod: %v", err)

			GinkgoWriter.Printf("Pod annotations: %v\n", p.Annotations)
			GinkgoWriter.Printf("Pod resources: %v\n", p.Spec.Containers[0].Resources.Limits)
			Expect(p.Annotations[constant.MultusNetworkAttachmentAnnot]).To(Equal(fmt.Sprintf("%s/%s,%s/%s", namespace, "nad1", namespace, "nad2")))
			Expect(p.Spec.Containers[0].Resources.Requests).To(HaveKey(corev1.ResourceName("spidernet.io/rdma_resource_nad1")))
			Expect(p.Spec.Containers[0].Resources.Requests).To(HaveKey(corev1.ResourceName("spidernet.io/rdma_resource_nad2")))
		})
	})
})
