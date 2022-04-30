package setup_test_test

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/spidernet-io/spiderpool/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	. "github.com/onsi/gomega"
)

var _ = Describe("Setup", Label(framework.LabelSmoke), func() {
	var err error

	Context("test ipv4", Label(framework.LabelIpv4), func() {
		var podName, podNameSpace string

		BeforeEach(func() {
			podName = "ipv4test"
			podNameSpace = "default"
		})

		It("Create ipv4 Pod", func() {
			if frame.C.IpV4Enabled == false {
				Skip("ipv4 is not enabled by cluster, ignore case")
			}

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

			pod, err = frame.GetPod(podName, podNameSpace)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeeded to get pod : %+v \n", pod)

			err = frame.DeletePod(podName, podNameSpace)
			Expect(err).NotTo(HaveOccurred())

		})

	})

	Context("test ipv6", Label(framework.LabelIpv6), func() {

	})

})
