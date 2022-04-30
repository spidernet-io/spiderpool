package ippool_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/test/e2e/framework"
	"github.com/spidernet-io/spiderpool/test/e2e/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var _ = Describe("test pod", Label(framework.LabelSmoke), func() {
	var err error
	const namespacePrefix string = "ippool_pod_"

	framework.NamespacedTest(namespacePrefix+"simple", frame, func(namespace string) {
		It("test default ippool", func() {
			podName := "simple"
			namespace = "default"
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

})
