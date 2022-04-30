package setup_test_test

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/spidernet-io/spiderpool/test/e2e/framework"
	"github.com/spidernet-io/spiderpool/test/e2e/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	// "sigs.k8s.io/controller-runtime/pkg/client"
	"context"
	"github.com/asaskevich/govalidator"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup", Label(framework.LabelSmoke), func() {
	var err error

	Context("test pod ip", Label(framework.LabelIpv4), func() {
		var podName, podNameSpace string

		BeforeEach(func() {
			podName = "ipv4test"
			podNameSpace = "default"
		})

		It("Create Pod", func() {

			// create pod
			GinkgoWriter.Printf("try to create pod \n")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: podNameSpace,
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
			pod, err = frame.WaitPodStarted(podName, podNameSpace, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())

			// check pod ip
			// pod, err := frame.GetPod(podName, podNameSpace)
			// Expect(err).NotTo(HaveOccurred())
			if len(pod.Status.PodIPs) == 0 {
				Fail("pod failed to get ip")
			}
			if frame.C.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue())
				GinkgoWriter.Printf("succeeded to check pod ipv4 ip \n")
			}
			if frame.C.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue())
				GinkgoWriter.Printf("succeeded to check pod ipv6 ip \n")
			}

			// delete pod
			err = frame.DeletePod(podName, podNameSpace)
			Expect(err).NotTo(HaveOccurred())

		})

	})

	Context("test ipv6", Label(framework.LabelIpv6), func() {

	})

})
