// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reservedip_test

import (
	"context"
	"encoding/json"
	"time"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test reservedIP", Label("reservedIP"), func() {
	var nsName, manualDeployName, autoDeployName, v4PoolName, v6PoolName, v4ReservedIpName, v6ReservedIpName string
	var v4PoolNameList, v6PoolNameList []string
	var iPv4PoolObj, iPv6PoolObj *spiderpoolv2beta1.SpiderIPPool
	var v4ReservedIpObj, v6ReservedIpObj *spiderpoolv2beta1.SpiderReservedIP
	var err error
	var v4SubnetName, v6SubnetName string
	var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet

	BeforeEach(func() {
		if frame.Info.SpiderSubnetEnabled {
			// Subnet Adaptation
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 5)
					err := common.CreateSubnet(frame, v4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 5)
					err := common.CreateSubnet(frame, v6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
		}

		// Init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("Try to create namespace %v \n", nsName)
		err = frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace %v", nsName)

		// Init test Deployment/Pod name
		manualDeployName = "sr-manual-pod" + tools.RandomName()
		autoDeployName = "sr-auto-pod" + tools.RandomName()

		Eventually(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(1)
				if frame.Info.SpiderSubnetEnabled {
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, 1)
				} else {
					err = common.CreateIppool(frame, iPv4PoolObj)
				}
				if err != nil {
					GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
					return err
				}
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
			}

			if frame.Info.IpV6Enabled {
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(1)
				if frame.Info.SpiderSubnetEnabled {
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, 1)
				} else {
					err = common.CreateIppool(frame, iPv6PoolObj)
				}
				if err != nil {
					GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
					return err
				}
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
			}
			return nil
		}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

		// Clean test env
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v", nsName)
			GinkgoWriter.Printf("Successful deletion of namespace %v \n", nsName)

			if frame.Info.IpV4Enabled {
				Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
				}
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
				}
			}
		})
	})

	It("S00001: an IP who is set in ReservedIP CRD, should not be assigned to a pod; S00003: Failed to set same IP in excludeIPs when an IP is assigned to a pod",
		Label("S00001", "smoke", "S00003", "I00011"), func() {
			const deployOriginialNum = int(1)
			const fixedIPNumber string = "+1"

			if frame.Info.IpV4Enabled {
				if frame.Info.SpiderSubnetEnabled {
					v4ReservedIpName, v4ReservedIpObj = common.GenerateExampleV4ReservedIpObject(v4SubnetObject.Spec.IPs)
				} else {
					v4ReservedIpName, v4ReservedIpObj = common.GenerateExampleV4ReservedIpObject(iPv4PoolObj.Spec.IPs)
				}
				err = common.CreateReservedIP(frame, v4ReservedIpObj)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Successfully created v4 reservedIP: %v \n", v4ReservedIpName)
			}

			if frame.Info.IpV6Enabled {
				if frame.Info.SpiderSubnetEnabled {
					v6ReservedIpName, v6ReservedIpObj = common.GenerateExampleV6ReservedIpObject(v6SubnetObject.Spec.IPs)
				} else {
					v6ReservedIpName, v6ReservedIpObj = common.GenerateExampleV6ReservedIpObject(iPv6PoolObj.Spec.IPs)
				}
				err = common.CreateReservedIP(frame, v6ReservedIpObj)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Successfully created v6 reservedIP : %v \n", v6ReservedIpName)
			}

			// Generate IPPool annotation string
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList)

			// Generate Deployment yaml and annotation
			manualDeployObject := common.GenerateExampleDeploymentYaml(manualDeployName, nsName, int32(deployOriginialNum))
			manualDeployObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			GinkgoWriter.Printf("Try to create Deployment: %v/%v with annotation %v = %v \n", nsName, manualDeployName, constant.AnnoPodIPPool, podIppoolAnnoStr)

			// Try to create a Deployment and wait for replicas to meet expectations（because the Pod is not running）
			ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel1()
			podlist, err := common.CreateDeployUntilExpectedReplicas(frame, manualDeployObject, ctx1)
			Expect(err).NotTo(HaveOccurred())
			Expect(int32(len(podlist.Items))).Should(Equal(*manualDeployObject.Spec.Replicas))

			// I00011: The subnet automatically creates an ippool and allocates IP, and should consider reservedIP
			if frame.Info.SpiderSubnetEnabled {
				subnetAnno := types.AnnoSubnetItem{}
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
				autoDeployObject := common.GenerateExampleDeploymentYaml(autoDeployName, nsName, int32(deployOriginialNum))
				autoDeployObject.Spec.Template.Annotations = annotationMap
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				GinkgoWriter.Printf("subnet feature enable and create deploy %v/%v. \n", nsName, autoDeployName)
				autoPod, err := common.CreateDeployUntilExpectedReplicas(frame, autoDeployObject, ctx)
				Expect(err).NotTo(HaveOccurred())

				ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel1()
				if frame.Info.IpV4Enabled {
					// Subnet function change, no more automatic empty pool creation action.
					Expect(common.WaitIppoolNumberInSubnet(ctx1, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
					v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
					Expect(err).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.WaitIppoolNumberInSubnet(ctx1, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
					v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
					Expect(err).NotTo(HaveOccurred())
				}

				// subnet feature enable
				podlist.Items = append(podlist.Items, autoPod.Items...)
			}

			// Get the Pod creation failure Event
			ctx, cancel := context.WithTimeout(context.Background(), 3*60*time.Second)
			defer cancel()
			for _, pod := range podlist.Items {
				Expect(frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
				GinkgoWriter.Printf("IP assignment for Deployment/Pod: %v/%v fails when an IP is set in the ReservedIP CRD. \n", pod.Namespace, pod.Name)
			}

			// Try deleting the reservedIP and check if the reserved IP can be assigned to a pod.
			ctx2, cancel2 := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel2()
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteResverdIPUntilFinish(ctx2, frame, v4ReservedIpName)).To(Succeed())
				GinkgoWriter.Printf("Delete v4 reservedIP: %v successfully \n", v4ReservedIpName)
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteResverdIPUntilFinish(ctx2, frame, v6ReservedIpName)).To(Succeed())
				GinkgoWriter.Printf("Delete v6 reservedIP: %v successfully \n", v6ReservedIpName)
			}

			GinkgoWriter.Println("After removing the reservedIP, wait for the Pod to restart until running.")
			err = frame.RestartDeploymentPodUntilReady(manualDeployName, nsName, common.PodReStartTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Succeeded to assign IPv4、IPv6 ip for Deployment/Pod and Deployment/Pod IP recorded in IPPool
			common.CheckPodIpReadyByLabel(frame, manualDeployObject.Spec.Selector.MatchLabels, v4PoolNameList, v6PoolNameList)
			GinkgoWriter.Printf("Pod %v/%v IP recorded in IPPool %v %v \n", nsName, manualDeployName, v4PoolNameList, v6PoolNameList)

			if frame.Info.SpiderSubnetEnabled {
				GinkgoWriter.Println("The subnet feature is enabled, check the ippool status after removing the reserved IP.")
				err = frame.RestartDeploymentPodUntilReady(autoDeployName, nsName, common.PodReStartTimeout)
				Expect(err).NotTo(HaveOccurred())

				ctx3, cancel3 := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel3()
				if frame.Info.IpV4Enabled {
					Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx3, frame, v4SubnetName)).NotTo(HaveOccurred())
					v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
					Expect(err).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx3, frame, v6SubnetName)).NotTo(HaveOccurred())
					v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
					Expect(err).NotTo(HaveOccurred())
				}

				// Check that the pod's ip is recorded in the ippool
				restartPodList, err := frame.GetPodList(client.InNamespace(nsName))
				Expect(err).NotTo(HaveOccurred())
				ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, restartPodList)
				Expect(ok).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
			}

			// S00003: Failed to set same IP in excludeIPs when an IP is assigned to a Pod
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Printf("Update the v4 IPPool and set the IP %v used by the Pod in the excludeIPs. \n", iPv4PoolObj.Spec.IPs)
				desiredV4PoolObj, err := common.GetIppoolByName(frame, v4PoolName)
				Expect(err).NotTo(HaveOccurred())
				desiredV4PoolObj.Spec.ExcludeIPs = desiredV4PoolObj.Spec.IPs
				err = common.PatchIppool(frame, desiredV4PoolObj, iPv4PoolObj)
				Expect(err).To(HaveOccurred())
				GinkgoWriter.Printf("Failed to update v4 IPPool %v when setting the IP %v used by the Pod in the IPPool's excludeIPs \n", v4PoolName, iPv4PoolObj.Spec.IPs)
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Printf("Update the v6 IPPool and set the IP %v used by the Pod in the excludeIPs. \n", iPv6PoolObj.Spec.IPs)
				desiredV6PoolObj, err := common.GetIppoolByName(frame, v6PoolName)
				Expect(err).NotTo(HaveOccurred())
				desiredV6PoolObj.Spec.ExcludeIPs = desiredV6PoolObj.Spec.IPs
				err = common.PatchIppool(frame, desiredV6PoolObj, iPv6PoolObj)
				Expect(err).To(HaveOccurred())
				GinkgoWriter.Printf("Failed to update v6 IPPool %v when setting the IP %v used by the Pod in the IPPool's excludeIPs \n", v6PoolName, iPv6PoolObj.Spec.IPs)
			}

			// Delete the deployment
			GinkgoWriter.Printf("Delete manual deployment: %v/%v \n", nsName, manualDeployName)
			Expect(frame.DeleteDeployment(manualDeployName, nsName)).To(Succeed())
			if frame.Info.SpiderSubnetEnabled {
				GinkgoWriter.Printf("Delete auto deployment: %v/%v \n", nsName, autoDeployName)
				Expect(frame.DeleteDeployment(autoDeployName, nsName)).To(Succeed())
			}
		})
})
