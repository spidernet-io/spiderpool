// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package affinity_test

import (
	"context"
	"encoding/json"
	"time"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("test Affinity", Label("affinity"), func() {
	Context("test different IPPool Affinity", func() {
		var (
			matchedPodName, matchedNamespace     string
			unmatchedPodName, unmatchedNamespace string
			matchedNode, unMatchedNode           *corev1.Node
		)
		var (
			v4PoolName string
			v6PoolName string
			v4Pool     *spiderpoolv1.SpiderIPPool
			v6Pool     *spiderpoolv1.SpiderIPPool
		)

		BeforeEach(func() {
			// Init matching and non-matching namespaces name and create its
			matchedNamespace = "matched-ns-" + tools.RandomName()
			unmatchedNamespace = "unmatched-ns-" + tools.RandomName()

			for _, namespace := range []string{matchedNamespace, unmatchedNamespace} {
				GinkgoWriter.Printf("create namespace %v \n", namespace)
				err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, time.Second*10)
				Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
			}

			// Init matching and non-matching Pod name
			matchedPodName = "matched-pod-" + tools.RandomName()
			unmatchedPodName = "unmatched-pod-" + tools.RandomName()

			// Get the list of nodes and determine if the test conditions are met, skip the test if they are not met
			GinkgoWriter.Println("get node list")
			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeList).NotTo(BeNil())
			GinkgoWriter.Printf("nodeList: %+v\n", nodeList.Items)

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
			if frame.Info.IpV4Enabled {
				v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
				GinkgoWriter.Printf("create v4 ippool %v\n", v4PoolName)
				v4Pool.Spec.NodeAffinity = new(v1.LabelSelector)
				v4Pool.Spec.NamespaceAffinity = new(v1.LabelSelector)
				v4Pool.Spec.PodAffinity = new(v1.LabelSelector)
				v4Pool.Spec.NodeAffinity.MatchLabels = matchedNode.GetLabels()
				v4Pool.Spec.NamespaceAffinity.MatchLabels = ns.Labels
				v4Pool.Spec.PodAffinity.MatchLabels = map[string]string{matchedPodName: matchedPodName}
				Expect(common.CreateIppool(frame, v4Pool)).To(Succeed())
				GinkgoWriter.Printf("succeeded to create ippool %v\n", v4Pool.Name)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6Pool = common.GenerateExampleIpv6poolObject(1)
				GinkgoWriter.Printf("create v6 ippool %v\n", v6PoolName)
				v6Pool.Spec.NodeAffinity = new(v1.LabelSelector)
				v6Pool.Spec.NamespaceAffinity = new(v1.LabelSelector)
				v6Pool.Spec.PodAffinity = new(v1.LabelSelector)
				v6Pool.Spec.NodeAffinity.MatchLabels = matchedNode.GetLabels()
				v6Pool.Spec.NamespaceAffinity.MatchLabels = ns.Labels
				v6Pool.Spec.PodAffinity.MatchLabels = map[string]string{matchedPodName: matchedPodName}
				Expect(common.CreateIppool(frame, v6Pool)).To(Succeed())
				GinkgoWriter.Printf("succeeded to create ippool %v\n", v6Pool.Name)
			}
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

			// create pod and check if affinity is active
			GinkgoWriter.Printf("create pod %v/%v\n", namespaceNM, podNM)
			podObj := common.GenerateExamplePodYaml(podNM, namespaceNM)
			Expect(podObj).NotTo(BeNil())
			podObj.Spec.NodeSelector = nodeLabel

			podAnno := types.AnnoPodIPPoolValue{}

			if frame.Info.IpV4Enabled {
				podAnno.IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podAnno.IPv6Pools = []string{v6PoolName}
			}
			b, err := json.Marshal(podAnno)
			podAnnoStr := string(b)
			Expect(err).NotTo(HaveOccurred())

			podObj.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool: podAnnoStr,
			}
			GinkgoWriter.Printf("podObj: %v\n", podObj)

			if allMatched {
				GinkgoWriter.Println("when matched affinity")
				Expect(frame.CreatePod(podObj)).To(Succeed())
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				pod, err := frame.WaitPodStarted(podNM, namespaceNM, ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod).NotTo(BeNil())
			}
			if !allMatched {
				GinkgoWriter.Println("when unmatched affinity")
				Expect(frame.CreatePod(podObj)).To(Succeed())
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				Expect(frame.WaitExceptEventOccurred(ctx, common.PodEventKind, podNM, namespaceNM, common.GetIpamAllocationFailed)).To(Succeed())
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
		var namespace, daemonSetName string
		var err error

		nodeV4PoolMap := make(map[string][]string)
		nodeV6PoolMap := make(map[string][]string)
		allV4PoolNameList := make([]string, 0)
		allV6PoolNameList := make([]string, 0)

		BeforeEach(func() {
			// create namespace
			namespace = "ns" + tools.RandomName()
			GinkgoWriter.Printf("create namespace %v \n", namespace)
			err = frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, time.Second*10)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
			GinkgoWriter.Printf("succeed to create namespace %v \n", namespace)

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
					GinkgoWriter.Printf("create v4 ippool %v\n", v4PoolName)
					v4Pool.Spec.NodeAffinity = new(v1.LabelSelector)
					v4Pool.Spec.NodeAffinity.MatchLabels = node.Labels
					Expect(common.CreateIppool(frame, v4Pool)).To(Succeed())
					GinkgoWriter.Printf("succeeded to create ippool %v\n", v4Pool.Name)

					allV4PoolNameList = append(allV4PoolNameList, v4PoolName)
					nodeV4PoolMap[node.Name] = []string{v4PoolName}
					GinkgoWriter.Printf("node: %v, v4PoolNameList: %+v \n", node.Name, nodeV4PoolMap[node.Name])
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6Pool := common.GenerateExampleIpv6poolObject(1)
					GinkgoWriter.Printf("create v6 ippool %v\n", v6PoolName)
					v6Pool.Spec.NodeAffinity = new(v1.LabelSelector)
					v6Pool.Spec.NodeAffinity.MatchLabels = node.Labels
					Expect(common.CreateIppool(frame, v6Pool)).To(Succeed())
					GinkgoWriter.Printf("succeeded to create ippool %v\n", v6Pool.Name)

					allV6PoolNameList = append(allV6PoolNameList, v6PoolName)
					nodeV6PoolMap[node.Name] = []string{v6PoolName}
					GinkgoWriter.Printf("node: %v, v6PoolNameList: %+v \n", node.Name, nodeV6PoolMap[node.Name])
				}
			}

			DeferCleanup(func() {
				// delete namespace
				GinkgoWriter.Printf("delete namespace %v \n", namespace)
				err := frame.DeleteNamespace(namespace)
				Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
				GinkgoWriter.Printf("succeed to delete namespace %v \n", namespace)

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
			anno := types.AnnoPodIPPoolValue{}
			if frame.Info.IpV4Enabled {
				anno.IPv4Pools = allV4PoolNameList
			}
			if frame.Info.IpV6Enabled {
				anno.IPv6Pools = allV6PoolNameList
			}
			annoB, err := json.Marshal(anno)
			Expect(err).NotTo(HaveOccurred(), "failed to marshal pod annotations %+v\n", anno)
			annoStr := string(annoB)

			daemonSetYaml.Spec.Template.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool: annoStr,
			}
			GinkgoWriter.Printf("the daemonSet yaml is :%+v\n", daemonSetYaml)

			// Create daemonSet and wait daemonSet ready
			GinkgoWriter.Printf("create daemonSet %v/%v \n", namespace, daemonSetName)
			Expect(frame.CreateDaemonSet(daemonSetYaml)).To(Succeed(), "failed to create daemonSet: %v/%v\n", namespace, daemonSetName)
			GinkgoWriter.Printf("wait daemonset %v/%v ready\n", namespace, daemonSetName)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
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
			ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
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

	Context("one IPPool can be used by multiple namespace", func() {
		var nsName1, nsName2 string
		var v4PoolName, v6PoolName string
		var v4PoolObj, v6PoolObj *spiderpoolv1.SpiderIPPool
		var v4PoolNameList, v6PoolNameList []string
		nsName1 = "ns" + tools.RandomName()
		nsName2 = "ns" + tools.RandomName()
		nsNames := []string{nsName1, nsName2}

		BeforeEach(func() {
			// create 2 namespaces
			for i, nsName := range nsNames {
				GinkgoWriter.Printf("create namespace %v %v\n", i, nsName)
				err := frame.CreateNamespace(nsName)
				Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
			}
			// create one ippool
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv4 pool
				Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv6 pool
				Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
			}

			// clean test env
			DeferCleanup(func() {
				// delete namespaces
				for i, nsName := range nsNames {
					GinkgoWriter.Printf("delete namespace %v %v\n", i, nsName)
					err := frame.DeleteNamespace(nsName)
					Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
				}

				// delete ippool
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolUntilFinish(frame, v4PoolName, ctx)).To(Succeed())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolUntilFinish(frame, v6PoolName, ctx)).To(Succeed())
				}
			})
		})

		It("deployment pod can running in this ippool with two namespaces ", Label("L00009"), func() {
			deployName1 := "deploy" + tools.RandomName()
			deployName2 := "deploy2" + tools.RandomName()

			// get namespace and set namespace regular label
			for i, nsName := range nsNames {
				GinkgoWriter.Printf("set %v %v namespace label\n", i, nsName)
				ns, err := frame.GetNamespace(nsName)
				Expect(ns).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				ns.Labels = map[string]string{nsName1: nsName1}
				Expect(frame.UpdateResource(ns)).To(Succeed())

				// set ns1 and ns2 annotation to this ippool
				ns.Annotations = make(map[string]string)
				if frame.Info.IpV4Enabled {
					v4IppoolAnnoValue := types.AnnoNSDefautlV4PoolValue{}
					common.SetNamespaceIppoolAnnotation(v4IppoolAnnoValue, ns, v4PoolNameList, pkgconstant.AnnoNSDefautlV4Pool)
				}
				if frame.Info.IpV6Enabled {
					v6IppoolAnnoValue := types.AnnoNSDefautlV6PoolValue{}
					common.SetNamespaceIppoolAnnotation(v6IppoolAnnoValue, ns, v6PoolNameList, pkgconstant.AnnoNSDefautlV6Pool)
				}
				Expect(frame.UpdateResource(ns)).To(Succeed())
			}

			// set ippool label selector to ns1 and ns2
			nsLabel := map[string]string{nsName1: nsName1}
			if frame.Info.IpV4Enabled {
				v4PoolObject := common.GetIppoolByName(frame, v4PoolName)
				v4PoolObject.Spec.NamespaceAffinity = new(v1.LabelSelector)
				v4PoolObject.Spec.NamespaceAffinity.MatchLabels = nsLabel
				errupdate := common.UpdateIppool(frame, v4PoolObject)
				Expect(errupdate).NotTo(HaveOccurred(), "Failed to update v4PoolObject")
			}
			if frame.Info.IpV6Enabled {
				v6PoolObject := common.GetIppoolByName(frame, v6PoolName)
				v6PoolObject.Spec.NamespaceAffinity = new(v1.LabelSelector)
				v6PoolObject.Spec.NamespaceAffinity.MatchLabels = nsLabel
				errupdate := common.UpdateIppool(frame, v6PoolObject)
				Expect(errupdate).NotTo(HaveOccurred(), "Failed to update v6PoolObject")
			}

			// create 2 deployment1 in different ns
			depMap := map[string]string{
				deployName1: nsName1,
				deployName2: nsName2,
			}
			for d, deps := range depMap {
				common.CreateDeployUnitlReadyCheckInIppool(frame, deps, depMap[d], int32(2), v4PoolNameList, v6PoolNameList)

			}

			// try to delete deployment
			for d, deps := range depMap {
				Expect(frame.DeleteDeploymentUntilFinish(deps, depMap[d], time.Minute)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", deps, depMap[deps])
			}
		})
	})
})
