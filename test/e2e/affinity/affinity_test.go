// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package affinity_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test Affinity", Label("affinity"), func() {
	var namespace string
	var v4SubnetName, v6SubnetName string
	var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet

	BeforeEach(func() {
		if frame.Info.SpiderSubnetEnabled {
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 5)
					err := common.CreateSubnet(frame, v4SubnetObject)
					if nil != err {
						GinkgoWriter.Printf("Failed to create v4 Subnet: %v \n", err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 5)
					err := common.CreateSubnet(frame, v6SubnetObject)
					if nil != err {
						GinkgoWriter.Printf("Failed to create v6 Subnet: %v \n", err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
		}

		// create namespace
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err = frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred())

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

	Context("test different IPPool Affinity", func() {
		var (
			matchedPodName, matchedNamespace     string
			unmatchedPodName, unmatchedNamespace string
			matchedNode, unMatchedNode           *corev1.Node
			v4PoolName                           string
			v6PoolName                           string
			v4Pool                               *spiderpoolv2beta1.SpiderIPPool
			v6Pool                               *spiderpoolv2beta1.SpiderIPPool
			v4PoolNameList, v6PoolNameList       []string
		)

		BeforeEach(func() {
			v4PoolNameList = []string{}
			v6PoolNameList = []string{}
			// Init matching and non-matching namespaces name and create its
			matchedNamespace = namespace
			unmatchedNamespace = "unmatched-ns-" + tools.RandomName()

			GinkgoWriter.Printf("create namespace %v \n", unmatchedNamespace)
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(unmatchedNamespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Init matching and non-matching Pod name
			matchedPodName = "matched-pod-" + tools.RandomName()
			unmatchedPodName = "unmatched-pod-" + tools.RandomName()

			// Get the list of nodes and determine if the test conditions are met, skip the test if they are not met
			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeList).NotTo(BeNil())
			GinkgoWriter.Printf("get nodeList: %+v\n", nodeList.Items)

			if len(nodeList.Items) < 2 {
				Skip("this case needs 2 nodes at least\n")
			}
			// Generate matching and non-matching node
			matchedNode = &nodeList.Items[0]
			unMatchedNode = &nodeList.Items[1]

			// Set matching namespace label
			GinkgoWriter.Printf("label namespace %v\n", matchedNamespace)
			ns, err := frame.GetNamespace(matchedNamespace)
			Expect(ns).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			ns.Labels = map[string]string{matchedNamespace: matchedNamespace}
			Expect(frame.UpdateResource(ns)).To(Succeed())

			// Assign different type of affinity to the ippool and create it
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
					if frame.Info.SpiderSubnetEnabled {
						v4Pool.Spec.Subnet = v4SubnetObject.Spec.Subnet
						v4Pool.Spec.IPs = v4SubnetObject.Spec.IPs
					}
					GinkgoWriter.Printf("create v4 ippool %v\n", v4PoolName)
					v4Pool.Spec.NodeAffinity = new(v1.LabelSelector)
					v4Pool.Spec.NamespaceAffinity = new(v1.LabelSelector)
					v4Pool.Spec.PodAffinity = new(v1.LabelSelector)
					v4Pool.Spec.NodeAffinity.MatchLabels = matchedNode.GetLabels()
					v4Pool.Spec.NamespaceAffinity.MatchLabels = ns.Labels
					v4Pool.Spec.PodAffinity.MatchLabels = map[string]string{matchedPodName: matchedPodName}
					err = common.CreateIppool(frame, v4Pool)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
						return err
					}
					v4PoolNameList = append(v4PoolNameList, v4PoolName)
					GinkgoWriter.Printf("succeeded to create ippool %v\n", v4Pool.Name)
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6Pool = common.GenerateExampleIpv6poolObject(1)
					if frame.Info.SpiderSubnetEnabled {
						v6Pool.Spec.Subnet = v6SubnetObject.Spec.Subnet
						v6Pool.Spec.IPs = v6SubnetObject.Spec.IPs
					}
					GinkgoWriter.Printf("create v6 ippool %v\n", v6PoolName)
					v6Pool.Spec.NodeAffinity = new(v1.LabelSelector)
					v6Pool.Spec.NamespaceAffinity = new(v1.LabelSelector)
					v6Pool.Spec.PodAffinity = new(v1.LabelSelector)
					v6Pool.Spec.NodeAffinity.MatchLabels = matchedNode.GetLabels()
					v6Pool.Spec.NamespaceAffinity.MatchLabels = ns.Labels
					v6Pool.Spec.PodAffinity.MatchLabels = map[string]string{matchedPodName: matchedPodName}
					err = common.CreateIppool(frame, v6Pool)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
						return err
					}
					v6PoolNameList = append(v6PoolNameList, v6PoolName)
					GinkgoWriter.Printf("succeeded to create ippool %v\n", v6Pool.Name)
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			// Clean test env
			DeferCleanup(func() {
				for _, namespace := range []string{matchedNamespace, unmatchedNamespace} {
					GinkgoWriter.Printf("delete namespace %v \n", namespace)
					err := frame.DeleteNamespace(namespace)
					Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
				}
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				}
			})
		})
		DescribeTable("create pod with ippool that matched different affinity", func(isNodeMatched, isNamespaceMatched, isPodMatched bool) {
			var namespaceNM, podNM string
			var nodeLabel map[string]string
			allMatched := false
			// Determine if conditions are met based on `isNodeMatched`, `isNamespaceMatched`, `isPodMatched`
			if isNodeMatched && isNamespaceMatched && isPodMatched {
				allMatched = true
				namespaceNM = matchedNamespace
				podNM = matchedPodName
				nodeLabel = matchedNode.GetLabels()
			}
			if !isNodeMatched {
				namespaceNM = matchedNamespace
				podNM = matchedPodName
				nodeLabel = unMatchedNode.GetLabels()
			}
			if !isNamespaceMatched {
				namespaceNM = unmatchedNamespace
				podNM = matchedPodName
				nodeLabel = matchedNode.GetLabels()
			}
			if !isPodMatched {
				namespaceNM = matchedNamespace
				podNM = unmatchedPodName
				nodeLabel = matchedNode.GetLabels()
			}

			// Generate IPPool annotation string
			podAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList)

			// create pod and check if affinity is active
			GinkgoWriter.Printf("create pod %v/%v\n", namespaceNM, podNM)
			podObj := common.GenerateExamplePodYaml(podNM, namespaceNM)
			Expect(podObj).NotTo(BeNil())
			podObj.Spec.NodeSelector = nodeLabel
			podObj.Annotations = map[string]string{constant.AnnoPodIPPool: podAnnoStr}

			if allMatched {
				GinkgoWriter.Println("when matched affinity")
				Expect(frame.CreatePod(podObj)).To(Succeed())
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				pod, err := frame.WaitPodStarted(podNM, namespaceNM, ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod).NotTo(BeNil())
			}
			if !allMatched {
				GinkgoWriter.Println("when unmatched affinity")
				Expect(frame.CreatePod(podObj)).To(Succeed())
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				Expect(frame.WaitExceptEventOccurred(ctx, common.OwnerPod, podNM, namespaceNM, common.GetIpamAllocationFailed)).To(Succeed())
				GinkgoWriter.Printf("succeeded to matched the message %v\n", common.GetIpamAllocationFailed)
			}
		},
			Entry("succeed to run pod who is bound to an ippool set with matched NodeAffinity NamespaceAffinity and PodAffinity", Label("smoke", "L00001", "L00003", "L00005"), true, true, true),
			Entry("failed to run pod who is bound to an ippool set with no-matched NodeAffinity", Label("L00002"), false, true, true),
			Entry("failed to run pod who is bound to an ippool set with no-matched NamespaceAffinity", Label("L00004"), true, false, true),
			Entry("failed to run pod who is bound to an ippool set with no-matched PodAffinity", Label("L00006"), true, true, false),
		)
	})

	Context("cross-zone daemonSet", func() {
		var daemonSetName string
		nodeV4PoolMap := make(map[string][]string)
		nodeV6PoolMap := make(map[string][]string)
		allV4PoolNameList := make([]string, 0)
		allV6PoolNameList := make([]string, 0)

		BeforeEach(func() {
			// daemonSetName name
			daemonSetName = "daemonset" + tools.RandomName()

			// Get the list of nodes and determine if the test conditions are met, skip the test if they are not met
			GinkgoWriter.Println("get node list")
			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeList).NotTo(BeNil())

			if len(nodeList.Items) < 2 {
				Skip("skip: this case need 2 nodes at least")
			}
			// Assign `NodeAffinity` to the ippool and create it
			for _, node := range nodeList.Items {
				if frame.Info.IpV4Enabled {
					v4PoolName, v4Pool := common.GenerateExampleIpv4poolObject(1)
					v4Pool.Spec.NodeAffinity = new(v1.LabelSelector)
					v4Pool.Spec.NodeAffinity.MatchLabels = node.Labels
					GinkgoWriter.Printf("Create v4 ippool %v\n", v4PoolName)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4Pool, 1)).To(Succeed())
					} else {
						Expect(common.CreateIppool(frame, v4Pool)).To(Succeed())
					}

					allV4PoolNameList = append(allV4PoolNameList, v4PoolName)
					nodeV4PoolMap[node.Name] = []string{v4PoolName}
					GinkgoWriter.Printf("node: %v, v4PoolNameList: %+v \n", node.Name, nodeV4PoolMap[node.Name])
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6Pool := common.GenerateExampleIpv6poolObject(1)
					v6Pool.Spec.NodeAffinity = new(v1.LabelSelector)
					v6Pool.Spec.NodeAffinity.MatchLabels = node.Labels
					GinkgoWriter.Printf("Create v6 ippool %v\n", v6PoolName)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6Pool, 1)).To(Succeed())
					} else {
						Expect(common.CreateIppool(frame, v6Pool)).To(Succeed())
					}

					allV6PoolNameList = append(allV6PoolNameList, v6PoolName)
					nodeV6PoolMap[node.Name] = []string{v6PoolName}
					GinkgoWriter.Printf("node: %v, v6PoolNameList: %+v \n", node.Name, nodeV6PoolMap[node.Name])
				}
			}

			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					for _, poolName := range allV4PoolNameList {
						Expect(common.DeleteIPPoolByName(frame, poolName)).NotTo(HaveOccurred())
					}
				}
				if frame.Info.IpV6Enabled {
					for _, poolName := range allV6PoolNameList {
						Expect(common.DeleteIPPoolByName(frame, poolName)).NotTo(HaveOccurred())
					}
				}
			})
		})

		It("Successfully run daemonSet/pod who is cross-zone daemonSet with matched `NodeAffinity`", Label("L00007"), func() {
			// generate daemonSet yaml
			GinkgoWriter.Println("generate example daemonSet yaml")
			daemonSetYaml := common.GenerateExampleDaemonSetYaml(daemonSetName, namespace)
			Expect(daemonSetYaml).NotTo(BeNil(), "failed to generate daemonSet %v/%v yaml\n", namespace, daemonSetName)

			// set annotation to add ippool
			GinkgoWriter.Println("add annotations to daemonSet yaml")
			podAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, allV4PoolNameList, allV6PoolNameList)

			daemonSetYaml.Spec.Template.Annotations = map[string]string{
				constant.AnnoPodIPPool: podAnnoStr,
			}
			GinkgoWriter.Printf("the daemonSet yaml is :%+v\n", daemonSetYaml)

			// Create daemonSet and wait daemonSet ready
			GinkgoWriter.Printf("create daemonSet %v/%v \n", namespace, daemonSetName)
			Expect(frame.CreateDaemonSet(daemonSetYaml)).To(Succeed(), "failed to create daemonSet: %v/%v\n", namespace, daemonSetName)
			GinkgoWriter.Printf("wait daemonset %v/%v ready\n", namespace, daemonSetName)
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			daemonSet, err := frame.WaitDaemonSetReady(daemonSetName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred(), "error: %v\n", err)
			Expect(daemonSet).NotTo(BeNil())

			// get podList
			GinkgoWriter.Printf("get pod list by label: %+v \n", daemonSet.Spec.Template.Labels)
			podList, err := frame.GetPodListByLabel(daemonSet.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred(), "failed to get podList,error: %v \n", err)
			Expect(podList).NotTo(BeNil())

			// check pod ip in different node-ippool
			GinkgoWriter.Println("check pod ip if in different node-ippool")
			for _, pod := range podList.Items {
				ok, _, _, err := common.CheckPodIpRecordInIppool(frame, nodeV4PoolMap[pod.Spec.NodeName], nodeV6PoolMap[pod.Spec.NodeName], &corev1.PodList{Items: []corev1.Pod{pod}})
				Expect(err).NotTo(HaveOccurred(), "error: %v\n", err)
				Expect(ok).To(BeTrue())
			}

			// delete daemonSet
			GinkgoWriter.Printf("delete daemonSet %v/%v\n", namespace, daemonSetName)
			Expect(frame.DeleteDaemonSet(daemonSetName, namespace)).To(Succeed(), "failed to delete daemonSet %v/%v\n", namespace, daemonSetName)
			ctx2, cancel2 := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel2()
			Expect(frame.WaitPodListDeleted(namespace, daemonSet.Spec.Template.Labels, ctx2)).To(Succeed(), "time out to wait podList deleted\n")

			// check pod ip if reclaimed in different node-ippool
			GinkgoWriter.Println("check pod ip if reclaimed in different node-ippool")
			for _, pod := range podList.Items {
				_, ok, _, err := common.CheckPodIpRecordInIppool(frame, nodeV4PoolMap[pod.Spec.NodeName], nodeV6PoolMap[pod.Spec.NodeName], &corev1.PodList{Items: []corev1.Pod{pod}})
				Expect(err).NotTo(HaveOccurred(), "error: %v\n", err)
				Expect(ok).To(BeTrue())
			}
		})
	})

	Context("Support Statefulset pod who will be always assigned same IP addresses.", func() {
		var v4PoolName, v6PoolName, defaultV4PoolName, defaultV6PoolName, statefulSetName string
		var v4PoolObj, v6PoolObj, defaultV4PoolObj, defaultV6PoolObj *spiderpoolv2beta1.SpiderIPPool
		var newPodList *corev1.PodList
		var defaultV4PoolNameList, defaultV6PoolNameList []string
		const stsOriginialNum = int(1)

		BeforeEach(func() {
			// test statefulSet name
			statefulSetName = "sts" + tools.RandomName()

			// Create IPv4 pools and IPv6 pools
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
					defaultV4PoolName, defaultV4PoolObj = common.GenerateExampleIpv4poolObject(5)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4PoolObj, 2)).NotTo(HaveOccurred())
						Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, defaultV4PoolObj, 2)).NotTo(HaveOccurred())
					} else {
						err := common.CreateIppool(frame, v4PoolObj)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
							return err
						}
						err = common.CreateIppool(frame, defaultV4PoolObj)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
							return err
						}
					}
					defaultV4PoolNameList = append(defaultV4PoolNameList, defaultV4PoolName)
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
					defaultV6PoolName, defaultV6PoolObj = common.GenerateExampleIpv6poolObject(5)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6PoolObj, 2)).NotTo(HaveOccurred())
						Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, defaultV6PoolObj, 2)).NotTo(HaveOccurred())
					} else {
						err := common.CreateIppool(frame, v6PoolObj)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v6 IPPool: %v \n", err)
							return err
						}
						err = common.CreateIppool(frame, defaultV6PoolObj)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v6 IPPool: %v \n", err)
							return err
						}
					}
					defaultV6PoolNameList = append(defaultV6PoolNameList, defaultV6PoolName)
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			DeferCleanup(func() {
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
					Expect(common.DeleteIPPoolByName(frame, defaultV4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
					Expect(common.DeleteIPPoolByName(frame, defaultV6PoolName)).NotTo(HaveOccurred())
				}
			})
		})

		It("Successfully restarted statefulSet/pod with matching podSelector, ip remains the same", Label("L00008", "A00009"), func() {
			// A00009:Modify the annotated IPPool for a specified StatefulSet pod
			// Generate ippool annotation string
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, defaultV4PoolNameList, defaultV6PoolNameList)

			stsObject := common.GenerateExampleStatefulSetYaml(statefulSetName, namespace, int32(stsOriginialNum))
			stsObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}

			// Try to create a statefulSet and wait for replicas to meet expectations
			ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel1()
			GinkgoWriter.Printf("try to create statefulset %v \n", stsObject)
			err := frame.CreateStatefulSet(stsObject)
			Expect(err).NotTo(HaveOccurred())
			stsObject, err = frame.WaitStatefulSetReady(statefulSetName, namespace, ctx1)
			Expect(stsObject).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			podlist, err := frame.GetPodListByLabel(stsObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred(), "failed to get pod list, reason= %v", err)
			Expect(int32(len(podlist.Items))).Should(Equal(stsObject.Status.ReadyReplicas))

			// check pod ip record in ippool
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, defaultV4PoolNameList, defaultV6PoolNameList, podlist)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			// Record the Pod IP、containerID、workloadendpoint UID information of the statefulset
			ipMap := make(map[string]string)
			containerIdMap := make(map[string]string)
			uidMap := make(map[string]string)
			for _, pod := range podlist.Items {
				if frame.Info.IpV4Enabled {
					podIPv4, ok := tools.CheckPodIpv4IPReady(&pod)
					Expect(ok).NotTo(BeFalse(), "failed to get ipv4 ip")
					Expect(podIPv4).NotTo(BeEmpty(), "podIPv4 is a empty string")
					ipMap[podIPv4] = pod.Name
				}
				if frame.Info.IpV6Enabled {
					podIPv6, ok := tools.CheckPodIpv6IPReady(&pod)
					Expect(ok).NotTo(BeFalse(), "failed to get ipv6 ip")
					Expect(podIPv6).NotTo(BeEmpty(), "podIPv6 is a empty string")
					ipMap[podIPv6] = pod.Name
				}
				for _, c := range pod.Status.ContainerStatuses {
					containerIdMap[c.ContainerID] = pod.Name
				}
				object, err := common.GetWorkloadByName(frame, pod.Namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(object).NotTo(BeNil())
				uidMap[string(object.UID)] = pod.Name
			}
			GinkgoWriter.Printf("StatefulSet %s/%s corresponding Pod IP allocations: %v \n", stsObject.Namespace, stsObject.Name, ipMap)

			// A00009：Modify the annotated IPPool for a specified StatefulSet pod, the pod wouldn't change IP
			podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, common.NIC1, []string{v4PoolName}, []string{v6PoolName})
			stsObject, err = frame.GetStatefulSet(statefulSetName, namespace)
			Expect(err).NotTo(HaveOccurred())
			stsObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			// Modify the ippool in annotation and update the statefulset
			GinkgoWriter.Printf("try to update StatefulSet %s/%s template with new annotations: %v \n", stsObject.Namespace, stsObject.Name, stsObject.Spec.Template.Annotations)
			Expect(frame.UpdateResource(stsObject)).NotTo(HaveOccurred())

			// Check that the container ID should be different
			ctx2, cancel2 := context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel2()
		LOOP:
			for {
				select {
				case <-ctx2.Done():
					Fail("After statefulset restart, the container id waits for the change to time out \n")
				default:
					newPodList, err = frame.GetPodListByLabel(stsObject.Spec.Selector.MatchLabels)
					Expect(err).NotTo(HaveOccurred())
					if len(newPodList.Items) == 0 {
						time.Sleep(common.ForcedWaitingTime)
						continue LOOP
					}
					for _, pod := range newPodList.Items {
						// Make sure the modified annotation takes effect in the pod
						if pod.Annotations[constant.AnnoPodIPPool] == podIppoolAnnoStr {
							GinkgoWriter.Printf("Pod %v/%v Annotations is %v", pod.Namespace, pod.Name, pod.Annotations[constant.AnnoPodIPPool])
						} else {
							time.Sleep(common.ForcedWaitingTime)
							continue LOOP
						}
						for _, c := range pod.Status.ContainerStatuses {
							if _, ok := containerIdMap[c.ContainerID]; ok {
								time.Sleep(common.ForcedWaitingTime)
								continue LOOP
							}
						}
						break LOOP
					}
				}
			}
			ctx3, cancel3 := context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel3()
			Expect(frame.WaitPodListRunning(stsObject.Spec.Selector.MatchLabels, stsOriginialNum, ctx3)).NotTo(HaveOccurred())
			newPodList, err = frame.GetPodListByLabel(stsObject.Spec.Selector.MatchLabels)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(newPodList.Items)).Should(Equal(stsOriginialNum))
			for _, pod := range newPodList.Items {
				// IP remains the same
				if frame.Info.IpV4Enabled {
					podIPv4, ok := tools.CheckPodIpv4IPReady(&pod)
					Expect(ok).NotTo(BeFalse(), "Failed to get IPv4 IP")
					Expect(podIPv4).NotTo(BeEmpty(), "podIPv4 is a empty string")
					d, ok := ipMap[podIPv4]
					Expect(ok).To(BeTrue(), fmt.Sprintf("original StatefulSet Pod IP allcations: %v, new Pod %s/%s IPv4 %s", ipMap, pod.Namespace, pod.Name, podIPv4))
					GinkgoWriter.Printf("Pod %v IP %v remains the same \n", d, podIPv4)
				}
				if frame.Info.IpV6Enabled {
					podIPv6, ok := tools.CheckPodIpv6IPReady(&pod)
					Expect(ok).NotTo(BeFalse(), "Failed to get IPv6 IP")
					Expect(podIPv6).NotTo(BeEmpty(), "podIPv6 is a empty string")
					d, ok := ipMap[podIPv6]
					Expect(ok).To(BeTrue(), fmt.Sprintf("original StatefulSet Pod IP allcations: %v, new Pod %s/%s IPv6 %s", ipMap, pod.Namespace, pod.Name, podIPv6))
					GinkgoWriter.Printf("Pod %v IP %v remains the same \n", d, podIPv6)
				}
				// WorkloadEndpoint UID remains the same
				object, err := common.GetWorkloadByName(frame, pod.Namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred(), "Failed to get the same uid")
				d, ok := uidMap[string(object.UID)]
				Expect(ok).To(BeTrue(), "Failed to get the same uid")
				GinkgoWriter.Printf("Pod %v workloadendpoint UID %v remains the same \n", d, object.UID)
			}

			// Delete Statefulset and Check if the Pod IP in IPPool reclaimed normally
			err = frame.DeleteStatefulSet(statefulSetName, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(common.WaitIPReclaimedFinish(frame, defaultV4PoolNameList, defaultV6PoolNameList, newPodList, common.IPReclaimTimeout)).To(Succeed())

			// Check workloadendpoint records are deleted
			ctx4, cancel4 := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel4()
			for _, pod := range newPodList.Items {
				err := common.WaitWorkloadDeleteUntilFinish(ctx4, frame, pod.Namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Context("one IPPool can be used by multiple namespace", func() {
		var v4PoolName, v6PoolName, nsName1 string
		var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
		var v4PoolNameList, v6PoolNameList []string
		nsName1 = "ns1" + tools.RandomName()

		BeforeEach(func() {
			// Create another namespace
			GinkgoWriter.Printf("create another namespace %v \n", nsName1)
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName1, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			// create IPPool
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
				if frame.Info.SpiderSubnetEnabled {
					v4PoolObj.Spec.Subnet = v4SubnetObject.Spec.Subnet
					v4PoolObj.Spec.IPs = v4SubnetObject.Spec.IPs
				}
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
				Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
				if frame.Info.SpiderSubnetEnabled {
					v6PoolObj.Spec.Subnet = v6SubnetObject.Spec.Subnet
					v6PoolObj.Spec.IPs = v6SubnetObject.Spec.IPs
				}
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
			}

			// clean test env
			DeferCleanup(func() {
				// delete namespaces
				GinkgoWriter.Printf("delete namespace %v\n", nsName1)
				Expect(frame.DeleteNamespace(nsName1)).NotTo(HaveOccurred())

				// delete ippool
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).To(Succeed())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).To(Succeed())
				}
			})
		})

		It("deployment pod can running in this ippool with two namespaces ", Label("L00009"), func() {
			deployName1 := "deploy1" + tools.RandomName()
			deployName2 := "deploy2" + tools.RandomName()
			var deployOriginialNum int32 = 1
			nsLabel := map[string]string{nsName1: nsName1}
			nsNamesList := []string{nsName1, namespace}

			for i, nsName := range nsNamesList {
				// get namespace and set namespace regular label
				GinkgoWriter.Printf("set %v-th, %v namespace label \n", i, nsName)
				nsObject, err := frame.GetNamespace(nsName)
				Expect(err).NotTo(HaveOccurred())
				nsObject.Labels = nsLabel
				Expect(frame.UpdateResource(nsObject)).To(Succeed())

				// set ns1 and ns2 annotation to this ippool
				nsObject.Annotations = make(map[string]string)
				if frame.Info.IpV4Enabled {
					v4IppoolAnnoValue := types.AnnoNSDefautlV4PoolValue{}
					common.SetNamespaceIppoolAnnotation(v4IppoolAnnoValue, nsObject, v4PoolNameList, constant.AnnoNSDefautlV4Pool)
				}
				if frame.Info.IpV6Enabled {
					v6IppoolAnnoValue := types.AnnoNSDefautlV6PoolValue{}
					common.SetNamespaceIppoolAnnotation(v6IppoolAnnoValue, nsObject, v6PoolNameList, constant.AnnoNSDefautlV6Pool)
				}
				Expect(frame.UpdateResource(nsObject)).To(Succeed())
			}

			// set ippool label selector to ns1 and ns2
			if frame.Info.IpV4Enabled {
				v4PoolObject, err := common.GetIppoolByName(frame, v4PoolName)
				Expect(err).NotTo(HaveOccurred())
				v4PoolObject.Spec.NamespaceAffinity = new(v1.LabelSelector)
				v4PoolObject.Spec.NamespaceAffinity.MatchLabels = nsLabel
				Expect(common.PatchIppool(frame, v4PoolObject, v4PoolObj)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6PoolObject, err := common.GetIppoolByName(frame, v6PoolName)
				Expect(err).NotTo(HaveOccurred())
				v6PoolObject.Spec.NamespaceAffinity = new(v1.LabelSelector)
				v6PoolObject.Spec.NamespaceAffinity.MatchLabels = nsLabel
				Expect(common.PatchIppool(frame, v6PoolObject, v6PoolObj)).NotTo(HaveOccurred())
			}

			// create 2 deployment in different ns
			depMap := map[string]string{
				deployName1: nsName1,
				deployName2: namespace,
			}
			for d, deps := range depMap {
				common.CreateDeployUnitlReadyCheckInIppool(frame, deps, depMap[d], deployOriginialNum, v4PoolNameList, v6PoolNameList)
			}

			// try to delete deployment
			for d, deps := range depMap {
				Expect(frame.DeleteDeployment(deps, depMap[d])).To(Succeed())
				GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", deps, depMap[deps])
			}
		})
	})
})
