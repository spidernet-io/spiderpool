// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reclaim_test

import (
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ip with reclaim ip case", Label("reclaim"), func() {
	var err error
	var podName, namespace string
	var podIPv4, podIPv6 string

	BeforeEach(func() {
		// create namespace
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err = frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

		// pod name
		podName = "pod" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	It("related IP resource recorded in ippool will be reclaimed after the namespace is deleted",
		Label("smoke", "G00001"), func() {
			// create pod
			_, podIPv4, podIPv6 = createPod(podName, namespace, time.Second*30)

			// ippool allocated ip
			var allocatedIPv4s, allocatedIPv6s []string

			// get ippool status.allocated_ips
			// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s

			if podIPv4 != "" {
				// TODO(bingzhesun) here we assume that we have obtained the allocated ips
				allocatedIPv4s = append(allocatedIPv4s, podIPv4)
				GinkgoWriter.Printf("allocatedIPv4s: %v\n", allocatedIPv4s)

				// check if podIP in ippool
				GinkgoWriter.Println("check if podIPv4 in ippool")
				Expect(allocatedIPv4s).To(ContainElement(podIPv4), "assign ipv4 failed")
			}
			if podIPv6 != "" {
				// TODO(bingzhesun) here we assume that we have obtained the allocated ips
				allocatedIPv6s = append(allocatedIPv6s, podIPv6)
				GinkgoWriter.Printf("allocatedIPv6s: %v\n", allocatedIPv6s)

				// check if podIP in ippool
				GinkgoWriter.Println("check if podIPv6 in ippool")
				Expect(allocatedIPv6s).To(ContainElement(podIPv6), "assign ipv6 failed")
			}

			// delete namespace
			GinkgoWriter.Printf("delete namespace %v\n", namespace)
			err = frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
			// TODO(bingzhesun) Here we will use the function waitNamespaceDeleted() to judge

			// get ippool status.allocated_ips after delete namespace
			// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s
			// here we assume that we have obtained the allocated ips

			//  TODO(bingzhesun) check if podIP in ippool
		})

	Context("delete the same-name pod within a different namespace", func() {
		// declare another namespace namespace1
		var namespace1 string

		BeforeEach(func() {
			// create namespace1
			namespace1 = "ns1-" + tools.RandomName()
			GinkgoWriter.Printf("create namespace1 %v \n", namespace1)
			err = frame.CreateNamespace(namespace1)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace1 %v", namespace1)

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete namespace1 %v \n", namespace1)
				err := frame.DeleteNamespace(namespace1)
				Expect(err).NotTo(HaveOccurred(), "failed to delete namespace1 %v", namespace1)
			})
		})

		It("the IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted",
			Label("G00002"), func() {
				namespaces := []string{namespace, namespace1}
				for _, ns := range namespaces {
					// create pod in namespaces
					_, podIPv4, podIPv6 = createPod(podName, ns, time.Second*30)
				}

				// delete pod in namespace1 until finish
				GinkgoWriter.Printf("delete the pod %v in namespace1 %v\n", podName, namespace1)
				ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
				defer cancel1()
				e2 := frame.DeletePodUntilFinish(podName, namespace1, ctx1)
				Expect(e2).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed delete pod %v/%v\n", namespace1, podName)

				// check if pod in namespace is running normally
				GinkgoWriter.Printf("check if pod %v in namespace %v is running normally\n", podName, namespace)
				pod3, e3 := frame.GetPod(podName, namespace)
				Expect(pod3).NotTo(BeNil())
				Expect(e3).NotTo(HaveOccurred())
				if podIPv4 != "" {
					GinkgoWriter.Println("check pod ipv4")
					podIPv4 := common.GetPodIPv4Address(pod3)
					Expect(podIPv4).NotTo(BeNil())
					GinkgoWriter.Printf("pod ipv4: %v\n", podIPv4)
				}
				if podIPv6 != "" {
					GinkgoWriter.Println("check pod ipv6")
					podIPv6 := common.GetPodIPv6Address(pod3)
					Expect(podIPv6).NotTo(BeNil())
					GinkgoWriter.Printf("pod ipv6: %v\n", podIPv6)
				}

				// TODO(bingzhesun) check the same-name pod , its ip in ippool not be reclaimed
			})
	})

	It("the IP should be reclaimed after its pod is deleted , even when CNI binary is gone on the host", Serial,
		Label("smoke", "G00003"), func() {
			// create pod
			GinkgoWriter.Printf("create pod %v/%v \n", namespace, podName)
			pod, _, _ := createPod(podName, namespace, time.Second*30)

			v := &corev1.PodList{
				Items: []corev1.Pod{*pod},
			}

			// check the pod ip information in ippool is correct
			GinkgoWriter.Printf("check the pod %v/%v ip information in ippool is correct\n", namespace, podName)
			ok1, _, _, err1 := common.CheckPodIpRecordInIppool(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, v)
			Expect(ok1).To(BeTrue())
			Expect(err1).NotTo(HaveOccurred())

			// remove cni bin
			GinkgoWriter.Println("remove cni bin")
			command := "mv /opt/cni/bin/multus /opt/cni/bin/multus.backup"
			common.ExecCommandOnKindNode(frame.Info.KindNodeList, command, time.Second*10)
			GinkgoWriter.Println("remove cni bin successfully")

			// delete pod
			GinkgoWriter.Printf("delete pod %v/%v\n", namespace, podName)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			opt := &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64Ptr(0),
			}
			Expect(frame.DeletePodUntilFinish(podName, namespace, ctx, opt)).To(Succeed())
			GinkgoWriter.Printf("delete pod %v/%v successfully\n", namespace, podName)

			// check if the pod ip in ippool reclaimed normally
			GinkgoWriter.Println("check podIP reclaimed")
			Expect(common.WaitIPReclaimedFinish(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, v, time.Minute)).To(Succeed())

			// restore cni bin
			GinkgoWriter.Println("restore cni bin")
			command = "mv /opt/cni/bin/multus.backup /opt/cni/bin/multus"
			common.ExecCommandOnKindNode(frame.Info.KindNodeList, command, time.Second*10)
			GinkgoWriter.Println("restore cni bin successfully")

			// wait nodes ready
			GinkgoWriter.Println("wait cluster node ready")
			ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
			defer cancel1()
			ok3, err3 := frame.WaitClusterNodeReady(ctx1)
			Expect(ok3).To(BeTrue())
			Expect(err3).NotTo(HaveOccurred())
		})

	Context("test the reclaim of dirty IP record in the IPPool", func() {
		var v4poolName, v6poolName string
		var v4poolNameList, v6poolNameList []string
		var v4poolObj, v6poolObj *spiderpool.SpiderIPPool
		var (
			podIPv4Record, podIPv6Record     = new(spiderpool.PoolIPAllocation), new(spiderpool.PoolIPAllocation)
			dirtyIPv4Record, dirtyIPv6Record = new(spiderpool.PoolIPAllocation), new(spiderpool.PoolIPAllocation)
		)
		var dirtyIPv4, dirtyIPv6 string
		var dirtyPodName, dirtyContainerID string

		BeforeEach(func() {
			// generate dirty ip, pod name and dirty containerID
			GinkgoWriter.Println("generate dirty ip")
			randomNum1 := common.GenerateRandomNumber(255)
			randomNum2 := common.GenerateRandomNumber(255)
			dirtyIPv4 = fmt.Sprintf("192.168.%s.%s", randomNum1, randomNum2)
			GinkgoWriter.Printf("dirtyIPv4:%v\n", dirtyIPv4)

			randomNum1 = common.GenerateRandomNumber(9999)
			randomNum2 = common.GenerateRandomNumber(9999)
			dirtyIPv6 = fmt.Sprintf("fe00:%s::%s", randomNum1, randomNum2)
			GinkgoWriter.Printf("dirtyIPv6:%v\n", dirtyIPv6)

			GinkgoWriter.Println("generate dirty pod name")
			dirtyPodName = "dirtyPod-" + tools.RandomName()
			GinkgoWriter.Printf("dirtyPodName:%v\n", dirtyPodName)

			GinkgoWriter.Println("generate dirty containerID")
			dirtyContainerID = common.GenerateString(64, true)
			GinkgoWriter.Printf("dirtyContainerID:%v\n", dirtyContainerID)

			// create ippool
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Println("create ipv4 pool")
				v4poolName, v4poolObj = common.GenerateExampleIpv4poolObject(1)
				v4poolNameList = []string{v4poolName}
				Expect(common.CreateIppool(frame, v4poolObj)).To(Succeed())
				GinkgoWriter.Printf("succeed to create ipv4 pool %v\n", v4poolName)
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Println("create ipv6 pool")
				v6poolName, v6poolObj = common.GenerateExampleIpv6poolObject(1)
				v6poolNameList = []string{v6poolName}
				Expect(common.CreateIppool(frame, v6poolObj)).To(Succeed())
				GinkgoWriter.Printf("succeed to create ipv6 pool %v\n", v6poolName)
			}

			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Println("delete ipv4 pool")
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					Expect(common.DeleteIPPoolUntilFinish(frame, v4poolName, ctx)).To(Succeed())
					GinkgoWriter.Printf("succeed to delete ipv4 pool %v\n", v4poolName)
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Println("delete ipv6 pool")
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					Expect(common.DeleteIPPoolUntilFinish(frame, v6poolName, ctx)).To(Succeed())
					GinkgoWriter.Printf("succeed to delete ipv6 pool %v\n", v6poolName)
				}
			})
		})
		DescribeTable("dirty IP record in the IPPool should be auto clean by Spiderpool", func(dirtyField string) {
			// generate podYaml
			GinkgoWriter.Println("generate pod yaml")
			podYaml := common.GenerateExamplePodYaml(podName, namespace)
			Expect(podYaml).NotTo(BeNil())

			podAnnoIPPoolValue := types.AnnoPodIPPoolValue{}

			if frame.Info.IpV4Enabled {
				podAnnoIPPoolValue.IPv4Pools = []string{v4poolName}
			}
			if frame.Info.IpV6Enabled {
				podAnnoIPPoolValue.IPv6Pools = []string{v6poolName}
			}

			b, err := json.Marshal(podAnnoIPPoolValue)
			Expect(err).NotTo(HaveOccurred(), "failed to marshal podAnnoIPPoolValue")
			Expect(b).NotTo(BeNil())
			podAnnoStr := string(b)
			podYaml.Annotations = map[string]string{
				constant.AnnoPodIPPool: podAnnoStr,
			}
			GinkgoWriter.Printf("succeed to generate podYaml: %v \n", podYaml)

			// create pod
			GinkgoWriter.Printf("create pod %v/%v until ready\n", namespace, podName)
			pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, namespace, time.Minute)
			Expect(pod).NotTo(BeNil(), "create pod failed\n")
			GinkgoWriter.Printf("podIPv4: %v\n", podIPv4)
			GinkgoWriter.Printf("podIPv6: %v\n", podIPv6)

			// check pod ip in ippool
			GinkgoWriter.Printf("check pod %v/%v ip in ippool\n", namespace, podName)
			podList := &corev1.PodList{
				Items: []corev1.Pod{
					*pod,
				},
			}
			allRecorded, _, _, err := common.CheckPodIpRecordInIppool(frame, v4poolNameList, v6poolNameList, podList)
			Expect(allRecorded).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeed to check pod %v/%v ip in ippool\n", namespace, podName)

			// get pod ip record in ippool
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Printf("get pod=%v/%v ip=%v record in ipv4 pool=%v\n", namespace, podName, podIPv4, v4poolName)
				v4poolObj = common.GetIppoolByName(frame, v4poolName)
				*podIPv4Record = v4poolObj.Status.AllocatedIPs[podIPv4]
				GinkgoWriter.Printf("the pod ip record in ipv4 pool is %v\n", *podIPv4Record)
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Printf("get pod=%v/%v ip=%v record in ipv6 pool=%v\n", namespace, podName, podIPv6, v6poolName)
				v6poolObj = common.GetIppoolByName(frame, v6poolName)
				*podIPv6Record = v6poolObj.Status.AllocatedIPs[podIPv6]
				GinkgoWriter.Printf("the pod ip record in ipv6 pool is %v\n", *podIPv6Record)
			}

			GinkgoWriter.Println("add dirty data to ippool")
			if frame.Info.IpV4Enabled {
				dirtyIPv4Record = podIPv4Record

				switch dirtyField {
				case "pod":
					dirtyIPv4Record.Pod = dirtyPodName
					allocatedIPCount := *v4poolObj.Status.AllocatedIPCount
					allocatedIPCount++
					GinkgoWriter.Printf("allocatedIPCount: %v\n", allocatedIPCount)
					v4poolObj.Status.AllocatedIPCount = pointer.Int64Ptr(allocatedIPCount)
				case "containerID":
					dirtyIPv4Record.ContainerID = dirtyContainerID
					allocatedIPCount := *v4poolObj.Status.AllocatedIPCount
					allocatedIPCount++
					GinkgoWriter.Printf("allocatedIPCount: %v\n", allocatedIPCount)
					v4poolObj.Status.AllocatedIPCount = pointer.Int64Ptr(allocatedIPCount)
				default:
					Fail("invalid parameter\n")
				}

				v4poolObj.Status.AllocatedIPs[dirtyIPv4] = *dirtyIPv4Record

				// update ipv4 pool
				GinkgoWriter.Printf("update ippool %v for adding dirty record: %+v \n", v4poolName, *dirtyIPv4Record)
				Expect(frame.UpdateResourceStatus(v4poolObj)).To(Succeed(), "failed to update ipv4 pool %v\n", v4poolName)
				GinkgoWriter.Printf("ipv4 pool %+v\n", v4poolObj)

				// check if dirty data added successfully
				v4poolObj = common.GetIppoolByName(frame, v4poolName)
				record, ok := v4poolObj.Status.AllocatedIPs[dirtyIPv4]
				Expect(ok).To(BeTrue())
				Expect(record).To(Equal(*dirtyIPv4Record))

			}
			if frame.Info.IpV6Enabled {
				dirtyIPv6Record = podIPv6Record

				switch dirtyField {
				case "pod":
					dirtyIPv6Record.Pod = dirtyPodName
					allocatedIPCount := *v6poolObj.Status.AllocatedIPCount
					allocatedIPCount++
					GinkgoWriter.Printf("allocatedIPCount: %v\n", allocatedIPCount)
					v6poolObj.Status.AllocatedIPCount = pointer.Int64Ptr(allocatedIPCount)
				case "containerID":
					dirtyIPv6Record.ContainerID = dirtyContainerID
					allocatedIPCount := *v6poolObj.Status.AllocatedIPCount
					allocatedIPCount++
					GinkgoWriter.Printf("allocatedIPCount: %v\n", allocatedIPCount)
					v6poolObj.Status.AllocatedIPCount = pointer.Int64Ptr(allocatedIPCount)
				default:
					Fail("invalid parameter\n")
				}

				v6poolObj.Status.AllocatedIPs[dirtyIPv6] = *dirtyIPv6Record

				// update ipv6 pool
				GinkgoWriter.Printf("update ippool %v for adding dirty record: %+v \n", v6poolName, *dirtyIPv6Record)
				Expect(frame.UpdateResourceStatus(v6poolObj)).To(Succeed(), "failed to update ipv6 pool %v\n", v6poolName)
				GinkgoWriter.Printf("ipv6 pool %+v\n", v6poolObj)

				// check if dirty data added successfully
				v6poolObj = common.GetIppoolByName(frame, v6poolName)
				record, ok := v6poolObj.Status.AllocatedIPs[dirtyIPv6]
				Expect(ok).To(BeTrue())
				Expect(record).To(Equal(*dirtyIPv6Record))

			}

			// restart spiderpool controller to trigger gc
			GinkgoWriter.Println("restart spiderpool controller")
			spiderpoolControllerPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": "spiderpoolcontroller"})
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderpoolControllerPodList).NotTo(BeNil(), "failed to get spiderpool controller podList\n")
			spiderpoolControllerPodList, err = frame.DeletePodListUntilReady(spiderpoolControllerPodList, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderpoolControllerPodList).NotTo(BeNil(), "failed to get spiderpool controller podList after restart\n")
			GinkgoWriter.Println("succeed to restart spiderpool controller pods")

			// check the real pod ip should be recorded in spiderpool, the dirty ip record should be reclaimed from spiderpool
			GinkgoWriter.Printf("check if the pod %v/%v ip recorded in ippool, check if the dirty ip record reclaimed from ippool\n", namespace, podName)
			if frame.Info.IpV4Enabled {
				// check if dirty data reclaimed successfully
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
			LOOPV4:
				for {
					select {
					default:
						v4poolObj = common.GetIppoolByName(frame, v4poolName)
						_, ok := v4poolObj.Status.AllocatedIPs[dirtyIPv4]
						if !ok {
							GinkgoWriter.Printf("succeed to reclaim the dirty ip %v record from ippool %v\n", dirtyIPv4, v4poolName)
							break LOOPV4
						}
						time.Sleep(time.Second)
					case <-ctx.Done():
						Fail("timeout to wait reclaim the dirty data from ippool\n")
					}
				}
			}
			if frame.Info.IpV6Enabled {
				// check if dirty data reclaimed successfully
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
			LOOPV6:
				for {
					select {
					default:
						v6poolObj = common.GetIppoolByName(frame, v6poolName)
						_, ok := v6poolObj.Status.AllocatedIPs[dirtyIPv6]
						if !ok {
							GinkgoWriter.Printf("succeed to reclaim the dirty ip %v record from ippool %v\n", dirtyIPv6, v6poolName)
							break LOOPV6
						}
						time.Sleep(time.Second)
					case <-ctx.Done():
						Fail("timeout to wait reclaim the dirty data from ippool\n")
					}
				}
			}

			// check pod ip in ippool
			allRecorded, _, _, err = common.CheckPodIpRecordInIppool(frame, v4poolNameList, v6poolNameList, podList)
			Expect(allRecorded).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeed to check pod %v/%v ip in ippool\n", namespace, podName)

			// delete pod
			GinkgoWriter.Printf("delete pod %v/%v\n", namespace, podName)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			Expect(frame.DeletePodUntilFinish(podName, namespace, ctx)).To(Succeed(), "timeout to delete pod %v/%v\n", namespace, podName)
			GinkgoWriter.Printf("succeed to delete pod %v/%v\n", namespace, podName)
		},
			Entry("a dirty IP record (pod name is wrong) in the IPPool should be auto clean by Spiderpool", Serial, Label("G00005"), "pod"),
			Entry("a dirty IP record (containerID is wrong) in the IPPool should be auto clean by Spiderpool", Serial, Label("G00007"), "containerID"),
		)
	})
})

func createPod(podName, namespace string, duration time.Duration) (pod *corev1.Pod, podIPv4, podIPv6 string) {
	// generate podYaml
	podYaml := common.GenerateExamplePodYaml(podName, namespace)
	Expect(podYaml).NotTo(BeNil())
	GinkgoWriter.Printf("podYaml: %v \n", podYaml)

	// create pod
	pod, podIPv4, podIPv6 = common.CreatePodUntilReady(frame, podYaml, podName, namespace, duration)
	Expect(pod).NotTo(BeNil(), "create pod failed")
	GinkgoWriter.Printf("podIPv4: %v\n", podIPv4)
	GinkgoWriter.Printf("podIPv6: %v\n", podIPv6)
	return
}
