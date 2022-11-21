// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package subnet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	subnetmanager "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
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

	Context("Automatic creation, extension and deletion of ippool by different controllers", func() {
		var (
			subnetAvailableIpNum int = 100
			fixedIPNumber            = "+0"
			v4PoolNameList       []string
			v6PoolNameList       []string
			podList              *corev1.PodList
		)

		BeforeEach(func() {
			if frame.Info.IpV4Enabled {
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(subnetAvailableIpNum)
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(subnetAvailableIpNum)
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
			}
		})

		It("Automatic creation, extension and deletion of ippool by different controllers", Label("I00003"), func() {
			var (
				stsName                 string = "sts-" + tools.RandomName()
				stsReplicasOriginialNum int32  = 1
				stsReplicasScaleupNum   int32  = 2
				rsName                  string = "rs-" + tools.RandomName()
				rsReplicasOriginialNum  int32  = 1
				rsReplicasScaleupNum    int32  = 2
				dsName                  string = "ds-" + tools.RandomName()
				controllerNum           int    = 3
			)

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

			// Create different controller resources
			// Generate example StatefulSet yaml and create StatefulSet
			stsYaml := common.GenerateExampleStatefulSetYaml(stsName, namespace, stsReplicasOriginialNum)
			stsYaml.Spec.Template.Annotations = annotationMap
			Expect(stsYaml).NotTo(BeNil())
			GinkgoWriter.Printf("Tty to create StatefulSet %v/%v \n", namespace, stsName)
			Expect(frame.CreateStatefulSet(stsYaml)).To(Succeed())

			// Generate example daemonSet yaml and create daemonSet
			dsYaml := common.GenerateExampleDaemonSetYaml(dsName, namespace)
			dsYaml.Spec.Template.Annotations = annotationMap
			Expect(dsYaml).NotTo(BeNil())
			GinkgoWriter.Printf("Try to create daemonSet %v/%v \n", namespace, dsName)
			Expect(frame.CreateDaemonSet(dsYaml)).To(Succeed())

			// Generate example replicaSet yaml and create replicaSet
			rsYaml := common.GenerateExampleReplicaSetYaml(rsName, namespace, rsReplicasOriginialNum)
			rsYaml.Spec.Template.Annotations = annotationMap
			Expect(rsYaml).NotTo(BeNil())
			GinkgoWriter.Printf("Try to create replicaSet %v/%v \n", namespace, rsName)
			Expect(frame.CreateReplicaSet(rsYaml)).To(Succeed())

			// All pods are running
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// Check that the ip of the subnet record matches the record in ippool
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, controllerNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, controllerNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}

			// Check that the pod's ip is recorded in the ippool
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// scaling up statefulset/replicaset
			rsObj, err := frame.GetReplicaSet(rsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleReplicasetUntilExpectedReplicas(ctx, frame, rsObj, int(rsReplicasScaleupNum))
			Expect(err).NotTo(HaveOccurred())
			stsObj, err := frame.GetStatefulSet(stsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleStatefulsetUntilExpectedReplicas(ctx, frame, stsObj, int(stsReplicasScaleupNum))
			Expect(err).NotTo(HaveOccurred())
			// Wait statefulSet/replicaset until Ready
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// Check that the ip of the subnet record matches the record in ippool
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}

			// Check that the pod's ip is recorded in the ippool
			podList, err = frame.GetPodList(client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// scaling down statefulset/replicaset
			stsObj, err = frame.GetStatefulSet(stsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleStatefulsetUntilExpectedReplicas(ctx, frame, stsObj, int(stsReplicasOriginialNum))
			Expect(err).NotTo(HaveOccurred())
			rsObj, err = frame.GetReplicaSet(rsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleReplicasetUntilExpectedReplicas(ctx, frame, rsObj, int(rsReplicasOriginialNum))
			Expect(err).NotTo(HaveOccurred())
			// All pods are running
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// Check that the ip of the subnet record matches the record in ippool
			ctx2, cancel2 := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel2()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx2, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx2, frame, v6SubnetName)).NotTo(HaveOccurred())
			}

			// Check that the pod's ip is recorded in the ippool
			podList, err = frame.GetPodList(client.InNamespace(namespace))
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// delete different controller resource
			GinkgoWriter.Printf("delete statefulSet %v/%v\n", namespace, stsName)
			Expect(frame.DeleteStatefulSet(stsName, namespace)).To(Succeed())

			GinkgoWriter.Printf("delete daemonSet %v/%v\n", namespace, dsName)
			Expect(frame.DeleteDaemonSet(dsName, namespace)).To(Succeed())

			GinkgoWriter.Printf("delete replicaset %v/%v\n", namespace, rsName)
			Expect(frame.DeleteReplicaSet(rsName, namespace)).To(Succeed())

			// Delete the resource and wait for the subnet to return to its original state
			ctx3, cancel3 := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel3()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx3, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx3, frame, v4SubnetName, 0)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx3, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx3, frame, v6SubnetName, 0)).NotTo(HaveOccurred())
			}
		})
	})

	Context("Validity of fields in subnet.spec", func() {
		var deployName string = "deploy" + tools.RandomName()
		var fixedIPNumber string = "2"
		var deployOriginiaNum int32 = 1

		BeforeEach(func() {
			v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(10)
			Expect(v4SubnetObject).NotTo(BeNil())
			v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(10)
			Expect(v6SubnetObject).NotTo(BeNil())

			// Delete namespaces and delete subnets
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

		It("valid fields succeed to create subnet. ", Label("I00001"), func() {
			var v4Ipversion, v6Ipversion = new(types.IPVersion), new(types.IPVersion)
			var ipv4Vlan, ipv6Vlan = new(types.Vlan), new(types.Vlan)
			v4Dst := "0.0.0.0/0"
			ipv4Gw := strings.Split(v4SubnetObject.Spec.Subnet, "0/")[0] + "1"
			v6Dst := "::/0"
			ipv6Gw := strings.Split(v6SubnetObject.Spec.Subnet, "/")[0] + "1"

			subnetAnno := subnetmanager.AnnoSubnetItems{}
			if frame.Info.IpV4Enabled {
				*v4Ipversion = int64(4)
				if i, err := strconv.Atoi(common.GenerateRandomNumber(4095)); err != nil {
					*ipv4Vlan = int64(i)
				}
				subnetAnno.IPv4 = []string{v4SubnetName}
				subnetRouteValue := []spiderpool.Route{
					{
						Dst: v4Dst,
						Gw:  ipv4Gw,
					},
				}
				v4SubnetObject.Spec.Vlan = ipv4Vlan
				v4SubnetObject.Spec.Routes = subnetRouteValue
				err := common.CreateSubnet(frame, v4SubnetObject)
				Expect(err).NotTo(HaveOccurred())
				v4Object := common.GetSubnetByName(frame, v4SubnetName)
				Expect(v4Object.Spec.IPVersion).To(Equal(v4Ipversion))
				Expect(v4Object.Spec.Vlan).To(Equal(ipv4Vlan))
				Expect(v4Object.Spec.Routes[0].Dst).To(Equal(v4Dst))
				Expect(v4Object.Spec.Routes[0].Gw).To(Equal(ipv4Gw))

			}
			if frame.Info.IpV6Enabled {
				*v6Ipversion = int64(6)
				if i, err := strconv.Atoi(common.GenerateRandomNumber(4095)); err != nil {
					*ipv6Vlan = int64(i)
				}
				subnetAnno.IPv6 = []string{v6SubnetName}
				subnetRouteValue := []spiderpool.Route{
					{
						Dst: v6Dst,
						Gw:  ipv6Gw,
					},
				}
				v6SubnetObject.Spec.Vlan = ipv6Vlan
				v6SubnetObject.Spec.Routes = subnetRouteValue
				err := common.CreateSubnet(frame, v6SubnetObject)
				Expect(err).NotTo(HaveOccurred())
				v6bject := common.GetSubnetByName(frame, v6SubnetName)
				Expect(v6bject.Spec.IPVersion).To(Equal(v6Ipversion))
				Expect(v6bject.Spec.Vlan).To(Equal(ipv6Vlan))
				Expect(v6bject.Spec.Routes[0].Dst).To(Equal(v6Dst))
				Expect(v6bject.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
			}
			subnetAnnoMarshal, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())
			annotationMap := map[string]string{
				constant.AnnoSpiderSubnetPoolIPNumber: fixedIPNumber,
				constant.AnnoSpiderSubnet:             string(subnetAnnoMarshal),
			}
			deployYaml := common.GenerateExampleDeploymentYaml(deployName, namespace, deployOriginiaNum)
			deployYaml.Spec.Template.Annotations = annotationMap
			Expect(deployYaml).NotTo(BeNil())
			GinkgoWriter.Printf("Tty to create deploy %v/%v \n", namespace, deployName)
			Expect(frame.CreateDeployment(deployYaml)).To(Succeed())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(2))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				v4poolList, err := common.GetIppoolsInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				for _, pool := range v4poolList.Items {
					Expect(pool.Spec.Vlan).To(Equal(ipv4Vlan))
					Expect(pool.Spec.Routes[0].Dst).To(Equal(v4Dst))
					Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
				}
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(2))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6poolList, err := common.GetIppoolsInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				for _, pool := range v6poolList.Items {
					Expect(pool.Spec.Vlan).To(Equal(ipv6Vlan))
					Expect(pool.Spec.Routes[0].Dst).To(Equal(v6Dst))
					Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
				}
			}
		})

		It("Automatically create multiple ippools that can not use the same network segment and use IPs other than excludeIPs. ", Label("I00004"), func() {

			subnetAnno := subnetmanager.AnnoSubnetItems{}
			// ExcludeIPs cannot be used by ippools that are created automatically
			if frame.Info.IpV4Enabled {
				v4SubnetObject.Spec.ExcludeIPs = v4SubnetObject.Spec.IPs
				subnetAnno.IPv4 = []string{v4SubnetName}
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6SubnetObject.Spec.ExcludeIPs = v6SubnetObject.Spec.IPs
				subnetAnno.IPv6 = []string{v6SubnetName}
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
			}
			GinkgoWriter.Printf("succeed to create v4 subnet %v, v6 subnet %v \n", v4SubnetName, v6SubnetName)
			subnetAnnoMarshal, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())
			annotationMap := map[string]string{
				constant.AnnoSpiderSubnetPoolIPNumber: fixedIPNumber,
				constant.AnnoSpiderSubnet:             string(subnetAnnoMarshal),
			}
			deployYaml := common.GenerateExampleDeploymentYaml(deployName, namespace, deployOriginiaNum)
			deployYaml.Spec.Template.Annotations = annotationMap
			Expect(deployYaml).NotTo(BeNil())
			GinkgoWriter.Printf("Tty to create deploy %v/%v \n", namespace, deployName)
			Expect(frame.CreateDeployment(deployYaml)).To(Succeed())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 0)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 0)).NotTo(HaveOccurred())
			}

			var podList *corev1.PodList
			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(deployYaml.Spec.Template.Labels)
				if nil != err || len(podList.Items) == 0 {
					return false
				}
				return true
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel1()
			for _, pod := range podList.Items {
				Expect(frame.WaitExceptEventOccurred(ctx1, common.OwnerPod, pod.Name, namespace, common.CNIFailedToSetUpNetwork)).NotTo(HaveOccurred())
			}

			Expect(frame.DeleteDeployment(deployName, namespace)).NotTo(HaveOccurred())
		})
	})
})
