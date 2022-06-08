// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reliability_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test reliability", Label("reliability"), Serial, func() {
	//var namespace string
	var depName, nsName string
	var err error

	BeforeEach(func() {
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		depName = "pod" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err := frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	//It("reboot node to check ip assign", Label("smoke", "R00006"), func() {
	DescribeTable("check ip assign after reboot node",
		func(replicas int32) {
			GinkgoWriter.Printf("create Deployment")
			// create Deployment
			GinkgoWriter.Printf("try to create Deployment %v/%v \n", depName, nsName)
			dep := common.GenerateExampleDeploymentYaml(depName, nsName, replicas)
			err = frame.CreateDeployment(dep)
			Expect(err).NotTo(HaveOccurred(), "failed to create Deployment")

			ctx3, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			dep2, err1 := frame.WaitDeploymentReady(depName, nsName, ctx3)
			Expect(err1).NotTo(HaveOccurred(), "time out to wait all Replicas ready")
			Expect(dep2).NotTo(BeNil())

			// check pods created by Deployment
			podlist, err2 := frame.GetDeploymentPodList(dep2)
			Expect(err2).NotTo(HaveOccurred(), "failed to list pod")
			Expect(int32(len(podlist.Items))).Should(Equal(dep2.Status.ReadyReplicas))

			// check all pods to created by controller
			err3 := frame.CheckPodListIpReady(podlist)
			Expect(err3).NotTo(HaveOccurred(), "failed to check ipv4 or ipv6")
			nodeMap := make(map[string]bool)

			// send docker restart command to reboot node
			for _, item := range podlist.Items {
				GinkgoWriter.Printf("item.Status.NodeName", item.Spec.NodeName)
				nodeMap[item.Spec.NodeName] = true
			}
			GinkgoWriter.Printf("nodeMap %v\n", nodeMap)
			for node := range nodeMap {
				GinkgoWriter.Printf("NODE====%v\n", node)
				bin := "/bin/bash"
				args := []string{
					"-c", fmt.Sprintf("docker restart %s", node),
				}
				cmd := exec.Command(bin, args...)
				GinkgoWriter.Printf("cmd %v\n", cmd)

				output, err := cmd.Output()
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("output %v\n", string(output))

				fmt.Println(cmd.ProcessState.ExitCode())
				if nil != err {
					fmt.Println("bad: ", cmd.ProcessState.ExitCode())
				}
				if len(output) != 0 {
					fmt.Println(string(output))
				} else {
					fmt.Println("null value")
				}
			}

			//check node up ok
			bin2 := "/bin/bash"
			args2 := []string{
				"-c", "docker ps |awk '{print$7'}",
			}
			cmd2 := exec.Command(bin2, args2...)
			GinkgoWriter.Printf("cmd2 %v\n", cmd2)
			output, err := cmd2.Output()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("node up output %v\n", string(output))
			if strings.Contains(string(output), "Up") {
				GinkgoWriter.Printf("node up ok\n")
			} else {
				Fail("node have not up ")
			}

			//check pod ip assign ok
			ReadyCheckIp(depName, nsName)

			//delete Deployment
			errdel := frame.DeleteDeployment(depName, nsName)
			Expect(errdel).NotTo(HaveOccurred(), "failed to delete Deployment %v: %v/%v \n", depName, nsName)

		},
		//	Entry("pod Replicas is 10", Label("R00006"), int32(10)),
		Entry("pod Replicas is 50", Label("R00006"), int32(50)),
	)
})

func ReadyCheckIp(depName1 string, nsName1 string) {
	// waiting for Deployment replicas to complete
	ctx3, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dep2, err1 := frame.WaitDeploymentReady(depName1, nsName1, ctx3)
	Expect(err1).NotTo(HaveOccurred(), "time out to wait all Replicas ready")
	Expect(dep2).NotTo(BeNil())

	// check pods created by Deployment，its assign ipv4 and ipv6 addresses success
	podlist, err2 := frame.GetDeploymentPodList(dep2)
	Expect(err2).NotTo(HaveOccurred(), "failed to list pod")
	Expect(int32(len(podlist.Items))).Should(Equal(dep2.Status.ReadyReplicas))

	//At present, the IPv6 addresses of the pod in the whereabout+macvlan environment are the same ,resulting in the failure of the E2E test to verify whether the IP addresses are duplicated
	//Check whether the IP is duplicated
	// ipMap := make(map[string]bool)
	// for _, item := range podlist.Items {
	// 	GinkgoWriter.Printf("item.Status.PodIPs", item.Status.PodIPs)
	// 	for _, ip := range item.Status.PodIPs {

	// 		_, ok := ipMap[ip.IP]
	// 		if ok {
	// 			Fail("ip repeat")
	// 		}
	// 		ipMap[ip.IP] = true
	// 	}

	// }

	// check all pods to created by controller，it`s assign ipv4 and ipv6 addresses success
	err3 := frame.CheckPodListIpReady(podlist)
	Expect(err3).NotTo(HaveOccurred(), "failed to check ipv4 or ipv6")

}
