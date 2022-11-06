// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package subnet_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	subnetmanager "github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test subnet", Serial, Label("subnet"), func() {
	var v4SubnetName, v6SubnetName, namespace string
	var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet
	var v4RandNum1, v4RandNum2, v6RandNum, v4Subnet, v6Subnet string

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
			GinkgoWriter.Printf("delete v4 subnet %v, v6 subnet %v \n", v4SubnetName, v6SubnetName)
			if frame.Info.IpV4Enabled && v4SubnetName != "" {
				Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled && v6SubnetName != "" {
				Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
			}
		})
	})

	// 1. Automatic pool creation succeeds in the case of a 1:1 number of subnet freeIPs and IPPool.
	// 2. Scale up the freeIPs of the subnet and scale up the replicas of the deployment, checking the status of free IPs in the subnet
	// 3. Scale down the replicas of the deployment, checking the status of free IPs in the subnet
	// 4. After deleting the deployment, all the assigned ip's, are returned to the subnet.
	// 5. Calculate the time spent on each step
	DescribeTable("Multiple automatic creation and recycling of ippools.",
		// `originialNum` is the initial number of subnets and pools to be created automatically
		// `scaleupNum` is the number of subnets and pool extensions created automatically
		func(originialNum int, scaleupNum int, timeout time.Duration) {
			// Define some test data
			var (
				deployNameList     []string
				deployOriginialNum int32  = 1
				deployScaleupNum   int32  = 2
				flexibleIPNumber   string = "+0"
			)

			// Generate subnet objects and customize gateway, subnet
			if frame.Info.IpV4Enabled {
				v4RandNum1 = common.GenerateRandomNumber(255)
				v4RandNum2 = common.GenerateRandomNumber(255)
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(1)
				Expect(v4SubnetObject).NotTo(BeNil())
				gateway := fmt.Sprintf("10.%s.%s.1", v4RandNum1, v4RandNum2)
				v4SubnetObject.Spec.Gateway = &gateway
				v4Subnet = fmt.Sprintf("10.%s.%s.0/24", v4RandNum1, v4RandNum2)
				v4SubnetObject.Spec.Subnet = v4Subnet
				GinkgoWriter.Printf("Generate v4subnet objects %v and customize gateway %v, subnet %v \n", v4SubnetName, *v4SubnetObject.Spec.Gateway, v4SubnetObject.Spec.Subnet)
			}
			if frame.Info.IpV6Enabled {
				v6RandNum = common.GenerateString(4, true)
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(1)
				Expect(v6SubnetObject).NotTo(BeNil())
				gateway := fmt.Sprintf("fd00:%s::1", v6RandNum)
				v6SubnetObject.Spec.Gateway = &gateway
				v6Subnet = fmt.Sprintf("fd00:%s::/120", v6RandNum)
				v6SubnetObject.Spec.Subnet = v6Subnet
				GinkgoWriter.Printf("Generate v6subnet objects %v and customize gateway %v, subnet %v \n", v6SubnetName, *v6SubnetObject.Spec.Gateway, v6SubnetObject.Spec.Subnet)
			}

			// Create a subnet with a specified number of IPs
			if frame.Info.IpV4Enabled {
				v4SubnetObject.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2-10.%s.%s.%s", v4RandNum1, v4RandNum2, v4RandNum1, v4RandNum2, strconv.Itoa(originialNum+1))}
				GinkgoWriter.Printf("Creating subnets %v using IPs %v \n", v4SubnetName, v4SubnetObject.Spec.IPs)
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6SubnetObject.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2-fd00:%s::%s", v6RandNum, v6RandNum, strconv.FormatInt(int64(originialNum+1), 16))}
				GinkgoWriter.Printf("Creating subnets %v using IPs %v \n", v6SubnetName, v6SubnetObject.Spec.IPs)
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
			}

			// Create a non-existent node and use the tag NodeSelector.kubernetes.io/hostname: NonExistentNodeName
			// so that the deployment doesn't actually start.
			NonExistentNodeName := "NonExistentNode" + tools.RandomName()
			nodeLabelSelector := map[string]string{corev1.LabelHostname: NonExistentNodeName}
			annotationMap := map[string]string{}
			subnetAnno := subnetmanager.AnnoSubnetItems{}
			if frame.Info.IpV4Enabled {
				subnetAnno.IPv4 = []string{v4SubnetName}
			}
			if frame.Info.IpV6Enabled {
				subnetAnno.IPv6 = []string{v6SubnetName}
			}
			b, err := json.Marshal(subnetAnno)
			Expect(err).NotTo(HaveOccurred())
			annotationMap[constant.AnnoSpiderSubnetPoolIPNumber] = flexibleIPNumber
			annotationMap[constant.AnnoSpiderSubnet] = string(b)
			deployNameList = common.BatchCreateDeployment(frame, originialNum, int(deployOriginialNum), namespace, annotationMap, nodeLabelSelector)
			GinkgoWriter.Printf("deployment %v successfully created \n", deployNameList)

			// Time consumption to create a certain number of pools automatically
			startT1 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, originialNum)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, originialNum)).NotTo(HaveOccurred())
			}
			GinkgoWriter.Println("The automatically created ippool is all ready to go")

			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
			// Check that all ip's on the subnet have been assigned
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(originialNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(originialNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}
			endT1 := time.Since(startT1)
			GinkgoWriter.Printf("%v pools are automatically created, time cost %v \n", originialNum, endT1)

			// Extend the freeIPs of the Subnet
			if frame.Info.IpV4Enabled {
				desiredV4SubnetObject := common.GetSubnetByName(frame, v4SubnetName)
				Expect(desiredV4SubnetObject).NotTo(BeNil())
				desiredV4SubnetObject.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2-10.%s.%s.%s", v4RandNum1, v4RandNum2, v4RandNum1, v4RandNum2, strconv.Itoa(scaleupNum+1))}
				Expect(common.PatchSpiderSubnet(frame, desiredV4SubnetObject, v4SubnetObject)).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Extend the freeIPs of the v4Subnet from %v to %v \n", originialNum, scaleupNum)
			}
			if frame.Info.IpV6Enabled {
				desiredV6SubnetObject := common.GetSubnetByName(frame, v6SubnetName)
				Expect(desiredV6SubnetObject).NotTo(BeNil())
				desiredV6SubnetObject.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2-fd00:%s::%s", v6RandNum, v6RandNum, strconv.FormatInt(int64(scaleupNum+1), 16))}
				Expect(common.PatchSpiderSubnet(frame, desiredV6SubnetObject, v6SubnetObject)).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Extend the freeIPs of the v6Subnet from %v to %v \n", originialNum, scaleupNum)
			}

			// Time consumption for automatically scaling up a certain number of pools
			startT2 := time.Now()
			// scaling up the replicas of the deployment
			wg := sync.WaitGroup{}
			wg.Add(len(deployNameList))
			for _, d := range deployNameList {
				name := d
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					deploy, err := frame.GetDeployment(name, namespace)
					Expect(err).NotTo(HaveOccurred())
					_, err = frame.ScaleDeployment(deploy, deployScaleupNum)
					Expect(err).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()
			GinkgoWriter.Printf("scaling up the replicas of the deployment from %v to %v \n", deployOriginialNum, deployScaleupNum)
			// At subnet IP:Pods = 1:1, Check the free IPs, it should all be allocated already.
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
			GinkgoWriter.Println("Check the free IPs again, it should all be allocated already.")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(scaleupNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(scaleupNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}
			endT2 := time.Since(startT2)
			GinkgoWriter.Printf("Allocated IP scaling up from %v to %v, time cost %v.\n", originialNum, scaleupNum, endT2)

			// Time consumption for automatically scaling down a certain number of pools
			startT3 := time.Now()
			// Scaling down the replicas of the deployment
			wg = sync.WaitGroup{}
			wg.Add(len(deployNameList))
			for _, d := range deployNameList {
				name := d
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					deploy, err := frame.GetDeployment(name, namespace)
					Expect(err).NotTo(HaveOccurred())
					_, err = frame.ScaleDeployment(deploy, deployOriginialNum)
					Expect(err).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()
			GinkgoWriter.Printf("scaling down the replicas of the deployment from %v to %v \n", deployScaleupNum, deployOriginialNum)

			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
			GinkgoWriter.Println("Checking AllocatedIP after automatic scaling down")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(originialNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(originialNum))).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}
			endT3 := time.Since(startT3)
			GinkgoWriter.Printf("Allocated IP scaling down from %v to %v, time cost %v.\n", scaleupNum, originialNum, endT3)

			// Time spent checking the AllocatedIP of the subnet and returning to the original state
			startT4 := time.Now()
			wg = sync.WaitGroup{}
			wg.Add(len(deployNameList))
			for _, d := range deployNameList {
				// delete deployment
				name := d
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(frame.DeleteDeployment(name, namespace)).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()

			// After deleting the resource, check the AllocatedIP of the subnet and expect it to return to its original state
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
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
			endT4 := time.Since(startT4)
			GinkgoWriter.Printf("subnet reclaim %v ip, time cost %v. \n", originialNum, endT4)

			// attaching Data to Reports
			AddReportEntry("Subnet Performance Results",
				fmt.Sprintf(`{"createOriginialNum":%v,"expansionOrReducedNum":%v,"createTime": %d , "expansionTime": %d,"reducedTime":%d, "deleteTime": %d }`,
					originialNum, scaleupNum-originialNum, int(endT1.Seconds()), int(endT2.Seconds()), int(endT3.Seconds()), int(endT4.Seconds())))

		},
		Entry("Multiple automatic creation and recycling of ippools, eventually the freeIPs in the subnet should be restored to its initial state",
			Label("I00006"), 30, 60, time.Minute*5),
	)

	Context("There are enough resources on the node that the deployment pod will eventually run", func() {
		var (
			deployName         string
			subnetIpNum        int   = 2
			deployOriginialNum int32 = 1
			deployScaleupNum   int32 = 2
			deployNameList     []string
			deployObject       *appsv1.Deployment
			flexibleIPNumber   string = "+0"
		)

		BeforeEach(func() {

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
		})

		// 1、Create 2 deployment with 1 IP from a subnet that has 2 IPs.
		// 2、Scale up 1 deployment to 2 IPs from a subnet that only 2 IPs, at which point the subnet's IPs are not enough.
		// 3、In this case, step 2 should fail
		// 4、Release a deployment, expect success scaling up 1 deployment to 2 IPs from a subnet that only 2 IPs
		// 5、clean
		It("There are enough resources on the node that the deployment pod will eventually run", Label("I00008"), func() {

			// create 2 deployment with 1 IP, from subnet who has 2 IP
			for i := 1; i <= 2; i++ {
				deployName = fmt.Sprintf("deploy-%v-%v", i, tools.RandomName())
				deployObject = common.GenerateExampleDeploymentYaml(deployName, namespace, deployOriginialNum)

				subnetAnno := subnetmanager.AnnoSubnetItems{}
				if frame.Info.IpV4Enabled {
					subnetAnno.IPv4 = []string{v4SubnetName}
				}
				if frame.Info.IpV6Enabled {
					subnetAnno.IPv6 = []string{v6SubnetName}
				}
				b, err := json.Marshal(subnetAnno)
				Expect(err).NotTo(HaveOccurred())
				subnetAnnoStr := string(b)

				deployObject.Spec.Template.Annotations = map[string]string{
					constant.AnnoSpiderSubnetPoolIPNumber: flexibleIPNumber,
					constant.AnnoSpiderSubnet:             subnetAnnoStr,
				}
				_, err = frame.CreateDeploymentUntilReady(deployObject, common.PodStartTimeout)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed to create deployment %v/%v \n", namespace, deployName)
				deployNameList = append(deployNameList, deployName)
			}

			// Verify that the number of ippools created automatically and the AllocatedIPCount in the subnet are accurate
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			GinkgoWriter.Printf("Check that ippool, subnet assignment ip records are automatically generated correctly")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, subnetIpNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, subnetIpNum)).NotTo(HaveOccurred())
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(subnetIpNum))).NotTo(HaveOccurred())
			}

			// Scale up 1 deployment with 2 IP, from subnet who has 2 IP
			// At this point the subnet ip is insufficient
			ctx, cancel = context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel()
			deployObject, err := frame.GetDeployment(deployNameList[0], namespace)
			Expect(err).NotTo(HaveOccurred())
			addPods, _, err := common.ScaleDeployUntilExpectedReplicas(frame, deployObject, int(deployScaleupNum), ctx)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Scale up deploy %v/%v replicas from %v to %v \n", namespace, deployNameList[0], deployOriginialNum, deployScaleupNum)

			// Insufficient ip, failed to Scale up deployment and get the Pod creation failure Event
			ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
			defer cancel()
			for _, pod := range addPods {
				Expect(frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
				GinkgoWriter.Printf("Insufficient subnet ip, pod %v/%v fails to run \n", pod.Namespace, pod.Name)
			}

			// Delete another deployment and wait for its ip to be released
			Expect(frame.DeleteDeployment(deployNameList[1], namespace)).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeed to delete deployment %v/%v \n", namespace, deployNameList[1])

			// There are enough resources on the node that the deployment pod will eventually run
			podList, err := frame.GetPodListByLabel(deployObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeed to running a new deployment pod \n")
			podList, err = frame.DeletePodListUntilReady(podList, common.PodReStartTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(podList.Items)).To(Equal(int(deployScaleupNum)))

			ctx, cancel = context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel()
			GinkgoWriter.Printf("Check the consistency of the ip recorded in the subnet with the ip recorded in the pool \n")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAndPoolIpConsistency(ctx, frame, v6SubnetName)).NotTo(HaveOccurred())
			}

			// delete all deployment
			Expect(frame.DeleteDeployment(deployNameList[0], namespace)).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeed to delete all deployment")

			// Wait for the allocatedIPCount in the subnet to return to their initial state
			ctx, cancel = context.WithTimeout(context.Background(), common.IPReclaimTimeout)
			defer cancel()
			GinkgoWriter.Printf("Wait for the freeIPs in the subnet to return to their initial state")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v4SubnetName, 0)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetAllocatedIPCount(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
				Expect(common.WaitIppoolNumberInSubnet(ctx, frame, v6SubnetName, 0)).NotTo(HaveOccurred())
			}
		})
	})
})
