// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reclaim_test

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ip with reclaim ip case", Label("reclaim"), func() {
	var err error
	var podName, namespace string
	var podIPv4, podIPv6 string

	BeforeEach(func() {
		// create namespace
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err = frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

		// pod name
		podName = "pod" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	It("related IP resource recorded in ippool will be reclaimed after the namespace is deleted",
		Label("smoke", "G00001"), func() {
			// create pod
			_, podIPv4, podIPv6 = createPod(podName, namespace, time.Second*30)

			// ippool allocated ip
			var allocatedIPv4s, allocatedIPv6s []string

			// get ippool status.allocated_ips
			// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s

			if podIPv4 != "" {
				// TODO(bingzhesun) here we assume that we have obtained the allocated ips
				allocatedIPv4s = append(allocatedIPv4s, podIPv4)
				GinkgoWriter.Printf("allocatedIPv4s: %v\n", allocatedIPv4s)

				// check if podIP in ippool
				GinkgoWriter.Println("check if podIPv4 in ippool")
				Expect(allocatedIPv4s).To(ContainElement(podIPv4), "assign ipv4 failed")
			}
			if podIPv6 != "" {
				// TODO(bingzhesun) here we assume that we have obtained the allocated ips
				allocatedIPv6s = append(allocatedIPv6s, podIPv6)
				GinkgoWriter.Printf("allocatedIPv6s: %v\n", allocatedIPv6s)

				// check if podIP in ippool
				GinkgoWriter.Println("check if podIPv6 in ippool")
				Expect(allocatedIPv6s).To(ContainElement(podIPv6), "assign ipv6 failed")
			}

			// delete namespace
			GinkgoWriter.Printf("delete namespace %v\n", namespace)
			err = frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
			// TODO(bingzhesun) Here we will use the function waitNamespaceDeleted() to judge

			// get ippool status.allocated_ips after delete namespace
			// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s
			// here we assume that we have obtained the allocated ips

			//  TODO(bingzhesun) check if podIP in ippool
		})

	Context("delete the same-name pod within a different namespace", func() {
		// declare another namespace namespace1
		var namespace1 string

		BeforeEach(func() {
			// create namespace1
			namespace1 = "ns1-" + tools.RandomName()
			GinkgoWriter.Printf("create namespace1 %v \n", namespace1)
			err = frame.CreateNamespace(namespace1)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace1 %v", namespace1)

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete namespace1 %v \n", namespace1)
				err := frame.DeleteNamespace(namespace1)
				Expect(err).NotTo(HaveOccurred(), "failed to delete namespace1 %v", namespace1)
			})
		})

		It("the IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted",
			Label("G00002"), func() {
				namespaces := []string{namespace, namespace1}
				for _, ns := range namespaces {
					// create pod in namespaces
					_, podIPv4, podIPv6 = createPod(podName, ns, time.Second*30)
				}

				// delete pod in namespace1 until finish
				GinkgoWriter.Printf("delete the pod %v in namespace1 %v\n", podName, namespace1)
				ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
				defer cancel1()
				e2 := frame.DeletePodUntilFinish(podName, namespace1, ctx1)
				Expect(e2).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed delete pod %v/%v\n", namespace1, podName)

				// check if pod in namespace is running normally
				GinkgoWriter.Printf("check if pod %v in namespace %v is running normally\n", podName, namespace)
				pod3, e3 := frame.GetPod(podName, namespace)
				Expect(pod3).NotTo(BeNil())
				Expect(e3).NotTo(HaveOccurred())
				if podIPv4 != "" {
					GinkgoWriter.Println("check pod ipv4")
					podIPv4 := common.GetPodIPv4Address(pod3)
					Expect(podIPv4).NotTo(BeNil())
					GinkgoWriter.Printf("pod ipv4: %v\n", podIPv4)
				}
				if podIPv6 != "" {
					GinkgoWriter.Println("check pod ipv6")
					podIPv6 := common.GetPodIPv6Address(pod3)
					Expect(podIPv6).NotTo(BeNil())
					GinkgoWriter.Printf("pod ipv6: %v\n", podIPv6)
				}

				// TODO(bingzhesun) check the same-name pod , its ip in ippool not be reclaimed
			})
	})

	DescribeTable("the IP can be reclaimed after its deployment, statefulSet, daemonSet, replicaSet, or job is deleted, even when CNI binary is gone on the host", Serial, func(resourceName string) {
		resourceNM := resourceName + tools.RandomName()
		podList := &corev1.PodList{}

		// create resource
		GinkgoWriter.Printf("create %v/%v \n", namespace, resourceNM)

		switch resourceName {
		case "pod":
			pod, _, _ := createPod(resourceNM, namespace, time.Minute)
			podList = &corev1.PodList{
				Items: []corev1.Pod{*pod},
			}
		case "deployment":
			// generate example resource yaml
			GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, resourceNM)
			resourceObj := common.GenerateExampleDeploymentYaml(resourceNM, namespace, int32(1))
			Expect(resourceObj).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, resourceNM)

			// create resource
			GinkgoWriter.Printf("create %v/%v \n", namespace, resourceNM)
			Expect(frame.CreateDeployment(resourceObj)).To(Succeed(), "failed to create %v/%v \n", namespace, resourceNM)

			// get podList
			podList, err = getPodList(namespace, resourceNM)
		case "statefulSet":
			// generate example resource yaml
			GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, resourceNM)
			resourceObj := common.GenerateExampleStatefulSetYaml(resourceNM, namespace, int32(1))
			Expect(resourceObj).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, resourceNM)

			// create resource
			GinkgoWriter.Printf("create %v/%v \n", namespace, resourceNM)
			Expect(frame.CreateStatefulSet(resourceObj)).To(Succeed(), "failed to create %v/%v \n", namespace, resourceNM)

			// get podList
			podList, err = getPodList(namespace, resourceNM)
		default:
			Fail("err: wrong parameters\n")
		}

		// check the resource ip information in ippool is correct
		GinkgoWriter.Printf("check the resource %v/%v ip information in ippool is correct\n", namespace, resourceNM)
		ok, _, _, err := common.CheckPodIpRecordInIppool(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, podList)
		Expect(err).NotTo(HaveOccurred(), "failed to check ip recorded in ippool, err: %v\n", err)
		Expect(ok).To(BeTrue())

		// remove cni bin
		GinkgoWriter.Println("remove cni bin")
		command := "mv /opt/cni/bin/multus /opt/cni/bin/multus.backup"
		common.ExecCommandOnKindNode(frame.Info.KindNodeList, command, time.Second*10)
		GinkgoWriter.Println("remove cni bin successfully")

		// delete resource
		GinkgoWriter.Printf("delete resource %v/%v\n", namespace, resourceNM)

		opt := &client.DeleteOptions{
			GracePeriodSeconds: pointer.Int64Ptr(0),
		}

		switch resourceName {
		case "pod":
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			Expect(frame.DeletePodUntilFinish(resourceNM, namespace, ctx, opt)).To(Succeed())
			GinkgoWriter.Printf("delete pod %v/%v successfully\n", namespace, podName)
		case "deployment":
			Expect(frame.DeleteDeploymentUntilFinish(resourceNM, namespace, time.Minute*2, opt)).To(Succeed())
		default:
			Fail("err: wrong parameters\n")
		}

		// check if the ip in ippool reclaimed normally
		GinkgoWriter.Println("check IP reclaimed")
		Expect(common.WaitIPReclaimedFinish(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, podList, time.Minute)).To(Succeed())

		// restore cni bin
		GinkgoWriter.Println("restore cni bin")
		command = "mv /opt/cni/bin/multus.backup /opt/cni/bin/multus"
		common.ExecCommandOnKindNode(frame.Info.KindNodeList, command, time.Second*10)
		GinkgoWriter.Println("restore cni bin successfully")

		// wait nodes ready
		GinkgoWriter.Println("wait cluster node ready")
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
		defer cancel1()
		ok3, err3 := frame.WaitClusterNodeReady(ctx1)
		Expect(ok3).To(BeTrue())
		Expect(err3).NotTo(HaveOccurred())

	},
		Entry("the IP should be reclaimed after its pod is deleted , even when CNI binary is gone on the host", Serial, Label("G00003"), "pod"),
		Entry("the IP should be reclaimed after its deployment is deleted , even when CNI binary is gone on the host", Serial, Label("G00003"), "deployment"),
	)
})

func createPod(podName, namespace string, duration time.Duration) (pod *corev1.Pod, podIPv4, podIPv6 string) {
	// generate podYaml
	podYaml := common.GenerateExamplePodYaml(podName, namespace)
	Expect(podYaml).NotTo(BeNil())
	GinkgoWriter.Printf("podYaml: %v \n", podYaml)

	// create pod
	pod, podIPv4, podIPv6 = common.CreatePodUntilReady(frame, podYaml, podName, namespace, duration)
	Expect(pod).NotTo(BeNil(), "create pod failed")
	GinkgoWriter.Printf("podIPv4: %v\n", podIPv4)
	GinkgoWriter.Printf("podIPv6: %v\n", podIPv6)
	return
}

func getPodList(namespace, resourceName string) (podList *corev1.PodList, err error) {
	GinkgoWriter.Printf("get %v/%v podList\n", namespace, resourceName)
	podList, err = frame.GetPodListByLabel(map[string]string{"app": resourceName})
	Expect(err).NotTo(HaveOccurred(), "failed to get podList, err: %v\n", err)
	Expect(podList).NotTo(BeNil())
	return
}
