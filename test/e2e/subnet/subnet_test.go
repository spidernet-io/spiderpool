// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package subnet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			startT1 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), common.BatchCreateTimeout)
			defer cancel()
			deployNameList = common.BatchCreateDeploymentUntilReady(ctx, frame, deployOriginiaNum, int(deployReplicasOriginialNum), namespace, annotationMap)
			GinkgoWriter.Printf("succeed to create deployments in %v:%v \n", namespace, deployNameList)

			// Check if the ip recorded in subnet status matches the ip recorded in ippool status
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			GinkgoWriter.Println("Create deployments in bulk in the subnet and check that the IPs recorded in the subnet status match the IPs recorded in the ippool status.")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, deployOriginiaNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, deployOriginiaNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
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
						_, _, err = common.ScaleDeployUntilExpectedReplicas(ctx, frame, deploy, int(deployReplicasScaleupNum), true)
						Expect(err).NotTo(HaveOccurred())
					} else {
						_, _, err = common.ScaleDeployUntilExpectedReplicas(ctx, frame, deploy, int(deployReplicasScaledownNum), true)
						Expect(err).NotTo(HaveOccurred())
					}
				}()
			}
			wg.Wait()

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

			// Check pod ip record in ippool
			podList, err = frame.GetPodList(client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())
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
			nodeList             *corev1.NodeList
			err                  error
		)

		BeforeEach(func() {
			nodeList, err = frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeList).NotTo(BeNil())
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
				deployNum               int    = 10
				deployReplicasNum       int32  = 1
				deployPatchReplicasNum  int32  = 2
			)

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

			// Generate example deplyment yaml and creating deploys in batches
			lock := lock.Mutex{}
			wg := sync.WaitGroup{}
			wg.Add(deployNum)
			var deployNameList []string
			GinkgoWriter.Println("creating deploys in batches")
			for i := 1; i <= deployNum; i++ {
				var deployObject *appsv1.Deployment
				j := strconv.Itoa(i)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					deployName := "deploy-" + j + "-" + tools.RandomName()
					deployObject = common.GenerateExampleDeploymentYaml(deployName, namespace, deployReplicasNum)
					deployObject.Spec.Template.Annotations = annotationMap
					Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())
					// Update deploy to trigger add callback
					desiredDeploy := common.GenerateExampleDeploymentYaml(deployName, namespace, deployPatchReplicasNum)
					desiredDeploy.Spec.Template.Annotations = annotationMap
					mergePatch := client.MergeFrom(deployObject)
					Expect(frame.PatchResource(desiredDeploy, mergePatch)).NotTo(HaveOccurred())

					lock.Lock()
					deployNameList = append(deployNameList, deployName)
					lock.Unlock()
				}()
			}
			wg.Wait()
			Expect(len(deployNameList)).To(Equal(deployNum))

			// Check that the ip of the subnet record matches the record in ippool
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, (controllerNum + deployNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, (controllerNum + deployNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
			}

			// Check that the pod's ip is recorded in the ippool
			allPod := len(nodeList.Items) + int(stsReplicasOriginialNum) + int(rsReplicasOriginialNum) + (deployNum * int(deployPatchReplicasNum))
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err || len(podList.Items) != allPod {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// scaling up statefulset/replicaset
			rsObj, err := frame.GetReplicaSet(rsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleReplicasetUntilExpectedReplicas(ctx, frame, rsObj, int(rsReplicasScaleupNum), true)
			Expect(err).NotTo(HaveOccurred())
			stsObj, err := frame.GetStatefulSet(stsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleStatefulsetUntilExpectedReplicas(ctx, frame, stsObj, int(stsReplicasScaleupNum), true)
			Expect(err).NotTo(HaveOccurred())

			// Check that the pod's ip is recorded in the ippool
			podList, err = frame.GetPodList(client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

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
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// scaling down statefulset/replicaset
			stsObj, err = frame.GetStatefulSet(stsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleStatefulsetUntilExpectedReplicas(ctx, frame, stsObj, int(stsReplicasOriginialNum), true)
			Expect(err).NotTo(HaveOccurred())
			rsObj, err = frame.GetReplicaSet(rsName, namespace)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = common.ScaleReplicasetUntilExpectedReplicas(ctx, frame, rsObj, int(rsReplicasOriginialNum), true)
			Expect(err).NotTo(HaveOccurred())

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
			Expect(err).NotTo(HaveOccurred())
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

			// delete all deployment
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
		var fixedIPNumber string = "+0"
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

		It("valid fields succeed to create subnet. ", Label("I00001", "I00002", "I00005"), func() {
			var v4Ipversion, v6Ipversion = new(types.IPVersion), new(types.IPVersion)
			var ipv4Vlan, ipv6Vlan = new(types.Vlan), new(types.Vlan)
			var v4Object, v6Object *spiderpool.SpiderSubnet
			var v4RouteValue, v6RouteValue []spiderpool.Route

			v4Dst := "0.0.0.0/0"
			ipv4Gw := strings.Split(v4SubnetObject.Spec.Subnet, "0/")[0] + "1"
			v6Dst := "::/0"
			ipv6Gw := strings.Split(v6SubnetObject.Spec.Subnet, "/")[0] + "1"
			subnetAnno := types.AnnoSubnetItem{}
			if frame.Info.IpV4Enabled {
				*v4Ipversion = int64(4)
				if i, err := strconv.Atoi(common.GenerateRandomNumber(4095)); err != nil {
					*ipv4Vlan = int64(i)
				}
				subnetAnno.IPv4 = []string{v4SubnetName}
				v4RouteValue = []spiderpool.Route{
					{
						Dst: v4Dst,
						Gw:  ipv4Gw,
					},
				}
				v4SubnetObject.Spec.Vlan = ipv4Vlan
				v4SubnetObject.Spec.Routes = v4RouteValue
				v4SubnetObject.Spec.Gateway = &ipv4Gw
				GinkgoWriter.Printf("Specify routes, gateways, etc. and then create subnets %v \n", v4SubnetName)
				err := common.CreateSubnet(frame, v4SubnetObject)
				Expect(err).NotTo(HaveOccurred())
				v4Object = common.GetSubnetByName(frame, v4SubnetName)
				Expect(v4Object.Spec.IPVersion).To(Equal(v4Ipversion))
				Expect(v4Object.Spec.Vlan).To(Equal(ipv4Vlan))
				Expect(v4Object.Spec.Routes[0].Dst).To(Equal(v4Dst))
				Expect(v4Object.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
				Expect(v4Object.Spec.Gateway).To(Equal(&ipv4Gw))
			}

			if frame.Info.IpV6Enabled {
				*v6Ipversion = int64(6)
				if i, err := strconv.Atoi(common.GenerateRandomNumber(4095)); err != nil {
					*ipv6Vlan = int64(i)
				}
				subnetAnno.IPv6 = []string{v6SubnetName}
				v6RouteValue = []spiderpool.Route{
					{
						Dst: v6Dst,
						Gw:  ipv6Gw,
					},
				}
				v6SubnetObject.Spec.Vlan = ipv6Vlan
				v6SubnetObject.Spec.Routes = v6RouteValue
				v6SubnetObject.Spec.Gateway = &ipv6Gw
				GinkgoWriter.Printf("Specify routes, gateways, etc. and then create subnets %v \n", v6SubnetName)
				err := common.CreateSubnet(frame, v6SubnetObject)
				Expect(err).NotTo(HaveOccurred())
				v6Object = common.GetSubnetByName(frame, v6SubnetName)
				Expect(v6Object.Spec.IPVersion).To(Equal(v6Ipversion))
				Expect(v6Object.Spec.Vlan).To(Equal(ipv6Vlan))
				Expect(v6Object.Spec.Routes[0].Dst).To(Equal(v6Dst))
				Expect(v6Object.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
				Expect(v6Object.Spec.Gateway).To(Equal(&ipv6Gw))
			}

			// Checking gateways and routes for automatically created pool inheritance subnets
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

			if frame.Info.IpV4Enabled {
				GinkgoWriter.Println("=====Ipv4=====")
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(deployOriginiaNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				GinkgoWriter.Println("Check that the gateways and routes recorded in the automatically created ippool are correct")
				v4poolList, err := common.GetIppoolsInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				for _, pool := range v4poolList.Items {
					Expect(pool.Spec.Vlan).To(Equal(ipv4Vlan))
					Expect(pool.Spec.Routes[0].Dst).To(Equal(v4Dst))
					Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
					Expect(pool.Spec.Gateway).To(Equal(&ipv4Gw))
				}

				// Create an ippool manually
				v4PoolName, iPv4PoolObj := common.GenerateExampleIpv4poolObject(1)
				iPv4PoolObj.Spec.Gateway = &ipv4Gw
				iPv4PoolObj.Spec.Routes = v4RouteValue
				Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, 1)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Update gateways and routes. ")
				newV4Dst := v4SubnetObject.Spec.Subnet
				newIpv4Gw := strings.Split(v4SubnetObject.Spec.Subnet, "0/")[0] + "255"
				subnetRouteValue := []spiderpool.Route{
					{
						Dst: newV4Dst,
						Gw:  newIpv4Gw,
					},
				}
				v4SubnetObject = common.GetSubnetByName(frame, v4SubnetName)
				*v4Object = *v4SubnetObject
				v4Object.Spec.Routes = subnetRouteValue
				v4Object.Spec.Gateway = &newIpv4Gw

				GinkgoWriter.Println("Add an ip belonging to the CIDR to the subnet.")
				oldIPs, err := ip.ParseIPRanges(*v4Object.Spec.IPVersion, v4Object.Spec.IPs)
				Expect(err).NotTo(HaveOccurred())
				nextIp := ip.NextIP(oldIPs[len(oldIPs)-1])
				newIps := append(oldIPs, nextIp)
				newIpRange, err := ip.ConvertIPsToIPRanges(*v4Object.Spec.IPVersion, newIps)
				Expect(err).NotTo(HaveOccurred())
				v4Object.Spec.IPs = newIpRange
				Expect(common.PatchSpiderSubnet(frame, v4Object, v4SubnetObject)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Add an ip that is not part of the CIDR to the subnet.")
				v4Object.Spec.IPs = []string{"0.0.0.0"}
				Expect(common.PatchSpiderSubnet(frame, v4Object, v4SubnetObject)).To(HaveOccurred())

				GinkgoWriter.Println("Check if the changes were successful.")
				v4Object = common.GetSubnetByName(frame, v4SubnetName)
				Expect(v4Object.Spec.Routes[0].Dst).To(Equal(newV4Dst))
				Expect(v4Object.Spec.Routes[0].Gw).To(Equal(newIpv4Gw))
				Expect(v4Object.Spec.Gateway).To(Equal(&newIpv4Gw))

				// Delete an ip that is being used in the subnet
				nextIpRange, err := ip.ConvertIPsToIPRanges(*v4Object.Spec.IPVersion, []net.IP{nextIp})
				Expect(err).NotTo(HaveOccurred())
				v4Object.Spec.IPs = nextIpRange
				GinkgoWriter.Println("Failed to remove an ip in use. ")
				Expect(common.PatchSpiderSubnet(frame, v4Object, v4SubnetObject)).To(HaveOccurred())

				// Delete unused ip in the subnet
				oldIpRange, err := ip.ConvertIPsToIPRanges(*v4Object.Spec.IPVersion, oldIPs)
				Expect(err).NotTo(HaveOccurred())
				v4Object = common.GetSubnetByName(frame, v4SubnetName)
				v4Object.Spec.IPs = oldIpRange
				GinkgoWriter.Println("Successfully deleted an unused IP.")
				Expect(common.PatchSpiderSubnet(frame, v4Object, v4SubnetObject)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Subnet routing gateway updated successfully, manual pool creation does not change.")
				iPv4PoolObj = common.GetIppoolByName(frame, v4PoolName)
				Expect(iPv4PoolObj.Spec.Routes[0].Dst).To(Equal(v4Dst))
				Expect(iPv4PoolObj.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
				Expect(iPv4PoolObj.Spec.Gateway).To(Equal(&ipv4Gw))

				v4poolList, err = common.GetIppoolsInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Println("Older pools are not affected.")
				for _, pool := range v4poolList.Items {
					if pool.Name != v4PoolName {
						Expect(pool.Spec.Vlan).To(Equal(ipv4Vlan))
						Expect(pool.Spec.Routes[0].Dst).To(Equal(v4Dst))
						Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
						Expect(pool.Spec.Gateway).To(Equal(&ipv4Gw))
					}
				}
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Println("=====Ipv6=====")
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(deployOriginiaNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6poolList, err := common.GetIppoolsInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				for _, pool := range v6poolList.Items {
					Expect(pool.Spec.Vlan).To(Equal(ipv6Vlan))
					Expect(pool.Spec.Routes[0].Dst).To(Equal(v6Dst))
					Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
					Expect(pool.Spec.Gateway).To(Equal(&ipv6Gw))
				}

				// Create an ippool manually
				v6PoolName, iPv6PoolObj := common.GenerateExampleIpv6poolObject(1)
				iPv6PoolObj.Spec.Gateway = &ipv6Gw
				iPv6PoolObj.Spec.Routes = v6RouteValue
				Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, 1)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Update gateways and routes. ")
				newV6Dst := v6SubnetObject.Spec.Subnet
				newIpv6Gw := strings.Split(v6SubnetObject.Spec.Subnet, "/")[0] + "fe"
				subnetRouteValue := []spiderpool.Route{
					{
						Dst: newV6Dst,
						Gw:  newIpv6Gw,
					},
				}
				v6SubnetObject = common.GetSubnetByName(frame, v6SubnetName)
				*v6Object = *v6SubnetObject
				v6Object.Spec.Routes = subnetRouteValue
				v6Object.Spec.Gateway = &newIpv6Gw

				GinkgoWriter.Println("Add an v6 ip belonging to the CIDR to the v6 subnet.")
				oldIPs, err := ip.ParseIPRanges(*v6Object.Spec.IPVersion, v6Object.Spec.IPs)
				Expect(err).NotTo(HaveOccurred())
				nextIp := ip.NextIP(oldIPs[len(oldIPs)-1])
				newIps := append(oldIPs, nextIp)
				newIpRange, err := ip.ConvertIPsToIPRanges(*v6Object.Spec.IPVersion, newIps)
				Expect(err).NotTo(HaveOccurred())
				v6Object.Spec.IPs = newIpRange
				Expect(common.PatchSpiderSubnet(frame, v6Object, v6SubnetObject)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Add an v6 ip that is not part of the CIDR to the v6 subnet.")
				v6Object.Spec.IPs = []string{"::"}
				Expect(common.PatchSpiderSubnet(frame, v6Object, v6SubnetObject)).To(HaveOccurred())

				GinkgoWriter.Println("Check if the changes were successful.")
				v6Object = common.GetSubnetByName(frame, v6SubnetName)
				Expect(v6Object.Spec.Routes[0].Dst).To(Equal(newV6Dst))
				Expect(v6Object.Spec.Routes[0].Gw).To(Equal(newIpv6Gw))
				Expect(v6Object.Spec.Gateway).To(Equal(&newIpv6Gw))

				// Delete an v6 ip that is being used in the v6 subnet
				nextIpRange, err := ip.ConvertIPsToIPRanges(*v6Object.Spec.IPVersion, []net.IP{nextIp})
				Expect(err).NotTo(HaveOccurred())
				v6Object.Spec.IPs = nextIpRange
				GinkgoWriter.Println("Failed to remove an ip in use. ")
				Expect(common.PatchSpiderSubnet(frame, v6Object, v6SubnetObject)).To(HaveOccurred())

				// Delete unused v6 ip in the v6 subnet
				oldIpRange, err := ip.ConvertIPsToIPRanges(*v6Object.Spec.IPVersion, oldIPs)
				Expect(err).NotTo(HaveOccurred())
				v6Object = common.GetSubnetByName(frame, v6SubnetName)
				v6Object.Spec.IPs = oldIpRange
				GinkgoWriter.Println("Successfully deleted an unused IP.")
				Expect(common.PatchSpiderSubnet(frame, v6Object, v6SubnetObject)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Subnet routing gateway updated successfully, manual pool creation does not change.")
				iPv6PoolObj = common.GetIppoolByName(frame, v6PoolName)
				Expect(iPv6PoolObj.Spec.Routes[0].Dst).To(Equal(v6Dst))
				Expect(iPv6PoolObj.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
				Expect(iPv6PoolObj.Spec.Gateway).To(Equal(&ipv6Gw))

				v6poolList, err = common.GetIppoolsInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Println("Older pools are not affected.")
				for _, pool := range v6poolList.Items {
					if pool.Name != v6PoolName {
						Expect(pool.Spec.Vlan).To(Equal(ipv6Vlan))
						Expect(pool.Spec.Routes[0].Dst).To(Equal(v6Dst))
						Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
						Expect(pool.Spec.Gateway).To(Equal(&ipv6Gw))
					}
				}
			}

			Expect(frame.DeleteDeployment(deployName, namespace)).NotTo(HaveOccurred())
		})

		It("Automatically create multiple ippools that can not use the same network segment and use IPs other than excludeIPs. ", Label("I00004", "S00004"), func() {
			subnetAnno := types.AnnoSubnetItem{}
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

			// Checking manually created ippools will automatically circumvent excludeIPs
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj := common.GenerateExampleIpv4poolObject(5)
				v4PoolObj.Spec.Subnet = v4SubnetObject.Spec.Subnet
				v4PoolObj.Spec.IPs = v4SubnetObject.Spec.IPs
				Expect(common.CreateIppool(frame, v4PoolObj)).NotTo(Succeed())
				GinkgoWriter.Printf("Failed to create an IPv4 IPPool %v. \n", v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj := common.GenerateExampleIpv6poolObject(5)
				v6PoolObj.Spec.Subnet = v6SubnetObject.Spec.Subnet
				v6PoolObj.Spec.IPs = v6SubnetObject.Spec.IPs
				Expect(common.CreateIppool(frame, v6PoolObj)).NotTo(Succeed())
				GinkgoWriter.Printf("Failed to create an IPv6 IPPool %v. \n", v6PoolName)
			}

			// Checking automatically created ippools will automatically circumvent excludeIPs
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
				// Check the number of AllocatedIPCount in the subnet
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
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

	Context("Manual create the subnet and ippool.", func() {
		var v4PoolObject, v6PoolObject *spiderpool.SpiderIPPool
		var v4SubnetNameList, v6SubnetNameList []string
		var v4PoolNameList, v6PoolNameList []string

		// failed to run case, Refer to https://github.com/spidernet-io/spiderpool/issues/868
		// the same spec is used to create the subnet and only one should succeed
		// the same spec is used to create the ippool and only one should succeed
		PIt("the same spec is used to create the subnet/ippool and only one should succeed.", Label("I00009", "I00010"), func() {
			var (
				batchCreateSubnetNumber int = 2
				batchCreateIPPoolNumber int = 2
			)

			// Generate example v4 or v6 subnetObject/poolObject
			if frame.Info.IpV4Enabled {
				_, v4SubnetObject = common.GenerateExampleV4SubnetObject(200)
				_, v4PoolObject = common.GenerateExampleIpv4poolObject(200)
				v4PoolObject.Spec.Subnet = v4SubnetObject.Spec.Subnet
				v4PoolObject.Spec.IPs = v4SubnetObject.Spec.IPs
			}
			if frame.Info.IpV6Enabled {
				_, v6SubnetObject = common.GenerateExampleV6SubnetObject(200)
				_, v6PoolObject = common.GenerateExampleIpv6poolObject(200)
				v6PoolObject.Spec.Subnet = v6SubnetObject.Spec.Subnet
				v6PoolObject.Spec.IPs = v6SubnetObject.Spec.IPs
			}
			GinkgoWriter.Printf("v4SubnetObject %v; v6SubnetObject %v \n", v4SubnetObject, v6SubnetObject)
			GinkgoWriter.Printf("v4PoolObject %v; v6PoolObject %v \n", v4PoolObject, v6PoolObject)

			lock := lock.Mutex{}
			wg := sync.WaitGroup{}
			wg.Add(batchCreateSubnetNumber)
			for i := 1; i <= batchCreateSubnetNumber; i++ {
				// Create `batchCreateSubnetNumber` subnets simultaneously using the same subnet.spec.
				// The same spec is used to create the subnet and only one should succeed
				j := i
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					if frame.Info.IpV4Enabled {
						v4SubnetName := "v4-ss-" + strconv.Itoa(j) + "-" + tools.RandomName()
						v4SubnetObj := &spiderpool.SpiderSubnet{
							ObjectMeta: metav1.ObjectMeta{
								Name: v4SubnetName,
							},
							Spec: v4SubnetObject.Spec,
						}
						err := common.CreateSubnet(frame, v4SubnetObj)
						if err == nil {
							subnet := common.GetSubnetByName(frame, v4SubnetName)
							if subnet.Spec.Subnet == v4SubnetObj.Spec.Subnet {
								GinkgoWriter.Printf("succeed to create subnet %v, spec.subnet is %v \n", v4SubnetName, v4SubnetObj.Spec.Subnet)
								lock.Lock()
								v4SubnetNameList = append(v4SubnetNameList, v4SubnetName)
								lock.Unlock()
							}
						}
					}
					if frame.Info.IpV6Enabled {
						v6SubnetName := "v6-ss-" + strconv.Itoa(j) + "-" + tools.RandomName()
						v6SubnetObj := &spiderpool.SpiderSubnet{
							ObjectMeta: metav1.ObjectMeta{
								Name: v6SubnetName,
							},
							Spec: v6SubnetObject.Spec,
						}
						err := common.CreateSubnet(frame, v6SubnetObj)
						if err == nil {
							subnet := common.GetSubnetByName(frame, v4SubnetName)
							if subnet.Spec.Subnet == v6SubnetObj.Spec.Subnet {
								GinkgoWriter.Printf("succeed to create subnet %v, spec.subnet is %v \n", v6SubnetName, v6SubnetObj.Spec.Subnet)
								lock.Lock()
								v6SubnetNameList = append(v6SubnetNameList, v6SubnetName)
								lock.Unlock()
							}
						}
					}
				}()
			}
			GinkgoWriter.Printf("v4SubnetNameList %v;v6SubnetNameList %v \n", v4SubnetNameList, v6SubnetNameList)
			wg.Wait()
			// TODO(tao.yang),failed to run the case,refer to https://github.com/spidernet-io/spiderpool/issues/868
			if frame.Info.IpV4Enabled {
				Expect(len(v4SubnetNameList)).To(Equal(1))
			}
			if frame.Info.IpV6Enabled {
				Expect(len(v6SubnetNameList)).To(Equal(1))
			}

			wg = sync.WaitGroup{}
			wg.Add(batchCreateIPPoolNumber)
			for i := 1; i <= batchCreateIPPoolNumber; i++ {
				// Create `batchCreateIPPoolNumber` ippools simultaneously using the same ippool.spec.
				// The same spec is used to create the ippool and only one should succeed.
				j := i
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					if frame.Info.IpV4Enabled {
						v4PoolName := "v4-pool-" + strconv.Itoa(j) + "-" + tools.RandomName()
						v4PoolObj := &spiderpool.SpiderIPPool{
							ObjectMeta: metav1.ObjectMeta{
								Name: v4PoolName,
							},
							Spec: v4PoolObject.Spec,
						}
						err := common.CreateIppool(frame, v4PoolObj)
						if err == nil {
							pool := common.GetIppoolByName(frame, v4PoolName)
							if pool.Spec.Subnet == v4PoolObj.Spec.Subnet {
								GinkgoWriter.Printf("succeed to create ippool %v, spec.ips is %v \n", v4PoolName, v4PoolObj.Spec.IPs)
								lock.Lock()
								v4PoolNameList = append(v4PoolNameList, v4PoolName)
								lock.Unlock()
							}
						}
					}
					if frame.Info.IpV6Enabled {
						v6PoolName := "v6-pool-" + strconv.Itoa(j) + "-" + tools.RandomName()
						v6PoolObj := &spiderpool.SpiderIPPool{
							ObjectMeta: metav1.ObjectMeta{
								Name: v6PoolName,
							},
							Spec: v6PoolObject.Spec,
						}
						err := common.CreateIppool(frame, v6PoolObj)
						if err == nil {
							pool := common.GetIppoolByName(frame, v6PoolName)
							if pool.Spec.Subnet == v6PoolObj.Spec.Subnet {
								GinkgoWriter.Printf("succeed to create ippool %v, spec.ips is %v \n", v6PoolName, v6PoolObj.Spec.IPs)
								lock.Lock()
								v6PoolNameList = append(v6PoolNameList, v6PoolName)
								lock.Unlock()
							}
						}
					}
				}()
			}
			GinkgoWriter.Printf("v4PoolNameList %v;v6PoolNameList %v \n", v4PoolNameList, v6PoolNameList)
			wg.Wait()
			// TODO(tao.yang), failed to run the case,refer to https://github.com/spidernet-io/spiderpool/issues/868
			if frame.Info.IpV4Enabled {
				Expect(len(v4PoolNameList)).To(Equal(1))
			}
			if frame.Info.IpV6Enabled {
				Expect(len(v6PoolNameList)).To(Equal(1))
			}

			// delete all ippool
			GinkgoWriter.Printf("delete v4 pool %v, v6 pool %v \n", v4PoolNameList, v6PoolNameList)
			if frame.Info.IpV4Enabled {
				for _, v := range v4PoolNameList {
					Expect(common.DeleteIPPoolByName(frame, v)).NotTo(HaveOccurred())
				}
			}
			if frame.Info.IpV6Enabled {
				for _, v := range v6PoolNameList {
					Expect(common.DeleteIPPoolByName(frame, v)).NotTo(HaveOccurred())
				}
			}
			// delete all subnet
			GinkgoWriter.Printf("delete v4 subnet %v, v6 subnet %v \n", v4SubnetNameList, v6SubnetNameList)
			if frame.Info.IpV4Enabled {
				for _, v := range v4SubnetNameList {
					Expect(common.DeleteSubnetByName(frame, v)).NotTo(HaveOccurred())
				}
			}
			if frame.Info.IpV6Enabled {
				for _, v := range v6SubnetNameList {
					Expect(common.DeleteSubnetByName(frame, v)).NotTo(HaveOccurred())
				}
			}
		})

		// Manual batch create of subnets and ippools and record time
		// batch delete ippools under subnet and record time
		// batch delete subnets and record time
		PIt("the different spec is used to create the subnet/ippool and should all be successful.", Label("I00007", "D00007"), func() {
			var (
				batchCreateSubnetNumber int = 20
				batchCreateIPPoolNumber int = 10
				subnetIpNumber          int = 200
				ippoolIpNumber          int = 2
				err                     error
			)
			// batch create subnet and ippool and record time
			if frame.Info.IpV4Enabled {
				startT1 := time.Now()
				v4SubnetNameList, err = common.BatchCreateSubnet(frame, constant.IPv4, batchCreateSubnetNumber, subnetIpNumber)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed to batch create %v v4 subnet \n", len(v4SubnetNameList))
				endT1 := time.Since(startT1)
				GinkgoWriter.Printf("Time cost to create %v v4 subnet is %v \n", batchCreateSubnetNumber, endT1)
				startT1 = time.Now()
				v4SubnetObject = common.GetSubnetByName(frame, v4SubnetNameList[1])
				v4PoolNameList, err = common.BatchCreateIPPoolsInSpiderSubnet(frame, constant.IPv4, v4SubnetObject.Spec.Subnet, v4SubnetObject.Spec.IPs, batchCreateIPPoolNumber, ippoolIpNumber)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(v4PoolNameList)).To(Equal(batchCreateIPPoolNumber))
				ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
				defer cancel()
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetNameList[1])).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed to batch create %v v4 pool \n", len(v4PoolNameList))
				endT1 = time.Since(startT1)
				GinkgoWriter.Printf("Time cost to create %v v4 ippool is %v \n", batchCreateIPPoolNumber, endT1)
			}
			if frame.Info.IpV6Enabled {
				startT1 := time.Now()
				v6SubnetNameList, err = common.BatchCreateSubnet(frame, constant.IPv6, batchCreateSubnetNumber, subnetIpNumber)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(v6SubnetNameList)).To(Equal(batchCreateSubnetNumber))
				GinkgoWriter.Printf("succeed to batch create %v v6 subnet \n", len(v6SubnetNameList))
				endT1 := time.Since(startT1)
				GinkgoWriter.Printf("Time cost to create %v v6 subnet is %v \n", batchCreateSubnetNumber, endT1)
				startT1 = time.Now()
				v6SubnetObject = common.GetSubnetByName(frame, v6SubnetNameList[1])
				v6PoolNameList, err = common.BatchCreateIPPoolsInSpiderSubnet(frame, constant.IPv6, v6SubnetObject.Spec.Subnet, v6SubnetObject.Spec.IPs, batchCreateIPPoolNumber, ippoolIpNumber)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(v6PoolNameList)).To(Equal(batchCreateIPPoolNumber))
				ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
				defer cancel()
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetNameList[1])).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed to batch create %v v6 pool \n", len(v6PoolNameList))
				endT1 = time.Since(startT1)
				GinkgoWriter.Printf("Time cost to create %v v6 ippool is %v \n", batchCreateIPPoolNumber, endT1)
			}

			// batch delete ippool under subnet and record time
			startT2 := time.Now()
			var poolNameList []string
			poolNameList = append(append(poolNameList, v4PoolNameList...), v6PoolNameList...)
			ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			wg := sync.WaitGroup{}
			wg.Add(len(poolNameList))
			for _, poolName := range poolNameList {
				name := poolName
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					err = common.DeleteIPPoolUntilFinish(frame, name, ctx)
					Expect(err).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()
			ctx, cancel = context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetNameList[1], 0)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetNameList[1], 0)).NotTo(HaveOccurred())
			}
			endT2 := time.Since(startT2)
			GinkgoWriter.Printf("Time cost to delete %v ippool is %v \n", batchCreateIPPoolNumber, endT2)

			// batch delete subnet and record time
			startT3 := time.Now()
			ctx, cancel = context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			var subnetNameList []string
			subnetNameList = append(append(subnetNameList, v4SubnetNameList...), v6SubnetNameList...)
			wg = sync.WaitGroup{}
			wg.Add(len(subnetNameList))
			for _, subnetName := range subnetNameList {
				name := subnetName
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					err = common.DeleteSubnetUntilFinish(ctx, frame, name)
					Expect(err).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()
			endT3 := time.Since(startT3)
			GinkgoWriter.Printf("Time cost to delete %v subnet is %v \n", batchCreateSubnetNumber, endT3)
		})
	})
})
