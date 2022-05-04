// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/test/e2e/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// v1 "k8s.io/client-go/applyconfigurations/core/v1"
	framework "github.com/spidernet-io/spiderpool/test/e2e/framework"
	k8sframework "k8s.io/kubernetes/test/e2e/framework"
	"time"
)

var _ = Describe("test pod", Label("smoke"), func() {
	var err error
	// namespace must be: lower case alphanumeric characters or '-', and must start and end with an alphanumeric character
	const namespacePrefix string = "ippool-pod-"

	frame := framework.NewFramework()

	Context("test default ippool", Label("E00001"), func() {
		var namespace = namespacePrefix + "simple"

		BeforeEach(func() {
			GinkgoWriter.Printf("create namespace %v \n", namespace)
			err = frame.CreateNamespace(namespace)
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err = frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("", func() {
			podName := "simple"

			// create pod
			GinkgoWriter.Printf("try to create pod \n")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      podName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
						},
					},
				},
			}
			err = frame.CreatePod(pod)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeeded to create pod \n")

			// wait for pod ip
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pod, err = frame.WaitPodStarted(podName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())

			// check pod ip
			// pod, err := frame.GetPod(podName, namespace)
			// Expect(err).NotTo(HaveOccurred())
			if len(pod.Status.PodIPs) == 0 {
				Fail("pod failed to get ip")
			}
			GinkgoWriter.Printf("pod %v/%v ip: %+v \n", namespace, podName, pod.Status.PodIPs)
			if frame.C.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue())
				By("succeeded to check pod ipv4 ip ")
			}
			if frame.C.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue())
				By("succeeded to check pod ipv6 ip \n")
			}

			// delete pod
			err = frame.DeletePod(podName, namespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("test k8s framework", func() {
		var podClient *k8sframework.PodClient

		BeforeEach(func() {
			podClient = frame.F.PodClient()
		})

		It("create pod", func() {
			namespace := "default"
			podName := "simple"

			podyaml := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      podName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
						},
					},
				},
			}
			// CreateSync creates a new pod according to the framework specifications,
			// and wait for it to start and be running and ready.
			pod := podClient.CreateSync(podyaml)
			// pod1 := podClient.Create(pod)

			if len(pod.Status.PodIPs) == 0 {
				Fail("pod failed to get ip")
			}
			GinkgoWriter.Printf("pod %v/%v ip: %+v \n", namespace, podName, pod.Status.PodIPs)
			if frame.C.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue())
				By("succeeded to check pod ipv4 ip ")
			}
			if frame.C.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue())
				By("succeeded to check pod ipv6 ip \n")
			}

			// delete pod
			podClient.DeleteSync(podName, metav1.DeleteOptions{}, k8sframework.DefaultPodDeletionTimeout)

		})
	})

})
