// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package kruise_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Third party control: OpenKruise", Label("kruise"), func() {
	var namespace, kruiseCloneSetName, kruiseStatefulSetName string
	var v4PoolNameList, v6PoolNameList []string
	var (
		podList           *corev1.PodList
		kruiseReplicasNum int32 = 1
	)

	BeforeEach(func() {
		namespace = "ns" + tools.RandomName()
		kruiseCloneSetName = "cloneset-" + tools.RandomName()
		kruiseStatefulSetName = "sts-" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v. \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Printf("delete namespace %v. \n", namespace)
			Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
		})
	})

	It("SpiderIPPool feature supports third party controllers. ", Label("kruise", "T00001", "T00003"), func() {

		podAnno := types.AnnoPodIPPoolValue{}
		if frame.Info.IpV4Enabled {
			v4PoolNameList = append(v4PoolNameList, common.SpiderPoolIPv4PoolDefault)
			podAnno.IPv4Pools = v4PoolNameList
		}
		if frame.Info.IpV6Enabled {
			v6PoolNameList = append(v6PoolNameList, common.SpiderPoolIPv6PoolDefault)
			podAnno.IPv6Pools = v6PoolNameList
		}
		podAnnoMarshal, err := json.Marshal(podAnno)
		Expect(err).NotTo(HaveOccurred())
		podAnnoStr := string(podAnnoMarshal)

		kruiseCloneSetObject := common.GenerateExampleKruiseCloneSetYaml(kruiseCloneSetName, namespace, kruiseReplicasNum)
		GinkgoWriter.Printf("create Kruise CloneSet %v/%v with annotations %v. \n", namespace, kruiseCloneSetName, podAnnoStr)
		kruiseCloneSetObject.Spec.Template.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podAnnoStr}
		Expect(common.CreateKruiseCloneSet(frame, kruiseCloneSetObject)).NotTo(HaveOccurred())

		kruiseStatefulsetObject := common.GenerateExampleKruiseStatefulSetYaml(kruiseStatefulSetName, namespace, kruiseReplicasNum)
		GinkgoWriter.Printf("create Kruise statefulset %v/%v with annotations %v. \n", namespace, kruiseStatefulSetName, podAnnoStr)
		kruiseStatefulsetObject.Spec.Template.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podAnnoStr}
		Expect(common.CreateKruiseStatefulSet(frame, kruiseStatefulsetObject)).NotTo(HaveOccurred())

		var podNameList []string
		podNameList = append(append(podNameList, kruiseCloneSetName), kruiseStatefulSetName)
		GinkgoWriter.Printf("Wait for the Pod running %v/%v. \n", namespace, podNameList)
		Eventually(func() bool {
			podList, err = frame.GetPodList(client.InNamespace(namespace))
			if nil != err || len(podList.Items) != int(kruiseReplicasNum)*2 {
				return false
			}
			return frame.CheckPodListRunning(podList)
		}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
		GinkgoWriter.Printf("check whether the Pod %v/%v IP is in the ippool %v/%v. \n", namespace, podNameList, v4PoolNameList, v6PoolNameList)
		ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
		Expect(ok).NotTo(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Printf("delete kruise all Pod in namespace. \n", namespace)
		Expect(common.DeleteKruiseCloneSetByName(frame, kruiseCloneSetName, namespace)).NotTo(HaveOccurred())
		Expect(common.DeleteKruiseStatefulSetByName(frame, kruiseStatefulSetName, namespace)).NotTo(HaveOccurred())

		// Check workloadendpoint records are deleted
		// The endpoint of the third-party statefulset can be removed without IP conflict
		ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
		defer cancel()
		for _, pod := range podList.Items {
			err := common.WaitWorkloadDeleteUntilFinish(ctx, frame, pod.Namespace, pod.Name)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("SpiderSubnet feature supports third party controllers.", func() {
		var v4SubnetName, v6SubnetName string
		var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet

		var (
			replicasNum                    int32 = 1
			thirdPartyAppName              string
			v4PoolNameList, v6PoolNameList []string
			IpNum                          int    = 5
			fixedIPNumber                  string = "2"
		)

		BeforeEach(func() {
			if !frame.Info.SpiderSubnetEnabled {
				Skip("SpiderSubnet is disabled, skip this use case")
			}

			thirdPartyAppName = "third-party-" + tools.RandomName()
			if frame.Info.SpiderSubnetEnabled {
				Eventually(func() error {
					if frame.Info.IpV4Enabled {
						v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, IpNum)
						err := common.CreateSubnet(frame, v4SubnetObject)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
							return err
						}
					}
					if frame.Info.IpV6Enabled {
						v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, IpNum)
						err := common.CreateSubnet(frame, v6SubnetObject)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
							return err
						}
					}
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			}

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}

				if frame.Info.SpiderSubnetEnabled {
					GinkgoWriter.Printf("delete v4subnet %v, v6subnet %v. \n", v4SubnetName, v6SubnetName)
					if frame.Info.IpV4Enabled {
						Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
					}
					if frame.Info.IpV6Enabled {
						Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
					}
				}
			})
		})

		It("SpiderSubnet feature supports third party controllers.", Label("kruise", "T00002", "T00003"), func() {
			GinkgoWriter.Println("Generate annotations for subnets Marshal.")
			subnetAnno := types.AnnoSubnetItem{}
			if frame.Info.IpV4Enabled {
				subnetAnno.IPv4 = []string{v4SubnetName}
			}
			if frame.Info.IpV6Enabled {
				subnetAnno.IPv6 = []string{v6SubnetName}
			}
			subnetAnnoMarshal, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Println("Generate annotations for third party control objects.")
			kruiseCloneSetObject := common.GenerateExampleKruiseCloneSetYaml(kruiseCloneSetName, namespace, kruiseReplicasNum)
			kruiseCloneSetObject.Spec.Template.Annotations = map[string]string{
				constant.AnnoSpiderSubnet: string(subnetAnnoMarshal),
				/*
					Notice
						You must specify a fixed IP number for auto-created IPPool if you want to use SpiderSubnet ipam.
						Here's an example ipam.spidernet.io/ippool-ip-number: "5".
				*/
				constant.AnnoSpiderSubnetPoolIPNumber: fixedIPNumber,
			}
			GinkgoWriter.Printf("create CloneSet %v/%v. \n", namespace, kruiseCloneSetName)
			Expect(common.CreateKruiseCloneSet(frame, kruiseCloneSetObject)).NotTo(HaveOccurred())

			kruiseStatefulsetObject := common.GenerateExampleKruiseStatefulSetYaml(kruiseStatefulSetName, namespace, kruiseReplicasNum)
			kruiseStatefulsetObject.Spec.Template.Annotations = map[string]string{
				constant.AnnoSpiderSubnet:             string(subnetAnnoMarshal),
				constant.AnnoSpiderSubnetPoolIPNumber: fixedIPNumber,
			}
			GinkgoWriter.Printf("create statefulSet %v/%v. \n", namespace, kruiseStatefulSetName)
			Expect(common.CreateKruiseStatefulSet(frame, kruiseStatefulsetObject)).NotTo(HaveOccurred())

			GinkgoWriter.Println("Wait for the all Pod running in namespace %v. \n", namespace)
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err || len(podList.Items) != int(kruiseReplicasNum)*2 {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			GinkgoWriter.Println("Check that the IP record for the pool is consistent with the subnet")
			v4PoolNameList = []string{}
			v6PoolNameList = []string{}
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 2)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 2)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("delete kruise all Pod in namespace %v. \n", namespace)
			Expect(common.DeleteKruiseCloneSetByName(frame, kruiseCloneSetName, namespace)).NotTo(HaveOccurred())
			Expect(common.DeleteKruiseStatefulSetByName(frame, kruiseStatefulSetName, namespace)).NotTo(HaveOccurred())

			// Check workloadendpoint records are deleted
			// The endpoint of the third-party statefulset can be removed without IP conflict
			ctx, cancel = context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			for _, pod := range podList.Items {
				err := common.WaitWorkloadDeleteUntilFinish(ctx, frame, pod.Namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())
			}

			// Check if the automatic pool of third party controllers has been removed.
			Eventually(func() bool {
				if frame.Info.IpV4Enabled {
					for _, v := range v4PoolNameList {
						if _, err = common.GetIppoolByName(frame, v); !api_errors.IsNotFound(err) {
							return false
						}
					}
				}
				if frame.Info.IpV6Enabled {
					for _, v := range v6PoolNameList {
						if _, err = common.GetIppoolByName(frame, v); !api_errors.IsNotFound(err) {
							return false
						}
					}
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})

		// T00004: Third-party applications with the same name and type can use the reserved IPPool.
		// I00005: Third-party applications with the same name and different types cannot use the reserved IPPool.
		It("A third-party application of the same name uses reserved IPPool", Label("T00004", "T00005"), func() {
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
				// Set the annotation ipam.spidernet.io/ippool-reclaim: "false"
				// to prevent the fixed pool from being deleted when the application is deleted.
				constant.AnnoSpiderSubnetReclaimIPPool: "false",
				constant.AnnoSpiderSubnet:              string(subnetAnnoMarshal),
				/*
					Notice
						You must specify a fixed IP number for auto-created IPPool if you want to use SpiderSubnet ipam.
						Here's an example ipam.spidernet.io/ippool-ip-number: "5".
				*/
				constant.AnnoSpiderSubnetPoolIPNumber: fixedIPNumber,
			}
			GinkgoWriter.Printf("Set the annotation ipam.spidernet.io/reclaim: false for the application %v/%v, and create. \n", namespace, thirdPartyAppName)
			kruiseCloneSetObject := common.GenerateExampleKruiseCloneSetYaml(thirdPartyAppName, namespace, kruiseReplicasNum)
			kruiseCloneSetObject.Spec.Template.Annotations = annotationMap
			Expect(common.CreateKruiseCloneSet(frame, kruiseCloneSetObject)).NotTo(HaveOccurred())

			// Check that the IP of the subnet record matches the record in IPPool
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}

			// Check that the pod's ip is recorded in the ippool
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			Expect(frame.WaitPodListRunning(kruiseCloneSetObject.Spec.Template.Labels, int(replicasNum), ctx)).NotTo(HaveOccurred())
			podList, err := frame.GetPodListByLabel(kruiseCloneSetObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("Delete third party applications %v/%v. \n", namespace, thirdPartyAppName)
			Expect(common.DeleteKruiseCloneSetByName(frame, thirdPartyAppName, namespace)).NotTo(HaveOccurred())
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err || len(podList.Items) != 0 {
					return false
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

			By("Third-party applications with the same name and type can use the reserved IPPool.")
			// Create third party applications with the same name again
			kruiseCloneSetObject = common.GenerateExampleKruiseCloneSetYaml(thirdPartyAppName, namespace, replicasNum)
			kruiseCloneSetObject.Spec.Template.Annotations = annotationMap
			GinkgoWriter.Printf("Create an application %v/%v with the same name. \n", namespace, thirdPartyAppName)
			Expect(common.CreateKruiseCloneSet(frame, kruiseCloneSetObject)).NotTo(HaveOccurred())

			// Check if the Pod IP of an application with the same name is recorded in IPPool
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			Expect(frame.WaitPodListRunning(kruiseCloneSetObject.Spec.Template.Labels, int(replicasNum), ctx)).NotTo(HaveOccurred())
			rePodList, err := frame.GetPodListByLabel(kruiseCloneSetObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, rePodList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// Delete third party applications again
			Expect(common.DeleteKruiseCloneSetByName(frame, thirdPartyAppName, namespace)).NotTo(HaveOccurred())
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err || len(podList.Items) != 0 {
					return false
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

			By(`Third-party applications with the same name and different type cannot use the reserved IPPool.`)
			// Create third party applications again with the same name but a different controller type
			kruiseStatefulsetObject := common.GenerateExampleKruiseStatefulSetYaml(thirdPartyAppName, namespace, kruiseReplicasNum)
			kruiseStatefulsetObject.Spec.Template.Annotations = annotationMap
			GinkgoWriter.Printf("Create an application %v/%v with the same name but a different type. \n", namespace, thirdPartyAppName)
			Expect(common.CreateKruiseStatefulSet(frame, kruiseStatefulsetObject)).NotTo(HaveOccurred())

			// A third-party application with the same name but a different controller type can be successfully created,
			// But their IP will not be recorded in the reserved IP pool, instead a new fixed IP pool will be created.
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			Expect(frame.WaitPodListRunning(kruiseStatefulsetObject.Spec.Template.Labels, int(replicasNum), ctx)).NotTo(HaveOccurred())
			stsPodList, err := frame.GetPodListByLabel(kruiseStatefulsetObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, stsPodList)
			Expect(ok).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 2)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				newV4PodList, err := common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("A new v4 pool will be created, %v. \n", newV4PodList)
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 2)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				newV6PodList, err := common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("A new v6 pool will be created, %v. \n", newV6PodList)
			}
		})
	})
})
