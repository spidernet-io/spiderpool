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
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
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
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

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
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, subnetIpNum)
					err := common.CreateSubnet(frame, v4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, subnetIpNum)
					err := common.CreateSubnet(frame, v6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

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

			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, subnetAvailableIpNum)
					err = common.CreateSubnet(frame, v4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, subnetAvailableIpNum)
					err = common.CreateSubnet(frame, v6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

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
			stsYaml.Spec.Ordinals = &appsv1.StatefulSetOrdinals{Start: stsReplicasOriginialNum}
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
			}, 2*common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
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

	PContext("Validity of fields in subnet.spec", func() {
		var fixedIPNumber string = "+0"
		var deployOriginiaNum int32 = 1
		var deployName string

		BeforeEach(func() {
			deployName = "deploy" + tools.RandomName()
			v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 10)
			Expect(v4SubnetObject).NotTo(BeNil())
			v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 10)
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
			var v4Object, v6Object *spiderpool.SpiderSubnet
			var v4RouteValue, v6RouteValue []spiderpool.Route

			v4Dst := "0.0.0.0/0"
			ipv4Gw := strings.Split(v4SubnetObject.Spec.Subnet, "0/")[0] + "1"
			v6Dst := "::/0"
			ipv6Gw := strings.Split(v6SubnetObject.Spec.Subnet, "/")[0] + "1"
			subnetAnno := types.AnnoSubnetItem{}
			if frame.Info.IpV4Enabled {
				*v4Ipversion = int64(4)
				subnetAnno.IPv4 = []string{v4SubnetName}
				v4RouteValue = []spiderpool.Route{
					{
						Dst: v4Dst,
						Gw:  ipv4Gw,
					},
				}
				v4SubnetObject.Spec.Routes = v4RouteValue
				v4SubnetObject.Spec.Gateway = &ipv4Gw
				GinkgoWriter.Printf("Specify routes, gateways, etc. and then create subnets %v \n", v4SubnetName)
				err := common.CreateSubnet(frame, v4SubnetObject)
				Expect(err).NotTo(HaveOccurred())
				v4Object, err = common.GetSubnetByName(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(v4Object.Spec.IPVersion).To(Equal(v4Ipversion))
				Expect(v4Object.Spec.Routes[0].Dst).To(Equal(v4Dst))
				Expect(v4Object.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
				Expect(v4Object.Spec.Gateway).To(Equal(&ipv4Gw))
			}

			if frame.Info.IpV6Enabled {
				*v6Ipversion = int64(6)
				subnetAnno.IPv6 = []string{v6SubnetName}
				v6RouteValue = []spiderpool.Route{
					{
						Dst: v6Dst,
						Gw:  ipv6Gw,
					},
				}
				v6SubnetObject.Spec.Routes = v6RouteValue
				v6SubnetObject.Spec.Gateway = &ipv6Gw
				GinkgoWriter.Printf("Specify routes, gateways, etc. and then create subnets %v \n", v6SubnetName)
				err := common.CreateSubnet(frame, v6SubnetObject)
				Expect(err).NotTo(HaveOccurred())
				v6Object, err = common.GetSubnetByName(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(v6Object.Spec.IPVersion).To(Equal(v6Ipversion))
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
			GinkgoWriter.Printf("Try to create deploy %v/%v \n", namespace, deployName)
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
				v4SubnetObject, err = common.GetSubnetByName(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				v4Object = v4SubnetObject.DeepCopy()
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
				newV4Object := v4Object.DeepCopy()
				newV4Object.Spec.IPs = []string{"0.0.0.0"}
				err = common.PatchSpiderSubnet(frame, newV4Object, v4Object)
				GinkgoWriter.Printf("failed to add an ip that is not part of the CIDR to the subnet,err is %v. \n", err)
				Expect(err).To(HaveOccurred())

				GinkgoWriter.Println("Check if the changes were successful.")
				v4Object, err = common.GetSubnetByName(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(v4Object.Spec.Routes[0].Dst).To(Equal(newV4Dst))
				Expect(v4Object.Spec.Routes[0].Gw).To(Equal(newIpv4Gw))
				Expect(v4Object.Spec.Gateway).To(Equal(&newIpv4Gw))

				// Delete an ip that is being used in the subnet
				newV4Object = v4Object.DeepCopy()
				nextIpRange, err := ip.ConvertIPsToIPRanges(*newV4Object.Spec.IPVersion, []net.IP{nextIp})
				Expect(err).NotTo(HaveOccurred())
				newV4Object.Spec.IPs = nextIpRange
				err = common.PatchSpiderSubnet(frame, newV4Object, v4Object)
				GinkgoWriter.Printf("failed to remove an ip in use: %v. \n", err)
				Expect(err).To(HaveOccurred())

				// Delete unused ip in the subnet
				oldIpRange, err := ip.ConvertIPsToIPRanges(*newV4Object.Spec.IPVersion, oldIPs)
				Expect(err).NotTo(HaveOccurred())
				newV4Object.Spec.IPs = oldIpRange
				GinkgoWriter.Printf("Successfully deleted an unused IP %v.\n", newV4Object.Spec.IPs)
				Expect(common.PatchSpiderSubnet(frame, newV4Object, v4Object)).NotTo(HaveOccurred(), err)

				GinkgoWriter.Println("Subnet routing gateway updated successfully, manual pool creation does not change.")
				iPv4PoolObj, err = common.GetIppoolByName(frame, v4PoolName)
				Expect(err).NotTo(HaveOccurred())
				Expect(iPv4PoolObj.Spec.Routes[0].Dst).To(Equal(v4Dst))
				Expect(iPv4PoolObj.Spec.Routes[0].Gw).To(Equal(ipv4Gw))
				Expect(iPv4PoolObj.Spec.Gateway).To(Equal(&ipv4Gw))

				v4poolList, err = common.GetIppoolsInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Println("Older pools are not affected.")
				for _, pool := range v4poolList.Items {
					if pool.Name != v4PoolName {
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
				v6SubnetObject, err = common.GetSubnetByName(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				v6Object = v6SubnetObject.DeepCopy()
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
				newV6Object := v6Object.DeepCopy()
				newV6Object.Spec.IPs = []string{"::"}
				err = common.PatchSpiderSubnet(frame, newV6Object, v6Object)
				GinkgoWriter.Printf("failed to add an ip that is not part of the CIDR to the subnet,err is %v \n", err)
				Expect(err).To(HaveOccurred())

				GinkgoWriter.Println("Check if the changes were successful.")
				v6Object, err = common.GetSubnetByName(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				Expect(v6Object.Spec.Routes[0].Dst).To(Equal(newV6Dst))
				Expect(v6Object.Spec.Routes[0].Gw).To(Equal(newIpv6Gw))
				Expect(v6Object.Spec.Gateway).To(Equal(&newIpv6Gw))

				// Delete an v6 ip that is being used in the v6 subnet
				newV6Object = v6Object.DeepCopy()
				nextIpRange, err := ip.ConvertIPsToIPRanges(*v6Object.Spec.IPVersion, []net.IP{nextIp})
				Expect(err).NotTo(HaveOccurred())
				newV6Object.Spec.IPs = nextIpRange
				err = common.PatchSpiderSubnet(frame, newV6Object, v6Object)
				GinkgoWriter.Printf("failed to remove an ip in use: %v. \n", err)
				Expect(err).To(HaveOccurred())

				// Delete unused v6 ip in the v6 subnet
				oldIpRange, err := ip.ConvertIPsToIPRanges(*v6Object.Spec.IPVersion, oldIPs)
				Expect(err).NotTo(HaveOccurred())
				newV6Object.Spec.IPs = oldIpRange
				GinkgoWriter.Printf("Successfully deleted an unused IP %v. \n", newV6Object.Spec.IPs)
				Expect(common.PatchSpiderSubnet(frame, newV6Object, v6Object)).NotTo(HaveOccurred())

				GinkgoWriter.Println("Subnet routing gateway updated successfully, manual pool creation does not change.")
				iPv6PoolObj, err = common.GetIppoolByName(frame, v6PoolName)
				Expect(err).NotTo(HaveOccurred())
				Expect(iPv6PoolObj.Spec.Routes[0].Dst).To(Equal(v6Dst))
				Expect(iPv6PoolObj.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
				Expect(iPv6PoolObj.Spec.Gateway).To(Equal(&ipv6Gw))

				v6poolList, err = common.GetIppoolsInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Println("Older pools are not affected.")
				for _, pool := range v6poolList.Items {
					if pool.Name != v6PoolName {
						Expect(pool.Spec.Routes[0].Dst).To(Equal(v6Dst))
						Expect(pool.Spec.Routes[0].Gw).To(Equal(ipv6Gw))
						Expect(pool.Spec.Gateway).To(Equal(&ipv6Gw))
					}
				}
			}

			Expect(frame.DeleteDeployment(deployName, namespace)).NotTo(HaveOccurred())
		})

		It("The excludeIPs in the subnet will not be used by pools created automatically or manually. ", Label("I00004", "S00004"), func() {
			subnetAnno := types.AnnoSubnetItem{}
			// ExcludeIPs cannot be used by ippools that are created automatically
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetObject.Spec.ExcludeIPs = v4SubnetObject.Spec.IPs
					subnetAnno.IPv4 = []string{v4SubnetName}
					err := common.CreateSubnet(frame, v4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 subnet %v: %v \n", v4SubnetObject.Name, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6SubnetObject.Spec.ExcludeIPs = v6SubnetObject.Spec.IPs
					subnetAnno.IPv6 = []string{v6SubnetName}
					err := common.CreateSubnet(frame, v6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 subnet %v: %v \n", v6SubnetObject.Name, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			GinkgoWriter.Printf("succeed to create v4 subnet %v, v6 subnet %v \n", v4SubnetName, v6SubnetName)

			// Checking manually created ippools will automatically circumvent excludeIPs
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj := common.GenerateExampleIpv4poolObject(5)
				v4PoolObj.Spec.Subnet = v4SubnetObject.Spec.Subnet
				v4PoolObj.Spec.IPs = v4SubnetObject.Spec.IPs
				err := common.CreateIppool(frame, v4PoolObj)
				Expect(err).NotTo(Succeed())
				GinkgoWriter.Printf("Failed to create an IPv4 IPPool %v, error: %v. \n", v4PoolName, err)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj := common.GenerateExampleIpv6poolObject(5)
				v6PoolObj.Spec.Subnet = v6SubnetObject.Spec.Subnet
				v6PoolObj.Spec.IPs = v6SubnetObject.Spec.IPs
				err := common.CreateIppool(frame, v6PoolObj)
				Expect(err).NotTo(Succeed())
				GinkgoWriter.Printf("Failed to create an IPv6 IPPool %v, error: %v. \n", v6PoolName, err)
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
			GinkgoWriter.Printf("Try to create deploy %v/%v \n", namespace, deployName)
			Expect(frame.CreateDeployment(deployYaml)).To(Succeed())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				// Check the number of AllocatedIPCount in the subnet
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 0)).NotTo(HaveOccurred())
				// Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 0)).NotTo(HaveOccurred())
				// Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
			}

			var podList *corev1.PodList
			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(deployYaml.Spec.Template.Labels)
				if nil != err {
					GinkgoWriter.Printf("failed to get pod list %v \n", err)
					return false
				}
				// Compare Pod List numbers
				if len(podList.Items) != int(deployOriginiaNum) {
					return false
				}
				return true
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
			defer cancel()
			for _, pod := range podList.Items {
				err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, namespace, common.CNIFailedToSetUpNetwork)
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
				_, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 200)
				_, v4PoolObject = common.GenerateExampleIpv4poolObject(200)
				v4PoolObject.Spec.Subnet = v4SubnetObject.Spec.Subnet
				v4PoolObject.Spec.IPs = v4SubnetObject.Spec.IPs
			}
			if frame.Info.IpV6Enabled {
				_, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 200)
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
							subnet, err := common.GetSubnetByName(frame, v4SubnetName)
							Expect(err).NotTo(HaveOccurred())
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
							subnet, err := common.GetSubnetByName(frame, v6SubnetName)
							Expect(err).NotTo(HaveOccurred())
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
							pool, err := common.GetIppoolByName(frame, v4PoolName)
							Expect(err).NotTo(HaveOccurred())
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
							pool, err := common.GetIppoolByName(frame, v6PoolName)
							Expect(err).NotTo(HaveOccurred())
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
				v4SubnetObject, err = common.GetSubnetByName(frame, v4SubnetNameList[1])
				Expect(err).NotTo(HaveOccurred())
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
				v6SubnetObject, err = common.GetSubnetByName(frame, v6SubnetNameList[1])
				Expect(err).NotTo(HaveOccurred())
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

	Context("SpiderSubnet supports multiple interfaces", func() {
		var v4SubnetName1, v6SubnetName1, v4SubnetName2, v6SubnetName2 string
		var v4SubnetObject1, v6SubnetObject1, v4SubnetObject2, v6SubnetObject2 *spiderpool.SpiderSubnet
		var deployName string

		BeforeEach(func() {
			// Create multiple subnets
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetName1, v4SubnetObject1 = common.GenerateExampleV4SubnetObject(frame, 10)
					err := common.CreateSubnet(frame, v4SubnetObject1)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName1, err)
						return err
					}

					v4SubnetName2, v4SubnetObject2 = common.GenerateExampleV4SubnetObject(frame, 10)
					err = common.CreateSubnet(frame, v4SubnetObject2)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName2, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName1, v6SubnetObject1 = common.GenerateExampleV6SubnetObject(frame, 10)
					err := common.CreateSubnet(frame, v6SubnetObject1)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName1, err)
						return err
					}

					v6SubnetName2, v6SubnetObject2 = common.GenerateExampleV6SubnetObject(frame, 10)
					err = common.CreateSubnet(frame, v6SubnetObject2)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName2, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			DeferCleanup(func() {
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteSubnetByName(frame, v4SubnetName1)).NotTo(HaveOccurred())
					Expect(common.DeleteSubnetByName(frame, v4SubnetName2)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteSubnetByName(frame, v6SubnetName1)).NotTo(HaveOccurred())
					Expect(common.DeleteSubnetByName(frame, v6SubnetName2)).NotTo(HaveOccurred())
				}
			})
		})

		It("SpiderSubnet supports multiple interfaces", Label("I00012"), func() {
			var v4PoolNameList1, v4PoolNameList2, v6PoolNameList1, v6PoolNameList2 []string
			var v4IPAddress1, v4IPAddress2, v6IPAddress1, v6IPAddress2 string

			/*
				To construct multiple interfaces annotations which looks like this:

				annotations:
					k8s.v1.cni.cncf.io/networks: kube-system/macvlan-cni2
					ipam.spidernet.io/subnets: |-
						[{"interface": "eth0", "ipv4": ["subnet-demo-v4-1"], "ipv6": ["subnet-demo-v6-1"]},
						 {"interface": "net2", "ipv4": ["subnet-demo-v4-2"], "ipv6": ["subnet-demo-v6-2"]}]
			*/
			GinkgoWriter.Println("Generate annotations for subnets multiple interfaces Marshal")
			type AnnoSubnetsItem []types.AnnoSubnetItem
			subnetsAnno := AnnoSubnetsItem{
				types.AnnoSubnetItem{
					Interface: common.NIC1,
				}, {
					Interface: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				subnetsAnno[0].IPv4 = []string{v4SubnetName1}
				subnetsAnno[1].IPv4 = []string{v4SubnetName2}
			}
			if frame.Info.IpV6Enabled {
				subnetsAnno[0].IPv6 = []string{v6SubnetName1}
				subnetsAnno[1].IPv6 = []string{v6SubnetName2}
			}
			subnetsAnnoMarshal, err := json.Marshal(subnetsAnno)
			Expect(err).NotTo(HaveOccurred())

			// Generate example deploy yaml and create deploy
			deployName = "deploy-" + tools.RandomName()
			deployObj := common.GenerateExampleDeploymentYaml(deployName, namespace, 1)
			deployObj.Spec.Template.Annotations = map[string]string{
				common.MultusNetworks: fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100),
				// second Interface
				constant.AnnoSpiderSubnets: string(subnetsAnnoMarshal),
			}
			Expect(deployObj).NotTo(BeNil())

			GinkgoWriter.Printf("Try to create deploy %v/%v \n", namespace, deployName)
			Expect(frame.CreateDeployment(deployObj)).To(Succeed())

			// Checking the pod run status should all be running.
			var podList *corev1.PodList
			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(deployObj.Spec.Template.Labels)
				if nil != err || len(podList.Items) == 0 {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, 2*common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// Check that the corresponding ippool is created under each subnet.
			// Get the application IP in IPPool
			GinkgoWriter.Println("Get the IPs assigned by multiple NICs")
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName1, 1)).NotTo(HaveOccurred())
				v4PoolNameList1, err = common.GetPoolNameListInSubnet(frame, v4SubnetName1)
				Expect(err).NotTo(HaveOccurred())
				v4IPAddress1, err = common.GetPodIPAddressFromIppool(frame, v4PoolNameList1[0], podList.Items[0].Namespace, podList.Items[0].Name)
				Expect(err).NotTo(HaveOccurred())

				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName2, 1)).NotTo(HaveOccurred())
				v4PoolNameList2, err = common.GetPoolNameListInSubnet(frame, v4SubnetName2)
				Expect(err).NotTo(HaveOccurred())
				v4IPAddress2, err = common.GetPodIPAddressFromIppool(frame, v4PoolNameList2[0], podList.Items[0].Namespace, podList.Items[0].Name)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName1, 1)).NotTo(HaveOccurred())
				v6PoolNameList1, err = common.GetPoolNameListInSubnet(frame, v6SubnetName1)
				Expect(err).NotTo(HaveOccurred())
				v6IPAddress1, err = common.GetPodIPAddressFromIppool(frame, v6PoolNameList1[0], podList.Items[0].Namespace, podList.Items[0].Name)
				Expect(err).NotTo(HaveOccurred())

				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName2, 1)).NotTo(HaveOccurred())
				v6PoolNameList2, err = common.GetPoolNameListInSubnet(frame, v6SubnetName2)
				Expect(err).NotTo(HaveOccurred())
				v6IPAddress2, err = common.GetPodIPAddressFromIppool(frame, v6PoolNameList2[0], podList.Items[0].Namespace, podList.Items[0].Name)
				Expect(err).NotTo(HaveOccurred())
			}

			// multiple interfaces are in effect and Pods are getting IPs from each NIC
			GinkgoWriter.Println("Check that the subnet multiple interfaces are in effect.")
			ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				command := fmt.Sprintf("ip a | grep '%s' |grep '%s'", common.NIC1, v4IPAddress1)
				_, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, command, ctx)
				Expect(err).NotTo(HaveOccurred())

				command = fmt.Sprintf("ip a | grep '%s' |grep '%s'", common.NIC2, v4IPAddress2)
				_, err = frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, command, ctx)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				command := fmt.Sprintf("ip a | grep -w '%s' -A5 | grep '%s'", common.NIC1, v6IPAddress1)
				_, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, command, ctx)
				Expect(err).NotTo(HaveOccurred())

				command = fmt.Sprintf("ip a | grep -w '%s' -A5 | grep '%s'", common.NIC2, v6IPAddress2)
				_, err = frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, command, ctx)
				Expect(err).NotTo(HaveOccurred())
			}

			// delete deploy
			Expect(frame.DeleteDeployment(deployName, namespace)).NotTo(HaveOccurred())
			ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName1, 0)).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName2, 0)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName1, 0)).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName2, 0)).NotTo(HaveOccurred())
			}
		})
	})

	Context("The default subnet should support different controller types.", func() {
		var ClusterDefaultV4SubnetList, ClusterDefaultV6SubnetList []string
		var v4PoolNameList, v6PoolNameList []string
		var err error
		var replicasNum int32 = 1
		var controllerNum int = 4
		var controllerNameList []string
		var podName, deployName, stsName, dsName, jobName string

		BeforeEach(func() {
			// Get the default subnet
			// and verify that the default subnet functions properly
			By("get cluster default subnet")
			// TODO(ty-dc), Default subnet to be determined for now
			// ClusterDefaultV4SubnetList, ClusterDefaultV6SubnetList, err = common.GetClusterDefaultSubnet(frame)
			Expect(err).NotTo(HaveOccurred())
			if frame.Info.IpV4Enabled && len(ClusterDefaultV4SubnetList) == 0 {
				Skip("v4 Default Subnet function is not enabled")
			}
			if frame.Info.IpV6Enabled && len(ClusterDefaultV6SubnetList) == 0 {
				Skip("v6 Default Subnet function is not enabled")
			}

			By("Generate data for test.")
			podName = "pod-" + tools.RandomName()
			controllerNameList = append(controllerNameList, podName)
			deployName = "deploy-" + tools.RandomName()
			controllerNameList = append(controllerNameList, deployName)
			stsName = "sts-" + tools.RandomName()
			controllerNameList = append(controllerNameList, stsName)
			dsName = "ds-" + tools.RandomName()
			controllerNameList = append(controllerNameList, dsName)
			jobName = "job-" + tools.RandomName()
			controllerNameList = append(controllerNameList, jobName)
		})

		// TODO(ty-dc), Default subnet to be determined for now
		PIt("The default subnet should support different controller types.", Label("I00013", "I00014"), func() {
			// The default subnet should support different controller types.
			By("Creating different controllers")
			GinkgoWriter.Printf("create pod %s/%s. \n", namespace, podName)
			podObj := common.GenerateExamplePodYaml(podName, namespace)
			Expect(podObj).NotTo(BeNil())
			Expect(frame.CreatePod(podObj)).To(Succeed())

			GinkgoWriter.Printf("create deploy %s/%s. \n", namespace, deployName)
			deployObject := common.GenerateExampleDeploymentYaml(deployName, namespace, replicasNum)
			Expect(deployObject).NotTo(BeNil())
			Expect(frame.CreateDeployment(deployObject)).To(Succeed())

			GinkgoWriter.Printf("create statefulset %s/%s. \n", namespace, stsName)
			stsObject := common.GenerateExampleStatefulSetYaml(stsName, namespace, replicasNum)
			Expect(stsObject).NotTo(BeNil())
			Expect(frame.CreateStatefulSet(stsObject)).To(Succeed())

			GinkgoWriter.Printf("create daemonSet %s/%s. \n", namespace, dsName)
			dsObject := common.GenerateExampleDaemonSetYaml(dsName, namespace)
			Expect(dsObject).NotTo(BeNil())
			Expect(frame.CreateDaemonSet(dsObject)).To(Succeed())

			GinkgoWriter.Printf("create Job %s/%s. \n", namespace, jobName)
			jobObject := common.GenerateExampleJobYaml(common.JobTypeRunningForever, jobName, namespace, ptr.To(replicasNum))
			Expect(jobObject).NotTo(BeNil())
			Expect(frame.CreateJob(jobObject)).To(Succeed())

			GinkgoWriter.Println("Wait until all pods should be running.")
			var podList *corev1.PodList
			// this serves for daemonset object to make sure its corresponding pods number
			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			allPodNumber := len(nodeList.Items) + controllerNum
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err || len(podList.Items) != int(allPodNumber) {
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// The default subnet automatically creates an ippool for each controller (containing Pod).
			GinkgoWriter.Println("Check that the record Ippool in the status of the subnet is correct.")
			var v4PoolInSubnetNameList, v6PoolInSubnetNameList []string
			v4PoolInSubnetNameList = []string{}
			v6PoolInSubnetNameList = []string{}
			for _, name := range controllerNameList {
				if frame.Info.IpV4Enabled {
					bingo := 0
					v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, ClusterDefaultV4SubnetList[0])
					Expect(err).NotTo(HaveOccurred())
					for _, poolName := range v4PoolNameList {
						// Get the ippool automatically created by the default subnet and check its uniqueness
						if ok := strings.Contains(poolName, name); ok {
							GinkgoWriter.Printf("The default v4 subnet automatically creates an IPPool '%s' and Pod '%s' running. \n", poolName, name)
							v4PoolInSubnetNameList = append(v4PoolInSubnetNameList, poolName)
							bingo++
						}
					}
					Expect(bingo).To(Equal(1))
				}
				if frame.Info.IpV6Enabled {
					bingo := 0
					v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, ClusterDefaultV6SubnetList[0])
					Expect(err).NotTo(HaveOccurred())
					for _, poolName := range v6PoolNameList {
						if ok := strings.Contains(poolName, name); ok {
							GinkgoWriter.Printf("The default v6 subnet automatically creates an IPPool '%s' and Pod '%s' running. \n", poolName, name)
							v6PoolInSubnetNameList = append(v6PoolInSubnetNameList, poolName)
							bingo++
						}
					}
					Expect(bingo).To(Equal(1))
				}
			}

			// Delete all resources by deleting namespace,
			// expecting all ip's and ippool's to be reclaimed
			By("Delete all resources.")
			Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
			Eventually(func() bool {
				podList, err = frame.GetPodList(client.InNamespace(namespace))
				if nil != err || len(podList.Items) != 0 {
					return false
				}
				GinkgoWriter.Println("Waiting for all resources to be released.")
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// After deleting a resource, the ippool should be automatically reclaimed
			// and the ippool recorded in the subnet will also be reclaimed
			By("Checking resource release.")
			Eventually(func() bool {
				if frame.Info.IpV4Enabled {
					for _, poolName := range v4PoolInSubnetNameList {
						_, err := common.GetIppoolByName(frame, poolName)
						if err == nil {
							GinkgoWriter.Printf("v4 pool '%s' still exists, please wait for deletion to complete.\n", poolName)
							return false
						}
						GinkgoWriter.Printf("The v4 ippool %v has been reclaimed. \n", poolName)
					}
					for _, controllerName := range controllerNameList {
						v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, ClusterDefaultV4SubnetList[0])
						Expect(err).NotTo(HaveOccurred())
						for _, v := range v4PoolNameList {
							if ok := strings.Contains(v, controllerName); ok {
								GinkgoWriter.Printf("The v4 ippool %v in the subnet has been reclaimed. \n", v)
								return false
							}
						}
					}
				}
				if frame.Info.IpV6Enabled {
					for _, poolName := range v6PoolInSubnetNameList {
						_, err := common.GetIppoolByName(frame, poolName)
						if err == nil {
							GinkgoWriter.Printf("v6 pool '%s' still exists, please wait for deletion to complete.\n", poolName)
							return false
						}
						GinkgoWriter.Printf("The v6 ippool %v has been reclaimed. \n", poolName)
					}
					for _, controllerName := range controllerNameList {
						v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, ClusterDefaultV6SubnetList[0])
						Expect(err).NotTo(HaveOccurred())
						for _, v := range v6PoolNameList {
							if ok := strings.Contains(v, controllerName); ok {
								GinkgoWriter.Printf("The v6 ippool %v in the subnet has been reclaimed. \n", v)
								return false
							}
						}
					}
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})
	})

	Context("Reserved IPPool usage.", func() {

		var (
			subnetIpNum                        int   = 5
			replicasNum                        int32 = 1
			longAppName                        string
			v4PoolNameList, v6PoolNameList     []string
			annotationMap                      map[string]string
			err                                error
			v4SubnetNameList, v6SubnetNameList []string
			stsOrdinalsStartNum                int32 = 2
		)

		BeforeEach(func() {
			// Ability to create fixed IPPools for applications with very long names
			longAppName = "long-app-name-" + tools.RandomName() + tools.RandomName()
			v4SubnetNameList, v6SubnetNameList = []string{}, []string{}

			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, subnetIpNum)
					err = common.CreateSubnet(frame, v4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
						return err
					}
					v4SubnetNameList = append(v4SubnetNameList, v4SubnetName)
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, subnetIpNum)
					err = common.CreateSubnet(frame, v6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
						return err
					}
					v6SubnetNameList = append(v6SubnetNameList, v6SubnetName)
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			subnetAnno := types.AnnoSubnetItem{}
			if frame.Info.IpV4Enabled {
				subnetAnno.IPv4 = []string{v4SubnetName}
			}
			if frame.Info.IpV6Enabled {
				subnetAnno.IPv6 = []string{v6SubnetName}
			}
			subnetAnnoMarshal, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())

			annotationMap = map[string]string{
				// Set the annotation ipam.spidernet.io/ippool-reclaim: "false"
				// to prevent the fixed pool from being deleted when the application is deleted.
				constant.AnnoSpiderSubnetReclaimIPPool: "false",
				constant.AnnoSpiderSubnet:              string(subnetAnnoMarshal),
			}

			DeferCleanup(func() {
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
		})

		// I00015: Applications with the same name and type can use the reserved IPPool.
		// I00016: Applications with the same name and different types cannot use the reserved IPPool.
		// I00017: Ability to create fixed IPPools for applications with very long names
		// I00018: automatic IPPool IPs are not modifiable
		// I00019: Change the annotation ipam.spidernet.io/ippool-reclaim to true and the reserved IPPool will be reclaimed.
		// I00021: Pod works correctly when multiple NICs are specified by annotations for applications of the same name
		It("Use of reserved IPPool by applications of the same name, same type/different types", Label("I00015", "I00016", "I00017", "I00018", "I00019", "I00021"), func() {

			GinkgoWriter.Printf("Specify annotations for the application %v/%v, and create \n", namespace, longAppName)
			deployObject := common.GenerateExampleDeploymentYaml(longAppName, namespace, replicasNum)
			deployObject.Spec.Template.Annotations = annotationMap
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

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
			Expect(frame.WaitPodListRunning(deployObject.Spec.Template.Labels, int(replicasNum), ctx)).NotTo(HaveOccurred())
			podList, err := frame.GetPodListByLabel(deployObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// Delete application
			GinkgoWriter.Printf("Delete application %v/%v \n", namespace, longAppName)
			Expect(frame.DeleteDeploymentUntilFinish(longAppName, namespace, common.ResourceDeleteTimeout)).NotTo(HaveOccurred())

			// Checkpoint: automatic IPPool'IPs are not modifiable
			if frame.Info.IpV4Enabled {
				autoV4PoolObject, err := common.GetIppoolByName(frame, v4PoolNameList[0])
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("IPs for v4 automated pools %v cannot be edited.\n", autoV4PoolObject.Name)
				desiredV4PoolObject := autoV4PoolObject.DeepCopy()
				ips, err := ip.ParseIPRanges(*autoV4PoolObject.Spec.IPVersion, autoV4PoolObject.Spec.IPs)
				Expect(err).NotTo(HaveOccurred())
				updateIPs, err := ip.ConvertIPsToIPRanges(*autoV4PoolObject.Spec.IPVersion, []net.IP{ip.NextIP(ips[0])})
				Expect(err).NotTo(HaveOccurred())
				desiredV4PoolObject.Spec.IPs = updateIPs
				err = common.PatchIppool(frame, desiredV4PoolObject, autoV4PoolObject)
				Expect(err).To(HaveOccurred())
				GinkgoWriter.Printf("Automatic v4 IPPool' IP editing failed, error: %v.\n", err.Error())
			}
			if frame.Info.IpV6Enabled {
				autoV6PoolObject, err := common.GetIppoolByName(frame, v6PoolNameList[0])
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("IPs for v6 automated pools %v cannot be edited.\n", autoV6PoolObject.Name)
				desiredV6PoolObject := autoV6PoolObject.DeepCopy()
				ips, err := ip.ParseIPRanges(*autoV6PoolObject.Spec.IPVersion, autoV6PoolObject.Spec.IPs)
				Expect(err).NotTo(HaveOccurred())
				updateIPs, err := ip.ConvertIPsToIPRanges(*autoV6PoolObject.Spec.IPVersion, []net.IP{ip.NextIP(ips[0])})
				Expect(err).NotTo(HaveOccurred())
				desiredV6PoolObject.Spec.IPs = updateIPs
				err = common.PatchIppool(frame, desiredV6PoolObject, autoV6PoolObject)
				Expect(err).To(HaveOccurred())
				GinkgoWriter.Printf("Automatic v6 IPPool' IP editing failed, error: %v. \n", err.Error())
			}

			By(`Applications with the same name and different types cannot use the reserved IPPool.`)
			// Create the application again with the same name but a different controller type
			// Checkpoint: The automatic IPPool adapts automatically even if the number of copies is changed
			newReplicasNum := int(replicasNum) + int(1)
			stsObject := common.GenerateExampleStatefulSetYaml(longAppName, namespace, int32(newReplicasNum))

			GinkgoWriter.Printf("set sts %v/%v annotations ipam.spidernet.io/reclaimippool to true. \n", namespace, longAppName)
			annotationMap[constant.AnnoSpiderSubnetReclaimIPPool] = "true"
			stsObject.Spec.Template.Annotations = annotationMap
			stsObject.Spec.Ordinals = &appsv1.StatefulSetOrdinals{Start: stsOrdinalsStartNum}
			Expect(frame.CreateStatefulSet(stsObject)).NotTo(HaveOccurred())

			// Applications with the same name but different controller types can be created successfully,
			// but their IPs will not be recorded in the stock IPPool, instead a new fixed IP pool will be created.
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			Expect(frame.WaitPodListRunning(stsObject.Spec.Template.Labels, newReplicasNum, ctx)).NotTo(HaveOccurred())
			podList, err = frame.GetPodListByLabel(stsObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Printf("Different types of applications with the same name will create a new v4 fixed IPPool. \n")
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 2)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Printf("Different types of applications with the same name will create a new v6 fixed IPPool. \n")
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 2)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}

			// Delete the application
			GinkgoWriter.Printf("delete the application %v/%v \n.", longAppName, namespace)
			Expect(frame.DeleteStatefulSet(longAppName, namespace)).NotTo(HaveOccurred())

			By("Applications with the same name and the same type will inherit the reserved IPPool.")
			type AnnoSubnetsItem []types.AnnoSubnetItem
			subnetsAnno := AnnoSubnetsItem{
				types.AnnoSubnetItem{
					Interface: common.NIC1,
				}, {
					Interface: common.NIC2,
				},
			}
			var newV4SubnetName, newV6SubnetName string
			var newV4SubnetObject, newV6SubnetObject *spiderpool.SpiderSubnet

			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					newV4SubnetName, newV4SubnetObject = common.GenerateExampleV4SubnetObject(frame, subnetIpNum)
					err = common.CreateSubnet(frame, newV4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", newV4SubnetName, err)
						return err
					}
					v4SubnetNameList = append(v4SubnetNameList, newV4SubnetName)
					subnetsAnno[0].IPv4 = []string{v4SubnetName}
					subnetsAnno[1].IPv4 = []string{newV4SubnetName}
				}
				if frame.Info.IpV6Enabled {
					newV6SubnetName, newV6SubnetObject = common.GenerateExampleV6SubnetObject(frame, subnetIpNum)
					err = common.CreateSubnet(frame, newV6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", newV6SubnetName, err)
						return err
					}
					v6SubnetNameList = append(v6SubnetNameList, newV6SubnetName)
					subnetsAnno[0].IPv6 = []string{v6SubnetName}
					subnetsAnno[1].IPv6 = []string{newV6SubnetName}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			subnetsAnnoMarshal, err := json.Marshal(subnetsAnno)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("Generate multi-NIC annotations for same name app  %v/%v \n", namespace, longAppName)
			annotationMap[constant.AnnoSpiderSubnets] = string(subnetsAnnoMarshal)
			annotationMap[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100)

			// Delete Single Card Annotations
			delete(annotationMap, constant.AnnoSpiderSubnet)

			// Create an application with the same name and modify the network card type to multiple network cards
			GinkgoWriter.Printf("Create a multi-card application of the same name %v/%v \n", namespace, longAppName)
			sameDeployNameObject := common.GenerateExampleDeploymentYaml(longAppName, namespace, replicasNum)
			sameDeployNameObject.Spec.Template.Annotations = annotationMap
			Expect(frame.CreateDeployment(sameDeployNameObject)).NotTo(HaveOccurred())

			// Check if the Pod IP of an application with the same name is recorded in IPPool
			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			Expect(frame.WaitPodListRunning(sameDeployNameObject.Spec.Selector.MatchLabels, int(replicasNum), ctx)).NotTo(HaveOccurred())
			podList, err = frame.GetPodListByLabel(sameDeployNameObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(ok).NotTo(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Printf("The expected v4 IPPool have been created for the multiple network interfaces. \n")
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, newV4SubnetName, 1)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Printf("The expected v6 IPPool have been created for the multiple network interfaces. \n")
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, newV6SubnetName, 1)).NotTo(HaveOccurred())
			}

			By("Change the annotation ipam.spidernet.io/ippool-reclaim to true and the reserved IPPool will be reclaimed.")
			// Checkpoint: Modify the annotation to ipam.spidernet.io/ippool-reclaim to true，
			// whether the automatic pool is reclaimed
			deploy, err := frame.GetDeployment(longAppName, namespace)
			Expect(err).NotTo(HaveOccurred())

			annotationMap[constant.AnnoSpiderSubnetReclaimIPPool] = "true"
			deploy.Spec.Template.Annotations = annotationMap
			Expect(frame.UpdateResource(deploy)).NotTo(HaveOccurred())

			// Delete the application
			GinkgoWriter.Printf("delete deploy %v/%v with ReclaimIPPool is true ", namespace, longAppName)
			Expect(frame.DeleteDeploymentUntilFinish(longAppName, namespace, common.ResourceDeleteTimeout)).NotTo(HaveOccurred())

			GinkgoWriter.Println("The fixed IPPool should have been reclaimed automatically.")
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

		It("Redundant IPs for automatic IPPool, which cannot be used by other applications", Label("I00020"), func() {
			deployObject := common.GenerateExampleDeploymentYaml(longAppName, namespace, replicasNum)
			GinkgoWriter.Printf("Set deployment %v/%v annotation ipam.spidernet.io/ippool-ip-number to +2 . \n", namespace, longAppName)
			annotationMap[constant.AnnoSpiderSubnetPoolIPNumber] = "+2"
			deployObject.Spec.Template.Annotations = annotationMap
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			// Check that the IP of the subnet record matches the record in IPPool
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podAnno := types.AnnoPodIPPoolValue{}
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Println("Waiting for the v4 fixed pool to be created.")
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
				v4PoolNameList, err = common.GetPoolNameListInSubnet(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				podAnno.IPv4Pools = v4PoolNameList
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Println("Waiting for the v6 fixed pool to be created.")
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 1)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
				v6PoolNameList, err = common.GetPoolNameListInSubnet(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				podAnno.IPv6Pools = v6PoolNameList
			}

			depName := "dep-" + tools.RandomName()
			GinkgoWriter.Printf("Generate a new deploy object %v/%v. \n", namespace, depName)
			podAnnoMarshal, err := json.Marshal(podAnno)
			Expect(err).NotTo(HaveOccurred())
			podAnnoStr := string(podAnnoMarshal)
			newDeployObject := common.GenerateExampleDeploymentYaml(depName, namespace, replicasNum)

			GinkgoWriter.Println("Using the annotation ipam.spidernet.io/ippool to specify a fixed IP pool.")
			newDeployObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podAnnoStr}

			ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podList, err := common.CreateDeployUntilExpectedReplicas(frame, newDeployObject, ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get the Pod creation failure Event
			GinkgoWriter.Println("The new application pod is unable to run because the specified fixed IP pool is dedicated.")
			for _, pod := range podList.Items {
				Expect(frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
			}
		})
	})

	It("Dirty data in the subnet should be recycled.", Label("I00022"), func() {
		var (
			subnetIpNum                    int = 5
			dirtyPoolName                  string
			v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet
		)

		dirtyPoolName = "dirtyPool-" + tools.RandomName()
		GinkgoWriter.Printf("generate dirty Pool name: %v \n", dirtyPoolName)

		if frame.Info.IpV4Enabled {
			Eventually(func() error {
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, subnetIpNum)
				err := common.CreateSubnet(frame, v4SubnetObject)
				if err != nil {
					GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
					return err
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			preAllocations := spiderpool.PoolIPPreAllocations{
				dirtyPoolName: spiderpool.PoolIPPreAllocation{
					IPs: v4SubnetObject.Spec.IPs,
				},
			}
			MarshalPreAllocations, err := convert.MarshalSubnetAllocatedIPPools(preAllocations)
			Expect(err).NotTo(HaveOccurred())

			// Update dirty data to IPv4 subnet.Status.ControlledIPPools
			Eventually(func() bool {
				v4SubnetObject, err = common.GetSubnetByName(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				v4SubnetObject.Status.AllocatedIPCount = ptr.To(int64(1))
				v4SubnetObject.Status.ControlledIPPools = MarshalPreAllocations
				GinkgoWriter.Printf("update subnet %v for adding dirty record: %+v \n", v4SubnetName, *v4SubnetObject)
				if err = frame.UpdateResourceStatus(v4SubnetObject); err != nil {
					GinkgoWriter.Printf("failed to update v4 subnet status,error is: %v \n", err)
					return false
				}
				return true
			}, common.PodReStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// After triggering recycling, dirty data should not exist.
			Eventually(func() bool {
				newV4SubnetObject, err := common.GetSubnetByName(frame, v4SubnetName)
				Expect(err).NotTo(HaveOccurred())
				if *newV4SubnetObject.Status.AllocatedIPCount != int64(0) {
					GinkgoWriter.Printf("AllocatedIPCount have not been recycledt: %v", *newV4SubnetObject.Status.AllocatedIPCount)
					return false
				}
				if newV4SubnetObject.Status.ControlledIPPools != nil {
					GinkgoWriter.Printf("ControlledIPPools have not been recycled: %v", *newV4SubnetObject.Status.ControlledIPPools)
					return false
				}
				return true
			}, common.IPReclaimTimeout, common.ForcedWaitingTime).Should(BeTrue())
		}
		if frame.Info.IpV6Enabled {
			Eventually(func() error {
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, subnetIpNum)
				err := common.CreateSubnet(frame, v6SubnetObject)
				if err != nil {
					GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
					return err
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			preAllocations := spiderpool.PoolIPPreAllocations{
				dirtyPoolName: spiderpool.PoolIPPreAllocation{
					IPs: v6SubnetObject.Spec.IPs,
				},
			}

			MarshalPreAllocations, err := convert.MarshalSubnetAllocatedIPPools(preAllocations)
			Expect(err).NotTo(HaveOccurred())

			// Update dirty data to IPv6 subnet.Status.ControlledIPPools
			Eventually(func() bool {
				v6SubnetObject, err = common.GetSubnetByName(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				v6SubnetObject.Status.AllocatedIPCount = ptr.To(int64(1))
				v6SubnetObject.Status.ControlledIPPools = MarshalPreAllocations
				GinkgoWriter.Printf("update subnet %v for adding dirty record: %+v \n", v6SubnetName, *v6SubnetObject)
				if err = frame.UpdateResourceStatus(v6SubnetObject); err != nil {
					GinkgoWriter.Printf("failed to update v6 subnet status,error is: %v", err)
					return false
				}
				return true
			}, common.PodReStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// After triggering recycling, dirty data should not exist.
			Eventually(func() bool {
				newV6SubnetObject, err := common.GetSubnetByName(frame, v6SubnetName)
				Expect(err).NotTo(HaveOccurred())
				if *newV6SubnetObject.Status.AllocatedIPCount != int64(0) {
					GinkgoWriter.Printf("AllocatedIPCount have not been recycledt: %v", *newV6SubnetObject.Status.AllocatedIPCount)
					return false
				}
				if newV6SubnetObject.Status.ControlledIPPools != nil {
					GinkgoWriter.Printf("ControlledIPPools have not been recycled: %v", *newV6SubnetObject.Status.ControlledIPPools)
					return false
				}
				return true
			}, common.IPReclaimTimeout, common.ForcedWaitingTime).Should(BeTrue())
		}
	})

	It("SpiderSubnet feature doesn't support orphan pod", Label("I00023"), func() {
		podName := "orphan-pod"
		podYaml := common.GenerateExamplePodYaml(podName, namespace)

		subnetAnno := types.AnnoSubnetItem{}
		if frame.Info.IpV4Enabled {
			subnetAnno.IPv4 = []string{v4SubnetName}
		}
		if frame.Info.IpV6Enabled {
			subnetAnno.IPv6 = []string{v6SubnetName}
		}
		subnetAnnoMarshal, err := json.Marshal(subnetAnno)
		Expect(err).NotTo(HaveOccurred())

		podYaml.Annotations = map[string]string{
			constant.AnnoSpiderSubnet: string(subnetAnnoMarshal),
		}

		GinkgoWriter.Printf("succeeded to generate pod yaml with same NIC name annotation: %+v. \n", podYaml)

		Expect(frame.CreatePod(podYaml)).To(Succeed())
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
		defer cancel()
		GinkgoWriter.Printf("wait for one minute that pod %v/%v would not ready. \n", namespace, podName)
		_, err = frame.WaitPodStarted(podName, namespace, ctx)
		Expect(err).To(HaveOccurred())
	})

	Context("The Pod would not be setup and no auto Pools created when the SpiderSubnet AutoPool feature is disabled", Label("I00024"), Serial, func() {
		const configYamlStr = "conf.yml"
		BeforeEach(func() {
			// make sure configmap 'enableAutoPoolForApplication' is 'false'
			configmap, err := frame.GetConfigmap(common.SpiderPoolConfigmapName, common.SpiderPoolConfigmapNameSpace)
			Expect(err).NotTo(HaveOccurred())
			var cmConfig types.SpiderpoolConfigmapConfig
			configStr, ok := configmap.Data[configYamlStr]
			Expect(ok).To(BeTrue())
			err = yaml.Unmarshal([]byte(configStr), &cmConfig)
			Expect(err).NotTo(HaveOccurred())

			if cmConfig.EnableAutoPoolForApplication == true {
				GinkgoWriter.Println("try to update ConfigMap spiderpool-conf EnableAutoPoolForApplication to be false")
				cmConfig.EnableAutoPoolForApplication = false
				marshal, err := yaml.Marshal(cmConfig)
				Expect(err).NotTo(HaveOccurred())

				configmap.Data[configYamlStr] = string(marshal)
				err = frame.KClient.Update(context.TODO(), configmap)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Println("succeeded to update ConfigMap spiderpool-conf EnableAutoPoolForApplication to be false")

				Eventually(func() bool {
					var newCmConfig types.SpiderpoolConfigmapConfig
					configmap, err := frame.GetConfigmap(common.SpiderPoolConfigmapName, common.SpiderPoolConfigmapNameSpace)
					if err != nil {
						return false
					}
					configStr, ok := configmap.Data[configYamlStr]
					if !ok {
						return false
					}
					err = yaml.Unmarshal([]byte(configStr), &newCmConfig)
					if err != nil {
						return false
					}
					GinkgoWriter.Println("newCmConfig.EnableAutoPoolForApplication:", newCmConfig.EnableAutoPoolForApplication)
					if newCmConfig.EnableAutoPoolForApplication != false {
						GinkgoWriter.Printf("enableAutoPoolForApplication is not false, but %v , waiting...\n", newCmConfig.EnableAutoPoolForApplication)
						return false
					}
					return true
				}, common.ResourceDeleteTimeout*2, common.ForcedWaitingTime).Should(BeTrue())

				// After modifying the configuration file, restart the spiderpool-agent and spiderpool-controller to make the configuration take effect.
				wg := sync.WaitGroup{}
				wg.Add(2)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					spiderpoolAgentPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolAgent})
					Expect(err).NotTo(HaveOccurred())
					GinkgoWriter.Printf("Get the podList of spiderpool-agent %v \n", len(spiderpoolAgentPodList.Items))
					spiderpoolAgentPodList, err = frame.DeletePodListUntilReady(spiderpoolAgentPodList, common.PodReStartTimeout)
					Expect(err).NotTo(HaveOccurred(), "failed to reboot the podList of spiderpool-agent %v", spiderpoolAgentPodList.Items)
				}()

				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					spiderpoolControllerPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
					Expect(err).NotTo(HaveOccurred())
					GinkgoWriter.Printf("Get the podList of spiderpool-controller %v \n", len(spiderpoolControllerPodList.Items))
					spiderpoolControllerPodList, err = frame.DeletePodListUntilReady(spiderpoolControllerPodList, common.PodReStartTimeout)
					Expect(err).NotTo(HaveOccurred(), "failed to reboot the podList of spiderpool-controller  %v", spiderpoolControllerPodList.Items)
				}()
				wg.Wait()
			}

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}

				// change the configmap 'enableAutoPoolForApplication' back to be 'true'
				configmap, err := frame.GetConfigmap(common.SpiderPoolConfigmapName, common.SpiderPoolConfigmapNameSpace)
				Expect(err).NotTo(HaveOccurred())
				var cmConfig types.SpiderpoolConfigmapConfig
				configStr, ok := configmap.Data[configYamlStr]
				Expect(ok).To(BeTrue())
				err = yaml.Unmarshal([]byte(configStr), &cmConfig)
				Expect(err).NotTo(HaveOccurred())

				if cmConfig.EnableAutoPoolForApplication == false {
					GinkgoWriter.Printf("try to update ConfigMap spiderpool-conf EnableAutoPoolForApplication to be true in the end")
					cmConfig.EnableAutoPoolForApplication = true
					marshal, err := yaml.Marshal(cmConfig)
					Expect(err).NotTo(HaveOccurred())

					configmap.Data[configYamlStr] = string(marshal)
					err = frame.KClient.Update(context.TODO(), configmap)
					Expect(err).NotTo(HaveOccurred())

					// After modifying the configuration file, restart the spiderpool-agent and spiderpool-controller to make the configuration take effect.
					wg := sync.WaitGroup{}
					wg.Add(2)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						spiderpoolAgentPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolAgent})
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Printf("Get the podList of spiderpool-agent %v \n", len(spiderpoolAgentPodList.Items))
						spiderpoolAgentPodList, err = frame.DeletePodListUntilReady(spiderpoolAgentPodList, common.PodReStartTimeout)
						Expect(err).NotTo(HaveOccurred(), "failed to reboot the podList of spiderpool-agent %v", spiderpoolAgentPodList.Items)
					}()

					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						spiderpoolControllerPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
						Expect(err).NotTo(HaveOccurred())
						GinkgoWriter.Printf("Get the podList of spiderpool-controller %v \n", len(spiderpoolControllerPodList.Items))
						spiderpoolControllerPodList, err = frame.DeletePodListUntilReady(spiderpoolControllerPodList, common.PodReStartTimeout)
						Expect(err).NotTo(HaveOccurred(), "failed to reboot the podList of spiderpool-controller  %v", spiderpoolControllerPodList.Items)
					}()
					wg.Wait()
				}
			})
		})

		It("The Pod would not start up", func() {
			depName := "dep-" + tools.RandomName()
			GinkgoWriter.Printf("Generate a new deploy object %v/%v. \n", namespace, depName)
			subnetAnno := types.AnnoSubnetItem{}
			if frame.Info.IpV4Enabled {
				subnetAnno.IPv4 = []string{v4SubnetName}
			}
			if frame.Info.IpV6Enabled {
				subnetAnno.IPv6 = []string{v6SubnetName}
			}
			subnetAnnoMarshal, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Println("Using the annotation 'ipam.spidernet.io/subnet' to specify auto pool for application")
			newDeployObject := common.GenerateExampleDeploymentYaml(depName, namespace, 1)
			newDeployObject.Spec.Template.Annotations = map[string]string{
				constant.AnnoSpiderSubnet: string(subnetAnnoMarshal),
			}

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podList, err := common.CreateDeployUntilExpectedReplicas(frame, newDeployObject, ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get the Pod creation failure Event
			autoPoolDisabledErrorString := "it's invalid to use 'ipam.spidernet.io/subnets' or 'ipam.spidernet.io/subnet' annotation when Auto-Pool feature is disabled"
			GinkgoWriter.Println("The Pod would not start up due to AutoPool for application feature is disabled")
			for _, pod := range podList.Items {
				Expect(frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, autoPoolDisabledErrorString)).To(Succeed())
			}

			// make sure no auto ippools created
			autoPoolMatchLabels := client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(appsv1.SchemeGroupVersion.String()),
				constant.LabelIPPoolOwnerApplicationKind:      constant.KindDeployment,
				constant.LabelIPPoolOwnerApplicationNamespace: namespace,
				constant.LabelIPPoolOwnerApplicationName:      newDeployObject.Name,
			}
			poolList, err := common.GetAllIppool(frame, autoPoolMatchLabels)
			Expect(err).NotTo(HaveOccurred())
			Expect(poolList.Items).To(HaveLen(0))
		})
	})
})
