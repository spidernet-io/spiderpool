// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package subnet_test

import (
	"context"
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
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test subnet", Label("subnet"), func() {
	var v4SubnetName, v6SubnetName, namespace, deployName string
	var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet
	var v4RandNum1, v4RandNum2, v6RandNum, v4Subnet, v6Subnet string
	var poolNameList []string

	BeforeEach(func() {
		// Init namespace and create
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

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
			GinkgoWriter.Printf("Generate v4subnet objects %v and customize gateway %v, subnet %v \n", v4SubnetName, v4SubnetObject.Spec.Gateway, v4SubnetObject.Spec.Subnet)
		}
		if frame.Info.IpV6Enabled {
			v6RandNum = common.GenerateString(4, true)
			v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(1)
			Expect(v6SubnetObject).NotTo(BeNil())
			gateway := fmt.Sprintf("fd00:%s::1", v6RandNum)
			v6SubnetObject.Spec.Gateway = &gateway
			v6Subnet = fmt.Sprintf("fd00:%s::/120", v6RandNum)
			v6SubnetObject.Spec.Subnet = v6Subnet
			GinkgoWriter.Printf("Generate v6subnet objects %v and customize gateway %v, subnet %v \n", v6SubnetName, v6SubnetObject.Spec.Gateway, v6SubnetObject.Spec.Subnet)
		}

		// Delete namespaces and delete subnets
		DeferCleanup(func() {
			Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
			}
		})
	})

	DescribeTable("Multiple automatic or manual creation and recycling of ippools.",
		// `originialNum` is the initial number of subnets and pools to be created automatically and manually.
		// `scaleupNum` is the number of subnets and pool extensions created automatically and manually.
		func(originialNum int, ScaleupNum int, OperationType string, timeout time.Duration) {

			// Define some test data
			var (
				deployNameList     []string
				deployOriginialNum int32  = 1
				deployScaleupNum   int32  = 2
				flexibleIPNumber   string = "0"
				IpNum              int    = 1
			)
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

			if OperationType == common.AutomaticallyCreated {
				lock := sync.Mutex{}
				wg := sync.WaitGroup{}
				for i := 1; i <= originialNum; i++ {
					wg.Add(1)
					var deployObject *appsv1.Deployment
					// Constructing a non-existent node so that deployment doesn't actually start
					NonExistentNodeName := "node" + tools.RandomName()
					j := strconv.Itoa(i)

					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						// Waiting time to avoid concurrent renaming
						deployName = "deploy-" + j + tools.RandomName()

						lock.Lock()
						deployNameList = append(deployNameList, deployName)
						lock.Unlock()

						annotationMap := map[string]string{}
						deployObject = common.GenerateExampleDeploymentYaml(deployName, namespace, deployOriginialNum)
						deployObject.Spec.Template.Spec.NodeSelector = map[string]string{corev1.LabelHostname: NonExistentNodeName}
						if frame.Info.IpV4Enabled {
							annotationMap[constant.AnnoSubnetManagerV4] = v4SubnetName
							annotationMap[constant.AnnoSubnetManagerFlexibleIPNumber] = flexibleIPNumber
						}
						if frame.Info.IpV6Enabled {
							annotationMap[constant.AnnoSubnetManagerV6] = v6SubnetName
							annotationMap[constant.AnnoSubnetManagerFlexibleIPNumber] = flexibleIPNumber
						}
						deployObject.Spec.Template.Annotations = annotationMap
						Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

					}()
				}
				wg.Wait()
			}
			GinkgoWriter.Printf("Deployment %v successfully created \n", deployNameList)

			startT1 := time.Now()
			if OperationType == common.AutomaticallyCreated {
				// Make sure the automatically created ippool is all ready
				GinkgoWriter.Println("Make sure the automatically created ippool is all ready")
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				if frame.Info.IpV4Enabled {
					Expect(common.WaitValidatePoolsCreatedAutomatically(ctx, frame, v4SubnetName, originialNum)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.WaitValidatePoolsCreatedAutomatically(ctx, frame, v6SubnetName, originialNum)).NotTo(HaveOccurred())
				}
			}

			if OperationType == common.ManuallyCreated {
				createIPPoolsForSpiderSubnet(v4Subnet, v6Subnet, originialNum, IpNum)
			}
			// Check that all ip's on the subnet have been assigned
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetFreeIPs(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetFreeIPs(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
			}
			endT1 := time.Since(startT1)
			GinkgoWriter.Printf("%v creation of %v pools with a time cost of %v \n", OperationType, originialNum, endT1)

			// Extend the freeIPs of the Subnet
			if frame.Info.IpV4Enabled {
				desiredV4SubnetObject := common.GetSubnetByName(frame, v4SubnetName)
				Expect(desiredV4SubnetObject).NotTo(BeNil())
				desiredV4SubnetObject.Spec.IPs = []string{fmt.Sprintf("10.%s.%s.2-10.%s.%s.%s", v4RandNum1, v4RandNum2, v4RandNum1, v4RandNum2, strconv.Itoa(ScaleupNum+1))}
				Expect(common.PatchSpiderSubnet(frame, desiredV4SubnetObject, v4SubnetObject)).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Extend the freeIPs of the v4Subnet from %v to %v \n", originialNum, ScaleupNum)
			}
			if frame.Info.IpV6Enabled {
				desiredV6SubnetObject := common.GetSubnetByName(frame, v6SubnetName)
				Expect(desiredV6SubnetObject).NotTo(BeNil())
				desiredV6SubnetObject.Spec.IPs = []string{fmt.Sprintf("fd00:%s::2-fd00:%s::%s", v6RandNum, v6RandNum, strconv.FormatInt(int64(ScaleupNum+1), 16))}
				Expect(common.PatchSpiderSubnet(frame, desiredV6SubnetObject, v6SubnetObject)).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Extend the freeIPs of the v6Subnet from %v to %v \n", originialNum, ScaleupNum)
			}

			if OperationType == common.AutomaticallyCreated {
				// Extend the replicas of the Deployment
				wg := sync.WaitGroup{}
				for _, d := range deployNameList {
					wg.Add(1)
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
				GinkgoWriter.Printf("Extend the replicas of the Deployment from %v to %v \n", deployOriginialNum, deployScaleupNum)
			}

			// Check the freeIPs, it should all be allocated already.
			startT2 := time.Now()

			if OperationType == common.ManuallyCreated {
				createIPPoolsForSpiderSubnet(v4Subnet, v6Subnet, ScaleupNum-originialNum, IpNum)
			}

			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
			GinkgoWriter.Println("Check the freeIPs again, it should all be allocated already.")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetFreeIPs(ctx, frame, v4SubnetName, int64(0))).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetFreeIPs(ctx, frame, v6SubnetName, int64(0))).NotTo(HaveOccurred())
			}
			endT2 := time.Since(startT2)
			GinkgoWriter.Printf("%v IP expansion from %v to %v at a time cost of %v \n", OperationType, originialNum, ScaleupNum, endT2)

			if OperationType == common.AutomaticallyCreated {
				// Deleting resources and hopefully reclaiming them automatically if the ippool was created automatically
				wg := sync.WaitGroup{}
				for _, d := range deployNameList {
					wg.Add(1)
					name := d
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						Expect(frame.DeleteDeployment(name, namespace)).NotTo(HaveOccurred())
					}()
				}
				wg.Wait()
			}

			// TODO(tao.yang),  Waiting for the implementation of the capacity reduction function

			// Time spent checking the freeIPs of the subnet and returning to the original state
			startT3 := time.Now()
			if OperationType == common.ManuallyCreated {
				wg := sync.WaitGroup{}
				for _, d := range poolNameList {
					wg.Add(1)
					poolName := d
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						Expect(common.DeleteIPPoolByName(frame, poolName)).NotTo(HaveOccurred())
					}()
				}
				wg.Wait()
			}
			// Check the freeIPs of the subnet back to the original state
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
			GinkgoWriter.Println("Check the freeIPs of the subnet back to the original state")
			if frame.Info.IpV4Enabled {
				Expect(common.WaitValidateSubnetFreeIPs(ctx, frame, v4SubnetName, int64(ScaleupNum))).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.WaitValidateSubnetFreeIPs(ctx, frame, v6SubnetName, int64(ScaleupNum))).NotTo(HaveOccurred())
			}
			endT3 := time.Since(startT3)
			GinkgoWriter.Printf("%v reclaim of %v IP`s with a time cost of %v \n", OperationType, ScaleupNum, endT3)
		},
		PEntry("Multiple automatic creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state",
			Label("I00006"), 30, 60, common.AutomaticallyCreated, time.Minute*2),
		Entry("Multiple manual creation and recycling of ippools, eventually the free IPs in the subnet should be restored to its initial state",
			Label("I00007"), 30, 60, common.ManuallyCreated, time.Minute*2),
	)
})

func createIPPoolsForSpiderSubnet(v4Subnet, v6Subnet string, poolName, ipNum int) []string {
	var poolNameList []string

	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	for i := 1; i <= poolName; i++ {
		wg.Add(1)
		j := i + 1
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj := common.GenerateExampleIpv4poolObject(1)
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
				v4PoolObj.Spec.Subnet = v4Subnet
				v4PoolObj.Spec.IPs = []string{strings.Split(v4Subnet, "0/")[0] + strconv.Itoa(j)}
				Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
				lock.Lock()
				poolNameList = append(poolNameList, v4PoolName)
				lock.Unlock()
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj := common.GenerateExampleIpv6poolObject(1)
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				v6PoolObj.Spec.Subnet = v6Subnet
				v6PoolObj.Spec.IPs = []string{strings.Split(v6Subnet, "/")[0] + strconv.FormatInt(int64(j), 16)}
				Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
				lock.Lock()
				poolNameList = append(poolNameList, v6PoolName)
				lock.Unlock()
			}
		}()
	}
	wg.Wait()
	return poolNameList
}
