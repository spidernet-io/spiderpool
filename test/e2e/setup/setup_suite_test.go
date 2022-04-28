// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Spider
package setup_test

import (
	"context"
	"flag"
	"github.com/spidernet-io/spiderpool/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"testing"

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
				_, err := f.KubeClientSet.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
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
