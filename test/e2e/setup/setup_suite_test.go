// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Spider
package setup_test

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"testing"

	"github.com/spidernet-io/spiderpool/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ipFamily string
var kubeconfig string
var multusInstall bool
var spiderInstall bool

func init() {
	testing.Init()
	flag.StringVar(&ipFamily, "ipFamily", "ipv4", "ip family, default is ipv4")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "the path to kubeconfig")
	flag.BoolVar(&multusInstall, "multus-install", false, "Indicates if multus is installed")
	flag.BoolVar(&spiderInstall, "spider-install", false, "Indicates if spider is installed")
	flag.Parse()
}

func ips() []string {
	var ips_list []string
	var i int
	for i = 100; i <= 200; i++ {
		ips_list = append(ips_list, fmt.Sprintf("172.18.0.%d", i))
	}
	return ips_list
}

func check_ip_in_ips(target string, str_array []string) bool {
	sort.Strings(str_array)
	index := sort.SearchStrings(str_array, target)
	if index < len(str_array) && str_array[index] == target {
		return true
	}
	return false
}

func TestSetup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Setup Suite")
}

var _ = Describe("Kind Cluster Setup", func() {

	var options []framework.Option
	options = append(options, framework.WithIpFamily(ipFamily))
	options = append(options, framework.WithMultus(multusInstall))
	options = append(options, framework.WithSpider(multusInstall))

	f := framework.NewFramework("Setup", kubeconfig, options...)

	Describe("Cluster Setup", func() {

		switch f.CLusterConfig.IpFamily {
		case "ipv4":
			It("List Spider Pods", Label("smoke", "list"), func() {
				_, err := f.KubeClientSet.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create Pod", Label("create"), func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "e2e-test"},
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

				By("create a pod")
				_, err := f.KubeClientSet.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				By("wait pod runnig")
				_, err = f.WaitPodReady("e2e-test", "default")
				Expect(err).NotTo(HaveOccurred())

				By("check pod ip is correct")
				podinfo, err := f.KubeClientSet.CoreV1().Pods("default").Get(context.Background(), "e2e-test", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				fmt.Printf("macvlan network pod ip is: %s\n", podinfo.Status.PodIP)
				resinfo := check_ip_in_ips(podinfo.Status.PodIP, ips())
				fmt.Printf("Is the ip in the ips，turn out: %t\n", resinfo)
				Expect(resinfo).To(Equal(true))

				By("Delete test pod")
				err = f.KubeClientSet.CoreV1().Pods("default").Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		case "ipv6":
			// TODO: implement it
			klog.Info("This is ipv6")
		default:
			// TODO: implement it
			klog.Info("This is Dual")
		}

	})

})
