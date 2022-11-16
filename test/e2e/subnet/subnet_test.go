// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package subnet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	subnetmanager "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("test subnet", Label("subnet"), func() {
	var v4SubnetName, v6SubnetName, namespace string
	var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet

	BeforeEach(func() {
		if !frame.Info.SpiderSubnetEnabled {
			Skip("Test conditions `enableSpiderSubnet:true` not met")
		}

		// Init namespace and create
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

		// Delete namespaces and delete subnets
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v. \n", namespace)
			Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
		})
	})

	// Create a subnet and limit the number of ip's to `subnetIpNum`
	// Create `deployOriginiaNum` deployments with `deployReplicasOriginialNum` replicas
	// and a fixed number of `fixedIPNumber` IPs from a subnet with `subnetIpNum` IPs
	// Scale up `deployScaleupNum` deploy replicas from `deployReplicasOriginialNum` to `deployReplicasScaleupNum`
	// Scale down other deploy replicas from `deployReplicasOriginialNum` to `deployReplicasScaledownNum`
	// Delete all deploy
	Context("Validate competition for simultaneous creation, expansion, and deletion", func() {
		var deployNameList, v4PoolNameList, v6PoolNameList []string

		var (
			// Available IP in Subnet
			subnetIpNum int = 150
			// Number of deployments created
			deployOriginiaNum int = 30
			// How much of the deployment is for scaling up?
			deployScaleupNum int = 15
			// Initial number of replicas of deploy
			deployReplicasOriginialNum int32 = 2
			// Number of Scaling up replicas of deploy
			deployReplicasScaleupNum int32 = 3
			// Number of Scaling down replicas of deploy
			deployReplicasScaledownNum int32 = 1
			// Number of fixed IP
			fixedIPNumber string = "5"
		)

		BeforeEach(func() {
			frame.EnableLog = false
			if frame.Info.IpV4Enabled {
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(subnetIpNum)
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(subnetIpNum)
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
			}

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete v4 subnet %v, v6 subnet %v \n", v4SubnetName, v6SubnetName)
				if frame.Info.IpV4Enabled && v4SubnetName != "" {
					Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled && v6SubnetName != "" {
					Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
				}
			})
		})

		It("Validate competition for simultaneous creation, expansion, and deletion", Serial, Label("I00006", "I00008"), func() {
			// Create deployments in bulk in a subnet
			subnetAnno := subnetmanager.AnnoSubnetItems{}
			if frame.Info.IpV4Enabled {
				subnetAnno.IPv4 = []string{v4SubnetName}
			}
			if frame.Info.IpV6Enabled {
				subnetAnno.IPv6 = []string{v6SubnetName}
			}
			subnetAnnoMarshal, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())

			annotationMap := map[string]string{
				constant.AnnoSpiderSubnetPoolIPNumber: fixedIPNumber,
				constant.AnnoSpiderSubnet:             string(subnetAnnoMarshal),
			}
			startT1 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel()
			deployNameList = common.BatchCreateDeploymentUntilReady(ctx, frame, deployOriginiaNum, int(deployReplicasOriginialNum), namespace, annotationMap)
			GinkgoWriter.Printf("succeed to create deployments in %v:%v \n", namespace, deployNameList)

			// Check if the ip recorded in subnet status matches the ip recorded in ippool status
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			GinkgoWriter.Println("Create deployments in bulk in the subnet and check that the IPs recorded in the subnet status match the IPs recorded in the ippool status.")
			if frame.Info.IpV4Enabled {
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, deployOriginiaNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, deployOriginiaNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}

			// Check pod ip record in ippool
			podList, err := frame.GetPodList(client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			endT1 := time.Since(startT1)
			GinkgoWriter.Printf("%v deploys were created and %v pools were automatically created at a time cost of %v. \n", deployOriginiaNum, deployOriginiaNum, endT1)

			// Scaling up and down the replicas of the deployment
			wg := sync.WaitGroup{}
			wg.Add(len(deployNameList))
			startT2 := time.Now()
			for i := 0; i < len(deployNameList); i++ {
				j := i
				name := deployNameList[i]
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					deploy, err := frame.GetDeployment(name, namespace)
					Expect(err).NotTo(HaveOccurred())
					// How much of the deployment is for scaling up?
					if j < deployScaleupNum {
						_, _, err = common.ScaleDeployUntilExpectedReplicas(frame, deploy, int(deployReplicasScaleupNum), ctx)
						Expect(err).NotTo(HaveOccurred())
					} else {
						_, _, err = common.ScaleDeployUntilExpectedReplicas(frame, deploy, int(deployReplicasScaledownNum), ctx)
						Expect(err).NotTo(HaveOccurred())
					}
				}()
			}
			wg.Wait()

			// All pods are running
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
			// Check pod ip record in ippool
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// After scaling up and down, check that the IPs recorded in the subnet status match the IPs recorded in the ippool status.
			GinkgoWriter.Println("After scaling up and down, check that the IPs recorded in the subnet status match the IPs recorded in the ippool status.")
			ctx, cancel = context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}
			endT2 := time.Since(startT2)
			GinkgoWriter.Printf("Scaling up and down deployments at a time cost of %v. \n", endT2)

			// Delete all deployments and wait for the automatic deletion of ippool resources
			startT3 := time.Now()
			wg = sync.WaitGroup{}
			wg.Add(len(deployNameList))
			for _, d := range deployNameList {
				name := d
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(frame.DeleteDeployment(name, namespace)).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()

			// After deleting the resource, check the AllocatedIP of the subnet and expect it to return to its original state
			ctx, cancel = context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			GinkgoWriter.Println("Check the AllocatedIP of the subnet back to the original state")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 0)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 0)).NotTo(HaveOccurred())
			}
			endT3 := time.Since(startT3)
			GinkgoWriter.Printf("All resources are recycled at a time cost of %v. \n", endT3)

			// attaching Data to Reports
			AddReportEntry("Subnet Performance Results",
				fmt.Sprintf(`{ "createTime": %d , "scaleupAndScaledownTime":%d, "deleteTime": %d }`,
					int(endT1.Seconds()), int(endT2.Seconds()), int(endT3.Seconds())))
		})
	})
})
