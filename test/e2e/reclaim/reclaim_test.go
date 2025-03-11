// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reclaim_test

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"golang.org/x/net/context"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubectl/pkg/util/podutils"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/utils/retry"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("test ip with reclaim ip case", Label("reclaim"), func() {
	var err error
	var podName, namespace string
	var globalDefaultV4IPPoolList, globalDefaultV6IPPoolList []string

	BeforeEach(func() {
		// Init test info and create namespace
		podName = "pod" + tools.RandomName()
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err = frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

		if frame.Info.IpV4Enabled {
			globalDefaultV4IPPoolList = nil
			globalDefaultV4IPPoolList = append(globalDefaultV4IPPoolList, common.SpiderPoolIPv4PoolDefault)
		}
		if frame.Info.IpV6Enabled {
			globalDefaultV6IPPoolList = nil
			globalDefaultV6IPPoolList = append(globalDefaultV6IPPoolList, common.SpiderPoolIPv6PoolDefault)
		}

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	DescribeTable("check ip release after job finished", func(behavior common.JobBehave) {
		var wg sync.WaitGroup
		var jdName string = "jd" + tools.RandomName()
		var jobHoldDuration = "sleep 5;"

		// Generate job yaml with different behaviour and create it
		GinkgoWriter.Printf("try to create Job %v/%v with job behavior is %v \n", namespace, jdName, behavior)
		jd := common.GenerateExampleJobYaml(behavior, jdName, namespace, ptr.To(int32(1)))
		podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)
		jd.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
		switch behavior {
		case common.JobTypeFail:
			jd.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", jobHoldDuration + "exit 1"}
		case common.JobTypeFinish:
			jd.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", jobHoldDuration + " exit 0"}
		}
		Expect(frame.CreateJob(jd)).NotTo(HaveOccurred(), "Failed to create job %v/%v \n", namespace, jdName)

		// Confirm that the `failed` and `succeeded` job have been assigned an IP address before the job finish
		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()
		wg.Add(1)
		go func() {
			defer GinkgoRecover()
		OUTER:
			for {
				select {
				case <-ctx.Done():
					Fail("IPs allocated during their validity period are not recorded in the ippool.")
				default:
					podList, err := frame.GetJobPodList(jd)
					Expect(err).NotTo(HaveOccurred())
					if len(podList.Items) == 0 {
						continue OUTER
					}
					GinkgoWriter.Printf("Confirm that the job has been assigned to the IP address \n")
					ok, _, _, err := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList)
					if !ok || err != nil {
						continue OUTER
					}
				}
				break OUTER
			}
			wg.Done()
		}()

		// Waiting for different behaviour job to be finished
		GinkgoWriter.Printf("Waiting for job to be finished and behaviour is %v \n", behavior)
		ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel1()
		jb, ok1, e := frame.WaitJobFinished(jdName, namespace, ctx1)
		Expect(e).NotTo(HaveOccurred())
		Expect(jb).NotTo(BeNil())
		wg.Wait()

		switch behavior {
		case common.JobTypeFail:
			Expect(ok1).To(BeFalse())
		case common.JobTypeFinish:
			Expect(ok1).To(BeTrue())
		default:
			Fail("input error")
		}
		GinkgoWriter.Printf("job %v/%v is finished, job conditions is : %v \n", namespace, jb, jb.Status.Conditions)

		// The IP should be reclaimed for the job pod finished with success or failure Status
		GinkgoWriter.Printf("Get a list of pods when the job is done \n")
		podlist, err := frame.GetJobPodList(jd)
		Expect(err).NotTo(HaveOccurred())
		Expect(podlist.Items).NotTo(HaveLen(0))

		GinkgoWriter.Println("The IP should be reclaimed for the job pod finished with success or failure Status")
		Expect(common.WaitIPReclaimedFinish(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podlist, time.Minute*2)).To(Succeed())

		GinkgoWriter.Printf("delete job: %v \n", jdName)
		Expect(frame.DeleteJob(jdName, namespace)).NotTo(HaveOccurred(), "failed to delete job: %v \n", jdName)
	},
		Entry("check ip release when job is failed", Label("G00006"), common.JobTypeFail),
		Entry("check ip release when job is succeeded", Label("G00006"), common.JobTypeFinish),
	)

	It(`G00002:the IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted;
		G00001:related IP resource recorded in ippool will be reclaimed after the namespace is deleted`, Label("G00002", "G00001", "smoke"), func() {
		var podList *corev1.PodList
		var namespace1 string = "ns1-" + tools.RandomName()

		// Create a namespace again and name it namespace1
		GinkgoWriter.Printf("create namespace1 %v \n", namespace1)
		err = frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace1, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace1 %v", namespace1)

		namespaces := []string{namespace, namespace1}
		for _, ns := range namespaces {
			// Create pods with the same name in different namespaces
			podYaml := common.GenerateExamplePodYaml(podName, ns)
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)
			podYaml.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(podYaml).NotTo(BeNil())
			pod, _, _ := common.CreatePodUntilReady(frame, podYaml, podName, ns, common.PodStartTimeout)
			Expect(pod).NotTo(BeNil(), "Failed to create Pod")
			// Construct a podlist for another namespace（namepace1）
			if ns == namespace1 {
				podList = &corev1.PodList{
					Items: []corev1.Pod{*pod},
				}
			}
		}

		// Delete pods from namespace until complete
		GinkgoWriter.Printf("Delete Pods %v/%v from namespace %v until complete \n", podName, namespace)
		ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
		defer cancel()
		e2 := frame.DeletePodUntilFinish(podName, namespace, ctx)
		Expect(e2).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Successful deletion of pods %v/%v \n", namespace, podName)

		// Another Pod in namespace1 with the same name has a normal status and IP
		Expect(frame.CheckPodListIpReady(podList)).NotTo(HaveOccurred())
		ok, _, _, err := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList)
		Expect(err).NotTo(HaveOccurred(), "error: %v\n", err)
		Expect(ok).To(BeTrue())

		By("G00001: related IP resource recorded in ippool will be reclaimed after the namespace is deleted")
		// Try to delete namespace1
		err = frame.DeleteNamespace(namespace1)
		Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace1)
		GinkgoWriter.Printf("Successfully deleted namespace %v \n", namespace1)

		// Check that the Pod IPs in the IPPool are reclaimed properly after deleting the namespace（namespace1）
		Expect(common.WaitIPReclaimedFinish(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList, common.IPReclaimTimeout)).To(Succeed())
	})

	It("the IP can be reclaimed after its deployment, statefulSet, daemonSet, replicaSet, or job is deleted, even when CNI binary is gone on the host",
		// E00001: Assign IP to a pod for ipv4, ipv6 and dual-stack case
		// E00002: Assign IP to deployment/pod for ipv4, ipv6 and dual-stack case
		// E00003: Assign IP to statefulSet/pod for ipv4, ipv6 and dual-stack case
		// E00004: Assign IP to daemonSet/pod for ipv4, ipv6 and dual-stack case
		// E00005: Assign IP to job/pod for ipv4, ipv6 and dual-stack case
		// E00006: Assign IP to replicaset/pod for ipv4, ipv6 and dual-stack case
		Label("G00003", "G00004", "E00001", "E00002", "E00003", "E00004", "E00005", "E00006", "smoke"), Serial, func() {
			var podList *corev1.PodList
			var (
				deployName     string = "deploy-" + tools.RandomName()
				depReplicasNum int32  = 1
				stsName        string = "sts-" + tools.RandomName()
				stsReplicasNum int32  = 1
				dsName         string = "ds-" + tools.RandomName()
				rsName         string = "rs-" + tools.RandomName()
				rsReplicasNum  int32  = 1
				jobName        string = "job-" + tools.RandomName()
				jobNum         int32  = *ptr.To(int32(1))
			)

			// Create different controller resources
			// Generate example podYaml and create Pod
			podYaml := common.GenerateExamplePodYaml(podName, namespace)
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)
			podYaml.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(podYaml).NotTo(BeNil())
			GinkgoWriter.Printf("try to create Pod %v/%v \n", namespace, podName)
			Expect(frame.CreatePod(podYaml)).NotTo(HaveOccurred(), "failed to create Pod %v/%v \n", namespace, podName)

			// Generate example StatefulSet yaml and create StatefulSet
			GinkgoWriter.Printf("Generate example StatefulSet %v/%v yaml \n", namespace, stsName)
			stsYaml := common.GenerateExampleStatefulSetYaml(stsName, namespace, stsReplicasNum)
			stsYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(stsYaml).NotTo(BeNil(), "failed to generate example %v/%v yaml \n", namespace, stsName)
			GinkgoWriter.Printf("Try to create StatefulSet %v/%v \n", namespace, stsName)
			Expect(frame.CreateStatefulSet(stsYaml)).To(Succeed(), "failed to create StatefulSet %v/%v \n", namespace, stsName)

			// Generate example daemonSet yaml and create daemonSet
			GinkgoWriter.Printf("Generate example daemonSet %v/%v yaml\n", namespace, dsName)
			dsYaml := common.GenerateExampleDaemonSetYaml(dsName, namespace)
			dsYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(dsYaml).NotTo(BeNil(), "failed to generate example daemonSet %v/%v yaml\n", namespace, dsName)
			GinkgoWriter.Printf("Try to create daemonSet %v/%v \n", namespace, dsName)
			Expect(frame.CreateDaemonSet(dsYaml)).To(Succeed(), "failed to create daemonSet %v/%v \n", namespace, dsName)

			// Generate example replicaSet yaml and create replicaSet
			GinkgoWriter.Printf("Generate example replicaSet %v/%v yaml \n", namespace, rsName)
			rsYaml := common.GenerateExampleReplicaSetYaml(rsName, namespace, rsReplicasNum)
			rsYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(rsYaml).NotTo(BeNil(), "failed to generate replicaSet example %v/%v yaml \n", namespace, rsName)
			GinkgoWriter.Printf("Try to create replicaSet %v/%v \n", namespace, rsName)
			Expect(frame.CreateReplicaSet(rsYaml)).To(Succeed(), "failed to create replicaSet %v/%v \n", namespace, rsName)

			// Generate example Deployment yaml and create Deployment
			GinkgoWriter.Printf("Generate example deployment %v/%v yaml \n", namespace, deployName)
			deployYaml := common.GenerateExampleDeploymentYaml(deployName, namespace, depReplicasNum)
			deployYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(deployYaml).NotTo(BeNil(), "failed to generate deployment example %v/%v yaml \n", namespace, deployName)
			GinkgoWriter.Printf("Try to create deployment %v/%v \n", namespace, deployName)
			Expect(frame.CreateDeployment(deployYaml)).NotTo(HaveOccurred())

			// Generate example job yaml and create Job resource
			GinkgoWriter.Printf("Generate example job %v/%v yaml\n", namespace, jobName)
			jobYaml := common.GenerateExampleJobYaml(common.JobTypeRunningForever, jobName, namespace, &jobNum)
			jobYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			Expect(jobYaml).NotTo(BeNil(), "failed to generate job example %v/%v yaml \n", namespace, jobName)
			GinkgoWriter.Printf("Try to create job %v/%v \n", namespace, jobName)
			Expect(frame.CreateJob(jobYaml)).To(Succeed(), "failed to create job %v/%v \n", namespace, jobName)

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
		LOOP:
			for {
				select {
				case <-ctx.Done():
					Fail("time out to wait all controller pod running \n")
				default:
					// Get all the pods created under namespace and check if the status is running
					podList, err = frame.GetPodList(client.InNamespace(namespace))
					Expect(err).NotTo(HaveOccurred(), "failed to get podList, error: %v \n", err)
					Expect(podList.Items).NotTo(HaveLen(0))
					// TODO(tao.yang), Inability to accurately sense the cause of failure
					isOk := frame.CheckPodListRunning(podList)
					if !isOk {
						time.Sleep(common.ForcedWaitingTime)
						continue LOOP
					}
					break LOOP
				}
			}

			// Please note that the following checkpoints for cases are included here
			// E00001、E00002、E00003、E00004、E00005、E00006
			// Check the resource ip information in ippool is correct
			GinkgoWriter.Printf("check the ip information of resources in the nippool is correct \n", namespace)
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList)
			Expect(err).NotTo(HaveOccurred(), "failed to check ip recorded in ippool, err: %v\n", err)
			Expect(ok).To(BeTrue())

			// remove cni bin
			GinkgoWriter.Println("remove cni bin")
			command := "mv /opt/cni/bin/multus /opt/cni/bin/multus.backup"
			ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			err = common.ExecCommandOnKindNode(ctx, frame.Info.KindNodeList, command)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("remove cni bin successfully")

			By("G00004: The IP should be reclaimed when deleting the pod with 0 second of grace period")
			opt := &client.DeleteOptions{
				GracePeriodSeconds: ptr.To(int64(0)),
			}
			// Delete resources with 0 second of grace period
			GinkgoWriter.Printf("delete pod %v/%v\n", namespace, podName)
			Expect(frame.DeletePod(podName, namespace, opt)).NotTo(HaveOccurred())

			GinkgoWriter.Printf("delete deployment %v/%v\n", namespace, deployName)
			Expect(frame.DeleteDeployment(deployName, namespace, opt)).To(Succeed())

			GinkgoWriter.Printf("delete statefulSet %v/%v\n", namespace, stsName)
			Expect(frame.DeleteStatefulSet(stsName, namespace, opt)).To(Succeed())

			GinkgoWriter.Printf("delete daemonSet %v/%v\n", namespace, dsName)
			Expect(frame.DeleteDaemonSet(dsName, namespace, opt)).To(Succeed())

			GinkgoWriter.Printf("delete replicaset %v/%v\n", namespace, rsName)
			Expect(frame.DeleteReplicaSet(rsName, namespace, opt)).To(Succeed())

			GinkgoWriter.Printf("delete job %v/%v\n", namespace, jobName)
			Expect(frame.DeleteJob(jobName, namespace, opt)).To(Succeed())

			// avoid that "GracePeriodSeconds" of workload does not take effect
			podList, err = frame.GetPodList(client.InNamespace(namespace))
			Expect(err).NotTo(HaveOccurred(), "failed to get podList, error: %v\n", err)
			Expect(frame.DeletePodList(podList, opt)).To(Succeed(), "failed to delete podList\n")

			// Check that the IP in the IPPool has been reclaimed correctly.
			Expect(common.WaitIPReclaimedFinish(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList, common.IPReclaimTimeout)).To(Succeed())
			GinkgoWriter.Println("Delete resource with 0 second grace period where the IP of the resource is correctly reclaimed")

			// restore cni bin
			GinkgoWriter.Println("restore cni bin")
			command = "mv /opt/cni/bin/multus.backup /opt/cni/bin/multus"
			ctx2, cancel2 := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel2()
			err = common.ExecCommandOnKindNode(ctx2, frame.Info.KindNodeList, command)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Println("restore cni bin successfully")

			// cni wait for node ready after recovery
			GinkgoWriter.Println("wait cluster node ready")
			ctx3, cancel3 := context.WithTimeout(context.Background(), common.NodeReadyTimeout)
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
		var dirtyPodName, dirtyContainerID, v4SubnetName, v6SubnetName string
		var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet

		BeforeEach(func() {
			if frame.Info.SpiderSubnetEnabled {
				Eventually(func() error {
					if frame.Info.IpV4Enabled {
						v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 1)
						err := common.CreateSubnet(frame, v4SubnetObject)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", v4SubnetName, err)
							return err
						}
					}
					if frame.Info.IpV6Enabled {
						v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 1)
						err := common.CreateSubnet(frame, v6SubnetObject)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", v6SubnetName, err)
							return err
						}
					}
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			}

			// generate dirty ip, pod name and dirty containerID
			dirtyIPv4 = common.GenerateRandomIPV4()
			GinkgoWriter.Printf("generate dirty IPv4 :%v \n", dirtyIPv4)

			dirtyIPv6 = common.GenerateRandomIPV6()
			GinkgoWriter.Printf("generate dirty IPv6:%v\n", dirtyIPv6)

			dirtyPodName = "dirtyPod-" + tools.RandomName()
			GinkgoWriter.Printf("generate dirty Pod name: %v \n", dirtyPodName)

			dirtyContainerID = common.GenerateString(64, true)
			GinkgoWriter.Printf("generate dirty containerID: %v\n", dirtyContainerID)

			// Create IPv4Pool and IPV6Pool
			if frame.Info.IpV4Enabled {
				Eventually(func() error {
					v4poolName, v4poolObj = common.GenerateExampleIpv4poolObject(1)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4poolObj, 1)
					} else {
						err = common.CreateIppool(frame, v4poolObj)
					}
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4poolName, err)
						return err
					}
					v4poolNameList = []string{v4poolName}
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			}
			if frame.Info.IpV6Enabled {
				Eventually(func() error {
					v6poolName, v6poolObj = common.GenerateExampleIpv6poolObject(1)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6poolObj, 1)
					} else {
						err = common.CreateIppool(frame, v6poolObj)
					}
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6poolName, err)
						return err
					}
					v6poolNameList = []string{v6poolName}
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			}
			DeferCleanup(func() {
				GinkgoWriter.Printf("Try to delete IPv4 %v, IPv6 %v IPPool \n", v4poolName, v6poolName)
				if frame.Info.IpV4Enabled {
					err := common.DeleteIPPoolByName(frame, v4poolName)
					Expect(err).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					err := common.DeleteIPPoolByName(frame, v6poolName)
					Expect(err).NotTo(HaveOccurred())
				}
				if frame.Info.SpiderSubnetEnabled {
					if frame.Info.IpV4Enabled {
						Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
					}
					if frame.Info.IpV6Enabled {
						Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
					}
				}
			})
		})
		DescribeTable("dirty IP record in the IPPool should be auto clean by Spiderpool", func() {
			// Generate IPPool annotation string
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4poolNameList, v6poolNameList)

			// generate podYaml and create pod
			podYaml := common.GenerateExamplePodYaml(podName, namespace)
			Expect(podYaml).NotTo(BeNil())
			podYaml.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			GinkgoWriter.Printf("create pod %v/%v \n", namespace, podName)
			pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, namespace, common.PodStartTimeout)
			Expect(pod).NotTo(BeNil())
			GinkgoWriter.Printf("podIPv4: %v; podIPv6 %v \n", podIPv4, podIPv6)

			// check pod ip in ippool
			GinkgoWriter.Printf("check pod %v/%v ip in ippool\n", namespace, podName)
			podList := &corev1.PodList{Items: []corev1.Pod{*pod}}
			allRecorded, _, _, err := common.CheckPodIpRecordInIppool(frame, v4poolNameList, v6poolNameList, podList)
			Expect(allRecorded).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			dirtyIPRecordList := []string{"Pod", "ContainerID"}

			for _, v := range dirtyIPRecordList {
				err = retry.RetryOnConflictWithContext(context.Background(), retry.DefaultBackoff, func(ctx context.Context) error {
					// get pod ip record in ippool
					if frame.Info.IpV4Enabled {
						GinkgoWriter.Printf("get pod=%v/%v ip=%v record in ipv4 pool=%v\n", namespace, podName, podIPv4, v4poolName)
						v4poolObj, err = common.GetIppoolByName(frame, v4poolName)
						Expect(err).NotTo(HaveOccurred())
						allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(v4poolObj.Status.AllocatedIPs)
						Expect(err).NotTo(HaveOccurred())
						*podIPv4Record = allocatedRecords[podIPv4]
						GinkgoWriter.Printf("the pod ip record in ipv4 pool is %v\n", *podIPv4Record)
					}
					if frame.Info.IpV6Enabled {
						GinkgoWriter.Printf("get pod=%v/%v ip=%v record in ipv6 pool=%v\n", namespace, podName, podIPv6, v6poolName)
						v6poolObj, err = common.GetIppoolByName(frame, v6poolName)
						Expect(err).NotTo(HaveOccurred())
						allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(v6poolObj.Status.AllocatedIPs)
						Expect(err).NotTo(HaveOccurred())
						*podIPv6Record = allocatedRecords[podIPv6]
						GinkgoWriter.Printf("the pod ip record in ipv6 pool is %v\n", *podIPv6Record)
					}

					GinkgoWriter.Println("add dirty data to ippool")
					if frame.Info.IpV4Enabled {
						dirtyIPv4Record = podIPv4Record
						if v == "Pod" {
							dirtyIPv4Record.NamespacedName = dirtyPodName
						} else {
							dirtyIPv4Record.PodUID = dirtyContainerID
						}
						allocatedIPCount := *v4poolObj.Status.AllocatedIPCount
						allocatedIPCount++
						GinkgoWriter.Printf("allocatedIPCount: %v\n", allocatedIPCount)
						v4poolObj.Status.AllocatedIPCount = ptr.To(allocatedIPCount)

						allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(v4poolObj.Status.AllocatedIPs)
						Expect(err).NotTo(HaveOccurred())
						allocatedRecords[dirtyIPv4] = *dirtyIPv4Record
						// Update dirty data to IPv4 IPPool.Status.AllocatedIPs
						GinkgoWriter.Printf("update v4 IPPool %v for adding dirty record: %+v \n", v4poolName, *dirtyIPv4Record)
						if err := frame.UpdateResourceStatus(v4poolObj); err != nil {
							if errors.IsConflict(err) {
								GinkgoWriter.Printf("A conflict occurred when updating the status of Spiderpool v4 IPPool %s: %s, errors: %v", v4poolObj.Name, err)
							}
							return err
						}
						GinkgoWriter.Printf("succeeded to update IPv4 Pool %+v \n", v4poolObj)

						// check if dirty data added successfully
						v4poolObj, err = common.GetIppoolByName(frame, v4poolName)
						Expect(err).NotTo(HaveOccurred())
						record, ok := allocatedRecords[dirtyIPv4]
						Expect(ok).To(BeTrue())
						Expect(record).To(Equal(*dirtyIPv4Record))
					}

					if frame.Info.IpV6Enabled {
						dirtyIPv6Record = podIPv6Record
						if v == "Pod" {
							dirtyIPv6Record.NamespacedName = dirtyPodName
						} else {
							dirtyIPv6Record.PodUID = dirtyContainerID
						}
						allocatedIPCount := *v6poolObj.Status.AllocatedIPCount
						allocatedIPCount++
						GinkgoWriter.Printf("allocatedIPCount: %v\n", allocatedIPCount)
						v6poolObj.Status.AllocatedIPCount = ptr.To(allocatedIPCount)

						allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(v6poolObj.Status.AllocatedIPs)
						Expect(err).NotTo(HaveOccurred())
						allocatedRecords[dirtyIPv6] = *dirtyIPv6Record
						// Update dirty data to IPv6 IPPool.Status.AllocatedIPs
						GinkgoWriter.Printf("update v6 IPPool %v for adding dirty record: %+v \n", v6poolName, *dirtyIPv6Record)
						if err := frame.UpdateResourceStatus(v6poolObj); err != nil {
							if errors.IsConflict(err) {
								GinkgoWriter.Printf("A conflict occurred when updating the status of Spiderpool v6 IPPool %s: %s, errors: %v", v6poolObj.Name, err)
							}
							return err
						}
						GinkgoWriter.Printf("succeeded to update IPv6 Pool %+v \n", v6poolObj)

						// Check if dirty data added successfully
						v6poolObj, err = common.GetIppoolByName(frame, v6poolName)
						Expect(err).NotTo(HaveOccurred())
						record, ok := allocatedRecords[dirtyIPv6]
						Expect(ok).To(BeTrue())
						Expect(record).To(Equal(*dirtyIPv6Record))
					}
					return nil
				})

				// check the real pod ip should be recorded in spiderpool, the dirty ip record should be reclaimed from spiderpool
				GinkgoWriter.Printf("check if the pod %v/%v ip recorded in ippool, check if the dirty ip record reclaimed from ippool\n", namespace, podName)
				if frame.Info.IpV4Enabled {
					// check if dirty IPv4 data reclaimed successfully
					ctx, cancel := context.WithTimeout(context.Background(), common.IPReclaimTimeout)
					defer cancel()
					Expect(common.WaitIppoolStatusConditionByAllocatedIPs(ctx, frame, v4poolName, dirtyIPv4, false)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					// check if dirty IPv6 data reclaimed successfully
					ctx, cancel := context.WithTimeout(context.Background(), common.IPReclaimTimeout)
					defer cancel()
					Expect(common.WaitIppoolStatusConditionByAllocatedIPs(ctx, frame, v6poolName, dirtyIPv6, false)).NotTo(HaveOccurred())
				}
			}
			// Check Pod IP in IPPool
			allRecorded, _, _, err = common.CheckPodIpRecordInIppool(frame, v4poolNameList, v6poolNameList, podList)
			Expect(allRecorded).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("succeed to check pod %v/%v ip in ippool\n", namespace, podName)

			// Delete Pod
			Expect(frame.DeletePod(podName, namespace)).To(Succeed(), "Failed to delete pod %v/%v\n", namespace, podName)
			GinkgoWriter.Printf("succeed to delete pod %v/%v\n", namespace, podName)

			// Check whether the dirty IP data is recovered successfully and whether the AllocatedIPCount decreases and meets expectations?
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					if err := common.CheckIppoolSanity(frame, v4poolName); err != nil {
						return err
					}
					GinkgoWriter.Printf("successfully checked sanity of v4 spiderIPPool %v \n", v4poolName)
				}
				if frame.Info.IpV6Enabled {
					if err := common.CheckIppoolSanity(frame, v6poolName); err != nil {
						return err
					}
					GinkgoWriter.Printf("successfully checked sanity of v6 spiderIPPool %v \n", v6poolName)
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 10).Should(BeNil())
		},
			Entry("a dirty IP record (pod name is wrong or containerID is wrong) in the IPPool should be auto clean by Spiderpool", Serial, Label("G00005", "G00007")),
		)
	})

	Context("choose to release conflicted ip of stateless workload with node not ready", Serial, func() {
		ctx := context.TODO()
		var workerNodeName string
		var originalPodList *corev1.PodList
		const SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED = "SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED"

		BeforeEach(func() {
			// 0. change the spiderpool-controller env
			trueStr := strconv.FormatBool(true)
			deployment, err := frame.GetDeployment(constant.SpiderpoolController, "kube-system")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			oldDeploy := deployment.DeepCopy()
			hasEnv := false
			for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
				if env.Name == SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED {
					if env.Value != trueStr {
						env.Value = trueStr
						hasEnv = true
						break
					}
				}
			}
			if !hasEnv {
				deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED,
					Value: trueStr,
				})
			}
			GinkgoWriter.Printf("successfully changed the spiderpool-controller env %v=%v \n", SPIDERPOOL_GC_STATELESS_TERMINATING_POD_ON_NOT_READY_NODE_ENABLED, trueStr)

			// set deployment maxSurge to be 0 to avoid Pending spiderpool-controller pod created after restart the worker node
			zeroMaxSurge := intstr.FromInt32(0)
			if deployment.Spec.Strategy.RollingUpdate == nil {
				deployment.Spec.Strategy.RollingUpdate = &v1.RollingUpdateDeployment{
					MaxSurge: &zeroMaxSurge,
				}
			} else {
				deployment.Spec.Strategy.RollingUpdate.MaxSurge = &zeroMaxSurge
			}
			GinkgoWriter.Printf("successfully changed the spiderpool-controller deployment maxSurge to be %v \n", zeroMaxSurge)

			// Get the original spiderpool-controller Pod list，
			// to check if the Pods have been restarted
			originalPodList, err = frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
			Expect(err).NotTo(HaveOccurred(), "failed to get spiderpoolController Pod list, error is: %v", err)

			// When maxSurge is 0, spiderpool-controller Pods will be restarted one by one
			err = frame.KClient.Patch(ctx, deployment, client.MergeFrom(oldDeploy))
			Expect(err).NotTo(HaveOccurred())

			// Get the name of the work-node for restarting kubelet
			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			for _, tmpNode := range nodeList.Items {
				if strings.Contains(tmpNode.Name, "worker") {
					workerNodeName = tmpNode.Name
					GinkgoWriter.Printf("worker node: %s\n", workerNodeName)
					break
				}
			}
			Expect(workerNodeName).NotTo(Equal(""))

			// In the current case, kubelet is stopped and all spiderpool-controllers are restarted.
			// After the use case is completed, check whether the environment returns to normal.
			DeferCleanup(func() {
				commandStr := "systemctl start kubelet"
				output, err := frame.DockerExecCommand(ctx, workerNodeName, commandStr)
				Expect(err).NotTo(HaveOccurred(), "Failed exec '%s' in docker container '%s', error is: %v,log: %v.", commandStr, workerNodeName, err, string(output))
				Eventually(func() error {
					checkCommandStr := "systemctl is-active kubelet"
					output, err := frame.DockerExecCommand(ctx, workerNodeName, checkCommandStr)
					if err != nil {
						return fmt.Errorf("Failed to check kubelet status: %v, log: %v", err, string(output))
					}
					if strings.TrimSpace(string(output)) != "active" {
						return fmt.Errorf("kubelet is not running, status: %v", strings.TrimSpace(string(output)))
					}
					return nil
				}).WithTimeout(common.PodReStartTimeout).WithPolling(10 * time.Second).Should(BeNil())
				GinkgoWriter.Println("succeed to check kubelet status.")

				Eventually(func() error {
					newPodList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
					if err != nil {
						return err
					}

					if len(newPodList.Items) == 0 && len(newPodList.Items) != len(frame.Info.KindNodeList) {
						return fmt.Errorf("The number of Spiderpool controllers does not meet expectations. Expected %d, but got %d.", len(frame.Info.KindNodeList), len(newPodList.Items))
					}

					for _, newPod := range newPodList.Items {
						// Prevent spiderpoolcontroller Pod termination failure, avoid spiderpoolcontroller Pod deletion timeout
						if !podutils.IsPodReady(&newPod) && newPod.DeletionTimestamp != nil {
							GinkgoWriter.Printf("delete spiderpoolcontroller pod %v/%v \n", newPod.Namespace, newPod.Name)
							err = frame.DeletePodList(&corev1.PodList{Items: []corev1.Pod{newPod}})
							if err != nil {
								return err
							}
						} else if !podutils.IsPodReady(&newPod) {
							// If the Pod is not running, return an error
							return fmt.Errorf("Pod %s/%s on node '%s' is not running yet", newPod.Namespace, newPod.Name, newPod.Spec.NodeName)
						} else {
							// After setting the spiderpool-controller Deployment’s maxSurge to 0
							// and patching the spiderpool-controller resource on line 671,
							// Pods will terminate and restart sequentially.
							// Check the status of all Pods to ensure they complete their restart and return to the Running state,
							// preventing any timing issues where some Pods may not have fully restarted
							// while the current test case ends, which could affect subsequent test cases.
							for _, originalPod := range originalPodList.Items {
								if originalPod.Name == newPod.Name && originalPod.Namespace == newPod.Namespace {
									return fmt.Errorf("The Pod %s/%s on the node %s has not yet finished restarting, please wait...", newPod.Namespace, newPod.Name, newPod.Spec.NodeName)
								}
							}
						}
					}
					return nil
				}).WithTimeout(common.PodReStartTimeout).WithPolling(10 * time.Second).Should(BeNil())
				GinkgoWriter.Println("succeed to check spiderpool-controller Pod status.")

				// wait for Node to be ready
				webhookHealthCheckClient := openapi.NewWebhookHealthCheckClient()
				Eventually(func() error {
					nodeList, err := frame.GetNodeList()
					if nil != err {
						return err
					}
					for _, node := range nodeList.Items {
						isNodeReady := nodemanager.IsNodeReady(&node)
						if !isNodeReady {
							return fmt.Errorf("node '%s' is still not ready", node.Name)
						}

						var nodeIP string
						for _, nodeAddress := range node.Status.Addresses {
							if nodeAddress.Type == corev1.NodeInternalIP {
								nodeIP = nodeAddress.Address
							}
						}
						Expect(nodeIP).NotTo(BeEmpty())
						err = openapi.WebhookHealthyCheck(webhookHealthCheckClient, common.WebhookPort, &nodeIP)
						if nil != err {
							return fmt.Errorf("node '%s' spiderpool-controller is still not ready with webhook", node.Name)
						}
					}
					return nil
				}).WithTimeout(4 * time.Minute).WithPolling(10 * time.Second).Should(BeNil())
				GinkgoWriter.Println("succeed to check node status and webhook status.")
			})
		})

		It("stateless workload IP could be released with node not ready", Label("G00009"), func() {
			// 1.create a pod
			podYaml := common.GenerateExamplePodYaml(podName, namespace)
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)
			podYaml.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			podYaml.Spec.NodeName = workerNodeName
			Expect(podYaml).NotTo(BeNil())
			GinkgoWriter.Printf("try to create Pod %v/%v \n", namespace, podName)
			err = frame.CreatePod(podYaml)
			Expect(err).NotTo(HaveOccurred(), "failed to create Pod %v/%v \n", namespace, podName)
			ctxWithTimeout, _ := context.WithTimeout(ctx, time.Minute*3)
			podYaml, err = frame.WaitPodStarted(podName, namespace, ctxWithTimeout)
			Expect(err).NotTo(HaveOccurred())

			// 2. record the pod IPs
			var podV4IP, podV6IP string
			var spiderEndpoint spiderpool.SpiderEndpoint
			err = frame.KClient.Get(ctx, types.NamespacedName{
				Namespace: podYaml.Namespace,
				Name:      podYaml.Name,
			}, &spiderEndpoint)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderEndpoint.Status.Current.IPs).To(HaveLen(1))
			if frame.Info.IpV4Enabled {
				Expect(spiderEndpoint.Status.Current.IPs[0].IPv4).NotTo(BeNil())
				podV4IP = strings.Split(*spiderEndpoint.Status.Current.IPs[0].IPv4, "/")[0]
			}
			if frame.Info.IpV6Enabled {
				Expect(spiderEndpoint.Status.Current.IPs[0].IPv6).NotTo(BeNil())
				podV6IP = strings.Split(*spiderEndpoint.Status.Current.IPs[0].IPv6, "/")[0]
			}
			GinkgoWriter.Printf("Pod '%s/%s' has IP '%v' \n", podYaml.Namespace, podYaml.Name, podYaml.Status.PodIPs)

			// 3. set "spider-worker" kubelet down
			commandStr := "systemctl stop kubelet"
			output, err := frame.DockerExecCommand(ctx, workerNodeName, commandStr)
			Expect(err).NotTo(HaveOccurred(), "Failed exec '%s' in docker container '%s', error is: %v,log: %v.", commandStr, workerNodeName, err, string(output))

			// 4. wait for the Node to be 'NotReady'
			tick := time.Tick(time.Minute * 3)
		END:
			for {
				select {
				case <-tick:
					Skip(fmt.Sprintf("timeout to wait for the Pod '%s/%s' to be Terminating, skip this case", namespace, podName))
				default:
					workerNode, err := frame.GetNode(workerNodeName)
					if nil != err {
						if errors.IsNotFound(err) {
							GinkgoWriter.Printf("Node '%s' is not found\n", workerNodeName)
							break END
						}
						GinkgoWriter.Printf("failed to get Node '%s', error: %v\n", workerNodeName, err)
					} else {
						isNodeReady := nodemanager.IsNodeReady(workerNode)
						if !isNodeReady {
							GinkgoWriter.Printf("Node '%s' is not ready right now\n", workerNodeName)
							break END
						}
					}
					time.Sleep(time.Second * 10)
				}
			}

			// 5. wait for the IPs to be released
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					defaultV4pool, err := common.GetIppoolByName(frame, common.SpiderPoolIPv4PoolDefault)
					if nil != err {
						return fmt.Errorf("failed to get IPPool %s, error: %v", common.SpiderPoolIPv4PoolDefault, err)
					}

					v4Allocations, err := convert.UnmarshalIPPoolAllocatedIPs(defaultV4pool.Status.AllocatedIPs)
					Expect(err).NotTo(HaveOccurred())
					_, ok := v4Allocations[podV4IP]
					if ok {
						return fmt.Errorf("IPv4 '%s' is still recorded in IPPool %s", podV4IP, common.SpiderPoolIPv4PoolDefault)
					}
					GinkgoWriter.Printf("the podIP '%s' is already released\n", podV4IP)
				}
				if frame.Info.IpV6Enabled {
					defaultV6pool, err := common.GetIppoolByName(frame, common.SpiderPoolIPv6PoolDefault)
					if nil != err {
						return fmt.Errorf("failed to get IPPool %s, error: %v", common.SpiderPoolIPv6PoolDefault, err)
					}

					v6Allocations, err := convert.UnmarshalIPPoolAllocatedIPs(defaultV6pool.Status.AllocatedIPs)
					Expect(err).NotTo(HaveOccurred())
					_, ok := v6Allocations[podV6IP]
					if ok {
						return fmt.Errorf("IPv4 '%s' is still recorded in IPPool %s", podV6IP, common.SpiderPoolIPv6PoolDefault)
					}
					GinkgoWriter.Printf("the podIP '%s' is already released\n", podV6IP)
				}
				return nil
			}).WithTimeout(3 * time.Minute).WithPolling(time.Second * 10).Should(BeNil())
		})
	})

	It("IP addresses not used by statefulSet can be released by gc all", Label("G00010", "overlay"), func() {
		if !common.CheckRunOverlayCNI() {
			Skip("overlay CNI is not installed , ignore this case")
		}

		var (
			stsName        string = "sts-" + tools.RandomName()
			stsReplicasNum int32  = 2
		)

		// 1. Using the default pool, create a set of statefulset applications and check that spiderpool assigns it an IP address.
		var annotations = make(map[string]string)
		podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)
		annotations[constant.AnnoPodIPPool] = podIppoolAnnoStr
		annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
		stsYaml := common.GenerateExampleStatefulSetYaml(stsName, namespace, stsReplicasNum)
		stsYaml.Spec.Template.Annotations = annotations
		GinkgoWriter.Printf("Try to create StatefulSet %v/%v \n", namespace, stsName)
		Expect(frame.CreateStatefulSet(stsYaml)).To(Succeed(), "failed to create StatefulSet %v/%v \n", namespace, stsName)

		var podList *corev1.PodList
		Eventually(func() bool {
			podList, err = frame.GetPodListByLabel(stsYaml.Spec.Template.Labels)
			if nil != err || len(podList.Items) == 0 {
				return false
			}
			return frame.CheckPodListRunning(podList)
		}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
		GinkgoWriter.Printf("Check that the Pod IP record is in the expected v4 pool %v , v6 pool %v \n", globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)
		ok, _, _, err := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())

		// 2. Remove the spiderpool annotation of the statefulset
		stsObj, err := frame.GetStatefulSet(stsName, namespace)
		Expect(err).NotTo(HaveOccurred())
		desiredStsObj := stsObj.DeepCopy()
		desiredStsObj.Spec.Template.Annotations = map[string]string{}
		Expect(common.PatchStatefulSet(frame, desiredStsObj, stsObj)).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Successfully removed statefulset's %v/%v annotations: %v about spiderpool \n", namespace, stsName, annotations)

		// 3. If the statefulSet does not use spiderpool resources, the spiderpool resources will be released in the gc all phase
		// The interval of gc all in CI is 30s, and we expect that the resources must be reclaimed within 5 minutes.
		Eventually(func() bool {
			newPodList, err := frame.GetPodListByLabel(stsObj.Spec.Template.Labels)
			if nil != err || len(newPodList.Items) == 0 || len(newPodList.Items) != int(stsReplicasNum) {
				return false
			}

			if !frame.CheckPodListRunning(newPodList) {
				return false
			}

			// Expected endpoint does not exist
			GinkgoWriter.Println("Start waiting for gc all to recycle spiderendpoint \n")

			for _, pod := range podList.Items {
				stsEndpoint, err := common.GetWorkloadByName(frame, namespace, pod.Name)
				if err != nil {
					if api_errors.IsNotFound(err) {
						GinkgoWriter.Printf("The statefulSet endpoint %v/%v has been recycled yet \n", namespace, pod.Name)
						continue
					} else {
						GinkgoWriter.Printf("failed to get endpoint %v/%v \n", namespace, pod.Name)
						return false
					}
				} else {
					GinkgoWriter.Printf("The statefulSet endpoint %v/%v has not been recycled yet, waiting... \n", namespace, stsEndpoint.Name)
					return false
				}
			}
			// The expected IP address does not exist in the pool
			ok, _, _, _ := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList, podList)
			if ok {
				GinkgoWriter.Printf("The historical IP of statefulSet %v/%v in ippool has not been recycled yet, waiting... \n", namespace, stsName)
				return false
			}
			GinkgoWriter.Printf("Check if the statefulset %v/%v IP address does not exist in the v4 pool %v and v6 pool %v \n", namespace, stsName, globalDefaultV4IPPoolList, globalDefaultV6IPPoolList)

			return true
		}, common.IPReclaimTimeout, 10*common.ForcedWaitingTime).Should(BeTrue())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			Expect(frame.DeleteStatefulSet(stsName, namespace)).NotTo(HaveOccurred())
		})
	})
})
