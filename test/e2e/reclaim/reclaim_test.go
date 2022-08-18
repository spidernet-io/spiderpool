// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reclaim_test

import (
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		err = frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, time.Second*10)
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
			err = frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace1, time.Second*10)
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

	It("the IP can be reclaimed after its deployment, statefulSet, daemonSet, replicaSet, or job is deleted, even when CNI binary is gone on the host", Label("G00003", "G00004"), Serial, func() {
		var podList *corev1.PodList

		deployName := "deploy-" + tools.RandomName()
		stsName := "sts-" + tools.RandomName()
		dsName := "ds-" + tools.RandomName()
		rsName := "rs-" + tools.RandomName()
		jobName := "job-" + tools.RandomName()

		// create resources
		// create pod
		GinkgoWriter.Printf("try to create %v/%v \n", namespace, podName)
		createPod(podName, namespace, time.Minute)

		// create deployment
		// generate example deployment yaml
		GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, deployName)
		deployYaml := common.GenerateExampleDeploymentYaml(deployName, namespace, int32(1))
		Expect(deployYaml).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, deployName)

		// create deployment
		GinkgoWriter.Printf("create deployment %v/%v \n", namespace, deployName)
		deployObj, err := frame.CreateDeploymentUntilReady(deployYaml, time.Minute)
		Expect(err).NotTo(HaveOccurred())
		Expect(deployObj).NotTo(BeNil())

		// create statefulSet
		// generate example statefulSet yaml
		GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, stsName)
		stsYaml := common.GenerateExampleStatefulSetYaml(stsName, namespace, int32(1))
		Expect(stsYaml).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, stsName)

		// create statefulSet
		GinkgoWriter.Printf("create %v/%v \n", namespace, stsName)
		Expect(frame.CreateStatefulSet(stsYaml)).To(Succeed(), "failed to create %v/%v \n", namespace, stsName)
		ctxSts, cancelSts := context.WithTimeout(context.Background(), time.Minute)
		defer cancelSts()
		statefulSet, err := frame.WaitStatefulSetReady(stsName, namespace, ctxSts)
		Expect(err).NotTo(HaveOccurred(), "failed to wait for statefulSet ready")
		Expect(statefulSet).NotTo(BeNil())

		// create daemonSet
		// generate example daemonSet yaml
		GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, dsName)
		dsYaml := common.GenerateExampleDaemonSetYaml(dsName, namespace)
		Expect(dsYaml).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, dsName)

		// create daemonSet
		GinkgoWriter.Printf("create %v/%v \n", namespace, dsName)
		Expect(frame.CreateDaemonSet(dsYaml)).To(Succeed(), "failed to create %v/%v \n", namespace, dsName)
		ctxDs, cancelDs := context.WithTimeout(context.Background(), time.Minute)
		defer cancelDs()
		daemonSet, err := frame.WaitDaemonSetReady(dsName, namespace, ctxDs)
		Expect(err).NotTo(HaveOccurred(), "failed to wait for daemonSet ready")
		Expect(daemonSet).NotTo(BeNil())

		// create replicaSet
		// generate example replicaSet yaml
		GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, rsName)
		rsYaml := common.GenerateExampleReplicaSetYaml(rsName, namespace, int32(1))
		Expect(rsYaml).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, rsName)

		// create replicaSet
		GinkgoWriter.Printf("create %v/%v \n", namespace, rsName)
		Expect(frame.CreateReplicaSet(rsYaml)).To(Succeed(), "failed to create %v/%v \n", namespace, rsName)
		ctxRs, cancelRs := context.WithTimeout(context.Background(), time.Minute)
		defer cancelRs()
		replicaSet, err := frame.WaitReplicaSetReady(rsName, namespace, ctxRs)
		Expect(err).NotTo(HaveOccurred(), "failed to wait for replicaSet ready")
		Expect(replicaSet).NotTo(BeNil())

		// create job
		// generate example job yaml
		GinkgoWriter.Printf("generate example %v/%v yaml\n", namespace, jobName)
		jobYaml := common.GenerateExampleJobYaml(common.JobTypeRunningForever, jobName, namespace, pointer.Int32Ptr(1))
		Expect(jobYaml).NotTo(BeNil(), "failed to generate example %v/%v yaml\n", namespace, jobName)

		// create resource
		GinkgoWriter.Printf("create %v/%v \n", namespace, jobName)
		Expect(frame.CreateJob(jobYaml)).To(Succeed(), "failed to create %v/%v \n", namespace, jobName)

		// wait all job pod running
		ctxJob, cancelJob := context.WithTimeout(context.Background(), time.Minute)
		defer cancelJob()
	LOOP:
		for {
			select {
			case <-ctxJob.Done():
				Fail("time out to wait all job pod running\n")
			default:
				job, err := frame.GetJob(jobName, namespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(job).NotTo(BeNil())
				if *job.Spec.Parallelism == job.Status.Active {
					podList, err := frame.GetPodListByLabel(job.Spec.Template.Labels)
					Expect(err).NotTo(HaveOccurred())
					Expect(podList).NotTo(BeNil())
					for _, pod := range podList.Items {
						for _, c := range pod.Status.Conditions {
							if c.Type == corev1.PodReady && c.Status != corev1.ConditionTrue {
								continue LOOP
							}
						}
					}
					break LOOP
				}
				time.Sleep(time.Second)
			}
		}

		// get podList
		podList, err = frame.GetPodList(client.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred(), "failed to get podList,error: %v\n", err)
		Expect(podList).NotTo(BeNil())

		// check the resource ip information in ippool is correct
		GinkgoWriter.Printf("check the ip information of resources in the namespace %v in ippool is correct\n", namespace)
		ok, _, _, err := common.CheckPodIpRecordInIppool(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, podList)
		Expect(err).NotTo(HaveOccurred(), "failed to check ip recorded in ippool, err: %v\n", err)
		Expect(ok).To(BeTrue())

		// remove cni bin
		GinkgoWriter.Println("remove cni bin")
		command := "mv /opt/cni/bin/multus /opt/cni/bin/multus.backup"
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()
		err = common.ExecCommandOnKindNode(ctx, frame.Info.KindNodeList, command)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Println("remove cni bin successfully")

		// delete resource
		opt := &client.DeleteOptions{
			GracePeriodSeconds: pointer.Int64Ptr(0),
		}
		// delete pod
		GinkgoWriter.Printf("delete pod %v/%v\n", namespace, podName)
		ctxPod, cancelPod := context.WithTimeout(context.Background(), time.Minute)
		defer cancelPod()
		Expect(frame.DeletePodUntilFinish(podName, namespace, ctxPod, opt)).To(Succeed(), "timeout to delete pod %v/%v\n", namespace, podName)

		// delete deployment
		GinkgoWriter.Printf("delete deployment %v/%v\n", namespace, deployName)
		Expect(frame.DeleteDeployment(deployName, namespace, opt)).To(Succeed(), "failed to delete deployment %v/%v\n", namespace, deployName)

		// delete statefulSet
		GinkgoWriter.Printf("delete statefulSet %v/%v\n", namespace, stsName)
		Expect(frame.DeleteStatefulSet(stsName, namespace, opt)).To(Succeed(), "failed to delete statefulSet %v/%v\n", namespace, stsName)

		// delete daemonSet
		GinkgoWriter.Printf("delete daemonSet %v/%v\n", namespace, dsName)
		Expect(frame.DeleteDaemonSet(dsName, namespace, opt)).To(Succeed(), "failed to delete daemonSet %v/%v\n", namespace, dsName)

		// delete replicaset
		GinkgoWriter.Printf("delete replicaset %v/%v\n", namespace, rsName)
		Expect(frame.DeleteReplicaSet(rsName, namespace, opt)).To(Succeed(), "failed to delete replicaset %v/%v\n", namespace, rsName)

		// delete job
		GinkgoWriter.Printf("delete job %v/%v\n", namespace, jobName)
		Expect(frame.DeleteJob(jobName, namespace, opt)).To(Succeed(), "failed to delete job %v/%v\n", namespace, jobName)

		// avoid that "GracePeriodSeconds" of workload does not take effect
		podList, err = frame.GetPodList(client.InNamespace(namespace))
		Expect(err).NotTo(HaveOccurred(), "failed to get podList, error: %v\n", err)
		Expect(frame.DeletePodList(podList, opt)).To(Succeed(), "failed to delete podList\n")

		// check if the ip in ippool reclaimed normally
		GinkgoWriter.Println("check IP reclaimed")
		Expect(common.WaitIPReclaimedFinish(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, podList, time.Minute*2)).To(Succeed())

		// restore cni bin
		GinkgoWriter.Println("restore cni bin")
		command = "mv /opt/cni/bin/multus.backup /opt/cni/bin/multus"
		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel2()
		err = common.ExecCommandOnKindNode(ctx2, frame.Info.KindNodeList, command)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Println("restore cni bin successfully")

		// wait nodes ready
		GinkgoWriter.Println("wait cluster node ready")
		ctx3, cancel3 := context.WithTimeout(context.Background(), time.Minute)
		defer cancel3()
		ok, err = frame.WaitClusterNodeReady(ctx3)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
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
			spiderpoolControllerPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": "spiderpool-controller"})
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderpoolControllerPodList).NotTo(BeNil(), "failed to get spiderpool controller podList\n")
			Expect(spiderpoolControllerPodList.Items).NotTo(BeEmpty(), "failed to get spiderpool controller podList\n")
			spiderpoolControllerPodList, err = frame.DeletePodListUntilReady(spiderpoolControllerPodList, time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderpoolControllerPodList).NotTo(BeNil(), "failed to get spiderpool controller podList after restart\n")
			Expect(spiderpoolControllerPodList.Items).NotTo(HaveLen(0), "failed to get spiderpool controller podList\n")
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
