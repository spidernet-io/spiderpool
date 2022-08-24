// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package assignip_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test pod", Label("assignip"), func() {
	var err error
	var testName, namespace string
	var dpm *appsv1.Deployment
	var sts *appsv1.StatefulSet
	var ds *appsv1.DaemonSet
	var rs *appsv1.ReplicaSet
	var podlist *corev1.PodList

	BeforeEach(func() {
		// init namespace name and create
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, time.Second*10)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
		// init test pod name
		testName = "pod" + tools.RandomName()
		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	DescribeTable("test pod assign ip", func(annotationKeyName string, annotationLength int) {
		// create pod
		GinkgoWriter.Printf("create pod %v/%v with annotationLength= %v \n", namespace, testName, annotationLength)
		podYaml := common.GenerateExamplePodYaml(testName, namespace)
		podYaml.Annotations = map[string]string{annotationKeyName: common.GenerateString(annotationLength, false)}
		Expect(podYaml).NotTo(BeNil())

		pod, _, _ := common.CreatePodUntilReady(frame, podYaml, testName, namespace, time.Second*30)
		Expect(pod).NotTo(BeNil())
		Expect(pod.Annotations[annotationKeyName]).To(Equal(podYaml.Annotations[annotationKeyName]))
		GinkgoWriter.Printf("create pod %v/%v successfully \n", namespace, testName)

		v := &corev1.PodList{
			Items: []corev1.Pod{*pod},
		}
		ok, _, _, e := common.CheckPodIpRecordInIppool(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, v)
		if e != nil || !ok {
			Fail(fmt.Sprintf("failed to CheckPodIpRecordInIppool, reason=%v", e))
		}

		// try to delete pod
		GinkgoWriter.Printf("delete pod %v/%v \n", namespace, testName)
		err = frame.DeletePod(testName, namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", namespace, testName)
	},
		Entry("assign IP to a pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00001"), "test", 0),
		Entry("succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case",
			Label("E00007"), "test", 100),
	)

	DescribeTable("assign IP to controller/pod for ipv4, ipv6 and dual-stack case",
		func(controllerType string, replicas int32) {
			// try to create controller
			GinkgoWriter.Printf("try to create controller %v: %v/%v \n", controllerType, testName, namespace)
			switch {
			case controllerType == common.DeploymentNameString:
				dpm = common.GenerateExampleDeploymentYaml(testName, namespace, replicas)
				err = frame.CreateDeployment(dpm)
			case controllerType == common.StatefulSetNameString:
				sts = common.GenerateExampleStatefulSetYaml(testName, namespace, replicas)
				err = frame.CreateStatefulSet(sts)
			case controllerType == common.DaemonSetNameString:
				ds = common.GenerateExampleDaemonSetYaml(testName, namespace)
				err = frame.CreateDaemonSet(ds)
			case controllerType == common.ReplicaSetNameString:
				rs = common.GenerateExampleReplicaSetYaml(testName, namespace, replicas)
				err = frame.CreateReplicaSet(rs)
			default:
				Fail("input variable is not valid")
			}
			Expect(err).NotTo(HaveOccurred(), "failed to create controller %v", controllerType)

			// setting timeout
			ctx1, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			GinkgoWriter.Printf("try to wait controller %v: %v/%v \n", controllerType, testName, namespace)
			switch {
			case controllerType == common.DeploymentNameString:
				dpm, err = frame.WaitDeploymentReady(testName, namespace, ctx1)
				Expect(dpm).NotTo(BeNil())
			case controllerType == common.StatefulSetNameString:
				sts, err = frame.WaitStatefulSetReady(testName, namespace, ctx1)
				Expect(sts).NotTo(BeNil())
			case controllerType == common.DaemonSetNameString:
				ds, err = frame.WaitDaemonSetReady(testName, namespace, ctx1)
				Expect(ds).NotTo(BeNil())
			case controllerType == common.ReplicaSetNameString:
				rs, err = frame.WaitReplicaSetReady(testName, namespace, ctx1)
				Expect(rs).NotTo(BeNil())
			}
			Expect(err).NotTo(HaveOccurred(), "time out to wait controller %v ready", controllerType)

			// try to get controller pod list
			switch {
			case controllerType == common.DeploymentNameString:
				podlist, err = frame.GetDeploymentPodList(dpm)
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
			case controllerType == common.StatefulSetNameString:
				podlist, err = frame.GetStatefulSetPodList(sts)
				Expect(int32(len(podlist.Items))).Should(Equal(sts.Status.ReadyReplicas))
			case controllerType == common.DaemonSetNameString:
				podlist, err = frame.GetDaemonSetPodList(ds)
				Expect(int32(len(podlist.Items))).Should(Equal(ds.Status.NumberReady))
			case controllerType == common.ReplicaSetNameString:
				podlist, err = frame.GetReplicaSetPodList(rs)
				Expect(int32(len(podlist.Items))).Should(Equal(rs.Status.ReadyReplicas))
			}
			Expect(err).NotTo(HaveOccurred(), "failed to list controller pod %v", controllerType)

			// check all pods to created by controller，it`s assign ipv4 and ipv6 addresses success
			err = frame.CheckPodListIpReady(podlist)
			Expect(err).NotTo(HaveOccurred(), "failed to check ipv4 or ipv6")

			// try to delete controller
			GinkgoWriter.Printf("try to delete controller %v: %v/%v \n", controllerType, testName, namespace)
			switch {
			case controllerType == common.DeploymentNameString:
				err = frame.DeleteDeployment(testName, namespace)
			case controllerType == common.StatefulSetNameString:
				err = frame.DeleteStatefulSet(testName, namespace)
			case controllerType == common.DaemonSetNameString:
				err = frame.DeleteDaemonSet(testName, namespace)
			case controllerType == common.ReplicaSetNameString:
				err = frame.DeleteReplicaSet(testName, namespace)
			}
			Expect(err).NotTo(HaveOccurred(), "failed to delete controller %v: %v/%v \n", controllerType, testName, namespace)
		},
		Entry("assign IP to deployment/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00002"), common.DeploymentNameString, int32(2)),
		Entry("assign IP to statefulSet/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00003"), common.StatefulSetNameString, int32(2)),
		Entry("assign IP to daemonset/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00004"), common.DaemonSetNameString, int32(0)),
		Entry("assign IP to replicaset/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00006"), common.ReplicaSetNameString, int32(2)),
	)

	Context("fail to run a pod when IP resource of an ippool is exhausted or its IP been set excludeIPs", func() {
		var deployName, v4PoolName, v6PoolName, nic, podAnnoStr string
		var v4PoolNameList, v6PoolNameList []string
		var v4PoolObj, v6PoolObj *spiderpoolv1.SpiderIPPool

		BeforeEach(func() {
			nic = "eth0"

			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(3)
				v4PoolObj.Spec.ExcludeIPs = strings.Split(v4PoolObj.Spec.IPs[0], "-")[:1]
				v4PoolNameList = append(v4PoolNameList, v4PoolName)

				// create ipv4 pool
				createIPPool(v4PoolObj)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(3)
				v6PoolObj.Spec.ExcludeIPs = strings.Split(v6PoolObj.Spec.IPs[0], "-")[:1]
				v6PoolNameList = append(v6PoolNameList, v6PoolName)

				// create ipv6 pool
				createIPPool(v6PoolObj)
			}

			deployName = "deploy" + tools.RandomName()

			// pod annotations
			podAnno := types.AnnoPodIPPoolValue{
				NIC: &nic,
			}
			if frame.Info.IpV4Enabled {
				podAnno.IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podAnno.IPv6Pools = []string{v6PoolName}
			}
			b, e1 := json.Marshal(podAnno)
			Expect(e1).NotTo(HaveOccurred())
			podAnnoStr = string(b)

			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					deleteIPPoolUntilFinish(v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					deleteIPPoolUntilFinish(v6PoolName)
				}
			})
		})

		It(" fail to run a pod when IP resource of an ippool is exhausted and an IP who is set in excludeIPs field of ippool, should not be assigned to a pod",
			Label("E00008", "S00002"), func() {
				// generate deployment yaml
				GinkgoWriter.Println("generate deploy yaml")
				deployYaml := common.GenerateExampleDeploymentYaml(deployName, namespace, int32(2))

				deployYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podAnnoStr}

				// create deployment until ready
				deploy, e1 := frame.CreateDeploymentUntilReady(deployYaml, time.Minute)
				Expect(e1).NotTo(HaveOccurred())

				// get podList
				GinkgoWriter.Println("get podList")
				podList, e2 := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
				Expect(e2).NotTo(HaveOccurred())
				GinkgoWriter.Printf("podList Num:%v\n", len(podList.Items))

				// check podIP record in IPPool
				GinkgoWriter.Println("check podIP record in ippool")
				ok, _, _, e3 := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
				Expect(e3).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())

				// scale deployment
				GinkgoWriter.Println("scale deployment to exhaust ip resource")
				_, e4 := frame.ScaleDeployment(deploy, int32(3))
				Expect(e4).NotTo(HaveOccurred())

				// wait expected number of pods
				GinkgoWriter.Println("wait expected number of pods")
				ctx5, cancel5 := context.WithTimeout(context.Background(), time.Minute)
				defer cancel5()
				var podList1 *corev1.PodList
				for {
					select {
					case <-ctx5.Done():
						Fail("time out to wait expected number of pods")
					default:
						podList1, err = frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
						Expect(err).NotTo(HaveOccurred())
						if len(podList1.Items) == 3 {
							goto WAITOK
						}
						time.Sleep(time.Second)
					}
				}
			WAITOK:

				// check event message is expected
				GinkgoWriter.Println("check event message is expected")
				pods := common.GetAdditionalPods(podList, podList1)
				Expect(len(pods)).To(Equal(1))

				newPod := &pods[0]
				ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
				defer cancel1()
				Expect(frame.WaitExceptEventOccurred(ctx1, common.PodEventKind, newPod.Name, newPod.Namespace, common.GetIpamAllocationFailed)).To(Succeed())
				GinkgoWriter.Printf("succeeded to detect the message expected: %v\n", common.GetIpamAllocationFailed)

				// delete deployment util finish
				GinkgoWriter.Printf("delete deployment %v/%v until finish\n", namespace, deployName)
				Expect(frame.DeleteDeploymentUntilFinish(deployName, namespace, time.Minute)).To(Succeed())

				// check ip release successfully
				GinkgoWriter.Println("check ip is release successfully")
				_, ok7, _, e7 := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
				Expect(e7).NotTo(HaveOccurred())
				Expect(ok7).To(BeTrue())
				GinkgoWriter.Println("succeeded to release ip")
			})
	})
})

func deleteIPPoolUntilFinish(poolName string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	GinkgoWriter.Printf("delete ippool %v\n", poolName)
	Expect(common.DeleteIPPoolUntilFinish(frame, poolName, ctx)).To(Succeed())
}

func createIPPool(IPPoolObj *spiderpoolv1.SpiderIPPool) {
	GinkgoWriter.Printf("create ippool %v\n", IPPoolObj.Name)
	Expect(common.CreateIppool(frame, IPPoolObj)).To(Succeed())
	GinkgoWriter.Printf("create ippool %v succceed\n", IPPoolObj.Name)
}
