// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package editcrd_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ippool CR", Label("editcrd"), func() {

	Context("create ippool with an ip that already exists in another ippool", func() {
		var v4PoolName, v4PoolName1, v6PoolName, v6PoolName1, podAnnoStr string
		var v4PoolNameList, v6PoolNameList []string
		var v4PoolObj, v4PoolObj1, v6PoolObj, v6PoolObj1 *spiderpoolv1.IPPool
		var deployName, namespace string
		BeforeEach(func() {
			deployName = "dep" + tools.RandomName()
			namespace = "ns" + tools.RandomName()
			GinkgoWriter.Printf("create namespace %v \n", namespace)
			err := frame.CreateNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())

				// create ipv4 pool
				createIPPool(v4PoolObj)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv6 pool
				createIPPool(v6PoolObj)
			}

			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					deleteIPPoolUntilFinish(v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					deleteIPPoolUntilFinish(v6PoolName)
				}
				GinkgoWriter.Printf("delete namespace %v \n", namespace)
				err = frame.DeleteNamespace(namespace)
				Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
			})
		})

		It(" fail to append an ip that already exists in another ippool to the ippool",
			Label("D00001"), Pending, func() {
				// create ippool with the same ip with the former
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("create v4 ippool with same ips %v\n", v4PoolObj.Spec.IPs)
					v4PoolName1, v4PoolObj1 = common.GenerateExampleIpv4poolObject(1)
					v4PoolObj1.Spec.Subnet = v4PoolObj.Spec.Subnet
					v4PoolObj1.Spec.IPs = v4PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v4PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("failed to create v4 ippool %v with the same ip with another ippool %v\n", v4PoolName1, v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("create v6 ippool with same ips %v\n", v6PoolObj.Spec.IPs)
					v6PoolName1, v6PoolObj1 = common.GenerateExampleIpv6poolObject(1)
					v6PoolObj1.Spec.Subnet = v6PoolObj.Spec.Subnet
					v6PoolObj1.Spec.IPs = v6PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v6PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("failed to create v6 ippool %v with the same ip with another ippool %v\n", v6PoolName1, v6PoolName)
				}
			})

		It(" fail to delete the ippool with assigned ip and succeced delete the ippool with no-assigned ip",
			Label("D00004"), Pending, func() {

				// pod annotations
				podAnno := types.AnnoPodIPPoolValue{}
				if frame.Info.IpV4Enabled {
					v4PoolNameList = append(v4PoolNameList, v4PoolName)
					podAnno.IPv4Pools = v4PoolNameList
				}
				if frame.Info.IpV6Enabled {
					v6PoolNameList = append(v6PoolNameList, v6PoolName)
					podAnno.IPv6Pools = v6PoolNameList
				}
				podAnnobyte, err := json.Marshal(podAnno)
				Expect(err).NotTo(HaveOccurred())
				podAnnoStr = string(podAnnobyte)

				// generate deployment yaml
				deployYaml := common.GenerateExampleDeploymentYaml(deployName, namespace, int32(1))
				deployYaml.Spec.Template.Annotations = map[string]string{
					constant.AnnoPodIPPool: podAnnoStr,
				}
				GinkgoWriter.Println("generate deploy Yaml: %v", deployYaml)

				// create deployment until ready
				deploy, err := frame.CreateDeploymentUntilReady(deployYaml, time.Minute)
				Expect(err).NotTo(HaveOccurred())

				// get pod list
				podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("pod list Num:%v \n", len(podList.Items))

				// check pod ip record in ippool
				GinkgoWriter.Println("check podIP record in ippool")
				ok1, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok1).To(BeTrue())

				// delete ippool
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(Succeed())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(Succeed())
				}

				// check pod ip record in ippool again
				GinkgoWriter.Println("check podIP record in ippool again")
				ok2, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok2).To(BeTrue())

				// delete this deployment
				err1 := frame.DeleteDeploymentUntilFinish(deployName, namespace, time.Second*30)
				Expect(err1).NotTo(HaveOccurred())

				// after del deployment to check pod ip record is not in ippool
				GinkgoWriter.Println("after del deployment to check podIP record is not in ippool")
				ok, relok, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
				Expect(ok).To(BeFalse())
				Expect(relok).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
			})
	})

})

func deleteIPPoolUntilFinish(poolName string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	GinkgoWriter.Printf("delete ippool %v\n", poolName)
	Expect(common.DeleteIPPoolUntilFinish(frame, poolName, ctx)).To(Succeed())
}

func createIPPool(IPPoolObj *spiderpoolv1.IPPool) {
	GinkgoWriter.Printf("create ippool %v\n", IPPoolObj.Name)
	Expect(common.CreateIppool(frame, IPPoolObj)).To(Succeed())
	GinkgoWriter.Printf("succeeded to create ippool %v \n", IPPoolObj.Name)
}
