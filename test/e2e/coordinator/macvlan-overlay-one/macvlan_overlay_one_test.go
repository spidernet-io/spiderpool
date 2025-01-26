// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package macvlan_overlay_one_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	kdoctorV1beta1 "github.com/kdoctor-io/kdoctor/pkg/k8s/apis/kdoctor.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

var _ = Describe("MacvlanOverlayOne", Label("overlay", "one-nic", "coordinator"), func() {

	Context("In overlay mode with macvlan connectivity should be normal", func() {

		BeforeEach(func() {
			defer GinkgoRecover()
			var annotations = make(map[string]string)

			task = new(kdoctorV1beta1.NetReach)
			targetAgent = new(kdoctorV1beta1.NetReachTarget)
			request = new(kdoctorV1beta1.NetHttpRequest)
			netreach = new(kdoctorV1beta1.AgentSpec)
			schedule = new(kdoctorV1beta1.SchedulePlan)
			condition = new(kdoctorV1beta1.NetSuccessCondition)
			name = "one-macvlan-overlay-" + tools.RandomName()

			// Update netreach.agentSpec to generate test Pods using the macvlan
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, macvlanVlan0)
			netreach.Annotation = annotations
			netreach.HostNetwork = false
			GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
			task.Spec.AgentSpec = netreach
		})

		It("kdoctor connectivity should be succeed with no annotations", Serial, Label("C00002", "C00013"), func() {

			enable := true
			disable := false
			// create task kdoctor crd
			task.Name = name
			GinkgoWriter.Printf("Start the netreach task: %v", task.Name)
			// target
			targetAgent.Ingress = &disable
			targetAgent.Endpoint = &enable
			targetAgent.ClusterIP = &enable
			targetAgent.MultusInterface = &enable
			targetAgent.NodePort = &enable
			targetAgent.EnableLatencyMetric = true
			targetAgent.IPv4 = &frame.Info.IpV4Enabled
			if common.CheckCiliumFeatureOn() {
				// TODO(tao.yang), set testIPv6 to false, reference issue: https://github.com/spidernet-io/spiderpool/issues/2007
				targetAgent.IPv6 = &disable
			} else {
				targetAgent.IPv6 = &frame.Info.IpV6Enabled
			}

			GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
			task.Spec.Target = targetAgent

			// request
			request.DurationInSecond = 5
			request.QPS = 1
			request.PerRequestTimeoutInMS = 7000
			task.Spec.Request = request

			// Schedule
			crontab := "1 1"
			schedule.Schedule = &crontab
			schedule.RoundNumber = 1
			schedule.RoundTimeoutMinute = 1
			task.Spec.Schedule = schedule

			// success condition
			condition.SuccessRate = &successRate
			condition.MeanAccessDelayInMs = &delayMs
			task.Spec.SuccessCondition = condition

			taskCopy := task
			GinkgoWriter.Printf("kdoctor task: %+v \n", task)
			err := frame.CreateResource(task)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd create failed")

			err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd get failed")

			if frame.Info.IpV4Enabled {
				kdoctorIPv4ServiceName := fmt.Sprintf("%s-%s-ipv4", "kdoctor-netreach", task.Name)
				var kdoctorIPv4Service *corev1.Service
				Eventually(func() bool {
					kdoctorIPv4Service, err = frame.GetService(kdoctorIPv4ServiceName, "kube-system")
					if api_errors.IsNotFound(err) {
						return false
					}
					if err != nil {
						return false
					}
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
				kdoctorIPv4Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				kdoctorIPv4Service.Spec.Type = corev1.ServiceTypeNodePort
				Expect(frame.UpdateResource(kdoctorIPv4Service)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				kdoctorIPv6ServiceName := fmt.Sprintf("%s-%s-ipv6", "kdoctor-netreach", task.Name)
				var kdoctorIPv6Service *corev1.Service
				Eventually(func() bool {
					kdoctorIPv6Service, err = frame.GetService(kdoctorIPv6ServiceName, "kube-system")
					if api_errors.IsNotFound(err) {
						return false
					}
					if err != nil {
						return false
					}
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
				kdoctorIPv6Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				kdoctorIPv6Service.Spec.Type = corev1.ServiceTypeNodePort
				Expect(frame.UpdateResource(kdoctorIPv6Service)).NotTo(HaveOccurred())
			}

			// frame.GetService()
			ctx, cancel := context.WithTimeout(context.Background(), common.KdoctorCheckTime)
			defer cancel()
			for run {
				select {
				case <-ctx.Done():
					run = false
					Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running kdoctor task timeout")
				default:
					err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
					Expect(err).NotTo(HaveOccurred(), "kdoctor nethttp crd get failed, err is %v", err)

					if taskCopy.Status.Finish == true {
						command := fmt.Sprintf("get netreaches.kdoctor.io %s -oyaml", taskCopy.Name)
						netreachesLog, _ := frame.ExecKubectl(command, ctx)
						GinkgoWriter.Printf("kdoctor's netreaches execution result %+v \n", string(netreachesLog))

						for _, v := range taskCopy.Status.History {
							if v.Status != "succeed" {
								err = errors.New("error has occurred")
								run = false
							}
						}
						run = false

						ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
						defer cancel1()
						for {
							select {
							case <-ctx1.Done():
								Expect(errors.New("wait kdoctorreport timeout")).NotTo(HaveOccurred(), "failed to run kdoctor task and wait kdoctorreport timeout")
							default:
								command = fmt.Sprintf("get kdoctorreport %s -oyaml", taskCopy.Name)
								kdoctorreportLog, err := frame.ExecKubectl(command, ctx)
								if err != nil {
									time.Sleep(common.ForcedWaitingTime)
									continue
								}
								GinkgoWriter.Printf("kdoctor's kdoctorreport execution result %+v \n", string(kdoctorreportLog))
							}
							break
						}
					}
					time.Sleep(time.Second * 5)
				}
			}
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("In overlay mode with macvlan connectivity should be normal with annotations: ipam.spidernet.io/default-route-nic: net1", func() {
		BeforeEach(func() {
			defer GinkgoRecover()
			var annotations = make(map[string]string)

			task = new(kdoctorV1beta1.NetReach)
			targetAgent = new(kdoctorV1beta1.NetReachTarget)
			request = new(kdoctorV1beta1.NetHttpRequest)
			netreach = new(kdoctorV1beta1.AgentSpec)
			schedule = new(kdoctorV1beta1.SchedulePlan)
			condition = new(kdoctorV1beta1.NetSuccessCondition)
			name = "one-macvlan-overlay-" + tools.RandomName()

			// Update netreach.agentSpec to generate test Pods using the macvlan
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, macvlanVlan0)
			annotations[constant.AnnoDefaultRouteInterface] = "net1"
			netreach.Annotation = annotations
			netreach.HostNetwork = false
			GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
			task.Spec.AgentSpec = netreach
		})

		It("kdoctor connectivity should be succeed with annotations: ipam.spidernet.io/default-route-nic: net1", Serial, Label("C00020"), func() {

			enable := true
			disable := false
			// create task kdoctor crd
			task.Name = name
			GinkgoWriter.Printf("Start the netreach task: %v", task.Name)
			// target
			targetAgent.Ingress = &disable
			targetAgent.Endpoint = &enable
			targetAgent.ClusterIP = &enable
			targetAgent.MultusInterface = &enable
			targetAgent.NodePort = &enable
			targetAgent.EnableLatencyMetric = true
			targetAgent.IPv4 = &frame.Info.IpV4Enabled
			if common.CheckCiliumFeatureOn() {
				// TODO(tao.yang), set testIPv6 to false, reference issue: https://github.com/spidernet-io/spiderpool/issues/2007
				targetAgent.IPv6 = &disable
			} else {
				targetAgent.IPv6 = &frame.Info.IpV6Enabled
			}

			GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
			task.Spec.Target = targetAgent

			// request
			request.DurationInSecond = 5
			request.QPS = 1
			request.PerRequestTimeoutInMS = 7000
			task.Spec.Request = request

			// Schedule
			crontab := "1 1"
			schedule.Schedule = &crontab
			schedule.RoundNumber = 1
			schedule.RoundTimeoutMinute = 1
			task.Spec.Schedule = schedule

			// success condition
			condition.SuccessRate = &successRate
			condition.MeanAccessDelayInMs = &delayMs
			task.Spec.SuccessCondition = condition

			taskCopy := task
			GinkgoWriter.Printf("kdoctor task: %+v \n", task)
			err := frame.CreateResource(task)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd create failed")

			err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd get failed")

			if frame.Info.IpV4Enabled {
				kdoctorIPv4ServiceName := fmt.Sprintf("%s-%s-ipv4", "kdoctor-netreach", task.Name)
				var kdoctorIPv4Service *corev1.Service
				Eventually(func() bool {
					kdoctorIPv4Service, err = frame.GetService(kdoctorIPv4ServiceName, "kube-system")
					if api_errors.IsNotFound(err) {
						return false
					}
					if err != nil {
						return false
					}
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
				kdoctorIPv4Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				kdoctorIPv4Service.Spec.Type = corev1.ServiceTypeNodePort
				Expect(frame.UpdateResource(kdoctorIPv4Service)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				kdoctorIPv6ServiceName := fmt.Sprintf("%s-%s-ipv6", "kdoctor-netreach", task.Name)
				var kdoctorIPv6Service *corev1.Service
				Eventually(func() bool {
					kdoctorIPv6Service, err = frame.GetService(kdoctorIPv6ServiceName, "kube-system")
					if api_errors.IsNotFound(err) {
						return false
					}
					if err != nil {
						return false
					}
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
				kdoctorIPv6Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				kdoctorIPv6Service.Spec.Type = corev1.ServiceTypeNodePort
				Expect(frame.UpdateResource(kdoctorIPv6Service)).NotTo(HaveOccurred())
			}

			// frame.GetService()
			ctx, cancel := context.WithTimeout(context.Background(), common.KdoctorCheckTime)
			defer cancel()
			for run {
				select {
				case <-ctx.Done():
					run = false
					Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running kdoctor task timeout")
				default:
					err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
					Expect(err).NotTo(HaveOccurred(), "kdoctor nethttp crd get failed, err is %v", err)

					if taskCopy.Status.Finish == true {
						command := fmt.Sprintf("get netreaches.kdoctor.io %s -oyaml", taskCopy.Name)
						netreachesLog, _ := frame.ExecKubectl(command, ctx)
						GinkgoWriter.Printf("kdoctor's netreaches execution result %+v \n", string(netreachesLog))

						for _, v := range taskCopy.Status.History {
							if v.Status != "succeed" {
								err = errors.New("error has occurred")
								run = false
							}
						}
						run = false

						ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
						defer cancel1()
						for {
							select {
							case <-ctx1.Done():
								Expect(errors.New("wait kdoctorreport timeout")).NotTo(HaveOccurred(), "failed to run kdoctor task and wait kdoctorreport timeout")
							default:
								command = fmt.Sprintf("get kdoctorreport %s -oyaml", taskCopy.Name)
								kdoctorreportLog, err := frame.ExecKubectl(command, ctx)
								if err != nil {
									time.Sleep(common.ForcedWaitingTime)
									continue
								}
								GinkgoWriter.Printf("kdoctor's kdoctorreport execution result %+v \n", string(kdoctorreportLog))
							}
							break
						}
					}
					time.Sleep(time.Second * 5)
				}
			}
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("coordinator's gateway, macperfix's authentication", func() {
		var v4PoolName, v6PoolName, namespace, macPrefix, depName, mode, multusNadName string
		var v4Gateway, v6Gateway string
		var podCidrType string

		BeforeEach(func() {
			// generate some test data
			macPrefix = "0a:1b"
			mode = "overlay"
			namespace = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)
			multusNadName = "test-multus-" + common.GenerateString(10, true)
			podCidrType = "cluster"

			// create namespace and ippool
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
				if frame.Info.IpV4Enabled {
					v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
					gateway := strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
					v4PoolObj.Spec.Gateway = &gateway
					err = common.CreateIppool(frame, v4PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
					v4Gateway = *v4PoolObj.Spec.Gateway
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
					gateway := strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
					v6PoolObj.Spec.Gateway = &gateway
					err = common.CreateIppool(frame, v6PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
					v6Gateway = *v6PoolObj.Spec.Gateway
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						PodMACPrefix:       &macPrefix,
						PodDefaultRouteNIC: &common.NIC2,
						Mode:               &mode,
						PodCIDRType:        &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}
				GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, multusNadName)
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())

				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("delete v4 ippool %v. \n", v4PoolName)
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("delete v6 ippool %v. \n", v6PoolName)
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				}
			})
		})

		It("the prefix of the pod mac address should be overridden and the default route should be on the specified NIC", Label("C00006", "C00005", "C00008"), func() {
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{v6PoolName}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()

			depObject, err := frame.WaitDeploymentReady(depName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred(), "waiting for deploy ready failed:  %v ", err)
			podList, err := frame.GetPodListByLabel(depObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred(), "failed to get podList: %v ", err)

			// Check pod's mac address prefix
			commandString := fmt.Sprintf("ip link show dev %s | awk '/ether/ {print substr($2,0,5)}'", common.NIC2)
			ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			data, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, commandString, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)

			// C00008: the prefix of the pod mac address should be overridden.
			Expect(strings.TrimRight(string(data), "\n")).To(Equal(macPrefix), "macperfix is not covered, %s != %s", string(data), macPrefix)

			// Check the network card where the default route of the pod is located
			ipv4ServiceSubnet, ipv6ServiceSubnet := getClusterServiceSubnet()
			for _, pod := range podList.Items {
				if frame.Info.IpV4Enabled {
					ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()

					// In this use case, the default routing NIC is specified as net1 (originally the default is eth0) through `CoordinatorSpec.PodDefaultRouteNIC`
					// ip r get <address outside the cluster>, should flow out from the correct NIC(net1).
					GinkgoWriter.Println("ip -4 r get <address outside the cluster>")
					runGetIPString := "ip -4 r get '8.8.8.8' "
					executeCommandResult, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC2), "Expected NIC %v mismatch", common.NIC2)

					// ip r get <IP in eth0 subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -4 r get <IP in eth0 subnet>")
					runGetIPString = fmt.Sprintf("ip -4 r get %v ", ip.NextIP(net.ParseIP(pod.Status.PodIP)).String())
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)

					// ip r get <IP in net1 subnet>, should flow out from net1
					GinkgoWriter.Println("ip -4 r get <IP in net1 subnet>")
					net1IP, err := common.GetPodIPAddressFromIppool(frame, v4PoolName, pod.Namespace, pod.Name)
					Expect(err).NotTo(HaveOccurred(), "Failed to obtain Pod %v/%v IP address from ippool %v ", pod.Namespace, pod.Name, v4PoolName)
					runGetIPString = fmt.Sprintf("ip -4 r get %v ", ip.NextIP(net.ParseIP(net1IP)).String())
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC2), "Expected NIC %v mismatch", common.NIC2)

					// ip r get <IP in service subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -4 r get <IP in service subnet>")
					ips, err := common.GenerateIPs(ipv4ServiceSubnet, 1)
					Expect(err).NotTo(HaveOccurred(), "Failed to generate IPs from subnet %v ", ipv4ServiceSubnet)
					runGetIPString = fmt.Sprintf("ip -4 r get %v ", ips[0])
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)
				}
				if frame.Info.IpV6Enabled {
					ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()

					// In this use case, the default routing NIC is specified as net1 (originally the default is eth0) through `CoordinatorSpec.PodDefaultRouteNIC`
					// ip r get <address outside the cluster>, should flow out from the correct NIC(net1).
					GinkgoWriter.Println("ip -6 r get <IP in service subnet>")
					runGetIPString := "ip -6 r get '2401:2401::1' "
					executeCommandResult, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC2), "Expected NIC %v mismatch", common.NIC2)

					// ip r get <IP in eth0 subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -6 r get <IP in eth0 subnet>")
					if frame.Info.IpV4Enabled {
						// Dual stack
						runGetIPString = fmt.Sprintf("ip -6 r get %v ", ip.NextIP(net.ParseIP(pod.Status.PodIPs[1].IP)).String())
					} else {
						// IPv6
						runGetIPString = fmt.Sprintf("ip -6 r get %v ", ip.NextIP(net.ParseIP(pod.Status.PodIP)).String())
					}
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)

					// ip r get <IP in net1 subnet>, should flow out from net1
					GinkgoWriter.Println("ip -6 r get <IP in net1 subnet>")
					net1IP, err := common.GetPodIPAddressFromIppool(frame, v6PoolName, pod.Namespace, pod.Name)
					Expect(err).NotTo(HaveOccurred(), "Failed to obtain Pod %v/%v IP address from v6 ippool %v ", pod.Namespace, pod.Name, v6PoolName)
					runGetIPString = fmt.Sprintf("ip -6 r get %v ", ip.NextIP(net.ParseIP(net1IP)).String())
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC2), "Expected NIC %v mismatch", common.NIC2)

					// ip r get <IP in service subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -6 r get <IP in service subnet>")
					ips, err := common.GenerateIPs(ipv6ServiceSubnet, 1)
					Expect(err).NotTo(HaveOccurred(), "Failed to generate IPs from subnet %v ", ipv6ServiceSubnet)
					runGetIPString = fmt.Sprintf("ip -6 r get %v ", ips[0])
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)
				}
			}
		})

		// Add case V00007: spidercoordinator has the lowest priority here.
		// Gateway detection is turned off in the default spidercoodinator:default,
		// turned on in the new multus configuration and takes effect.
		// Therefore, verifying spidercoodinator has the lowest priority.
		PIt("gateway connection detection", Label("V00007", "C00009"), func() {
			detectGatewayMultusName := "test-gateway-multus-" + common.GenerateString(10, true)

			// Define multus cni NetworkAttachmentDefinition and set DetectGateway to true
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      detectGatewayMultusName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode:        &mode,
						PodCIDRType: &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			podAnno := types.AnnoPodIPPoolValue{}
			if frame.Info.IpV4Enabled {
				podAnno.IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podAnno.IPv6Pools = []string{v6PoolName}
			}
			podAnnoMarshal, err := json.Marshal(podAnno)
			Expect(err).NotTo(HaveOccurred())

			// multus cni configure detectGatewayMultusName detectGateway is true
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, detectGatewayMultusName)
			annotations[constant.AnnoPodIPPool] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podList, err := common.CreateDeployUntilExpectedReplicas(frame, deployObject, ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get the Pod creation failure Event
			var networkV4ErrString, networkV6ErrString string
			if frame.Info.IpV4Enabled {
				networkV4ErrString = fmt.Sprintf("gateway %v is unreachable", v4Gateway)
				GinkgoWriter.Printf("The v4 gateway detects an abnormal message %v \n", networkV4ErrString)
			}
			if frame.Info.IpV6Enabled {
				// The ipv6 address on the network interface will be abbreviated. For example fd00:0c3d::2 becomes fd00:c3d::2.
				// Abbreviate the expected ipv6 address and use it in subsequent assertions.
				v6GatewayNetIP := net.ParseIP(v6Gateway)
				networkV6ErrString = fmt.Sprintf("gateway %v is unreachable", v6GatewayNetIP.String())
				GinkgoWriter.Printf("The v6 gateway detects an abnormal message %v \n", networkV6ErrString)
			}

			for _, pod := range podList.Items {
				if frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
					ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
					defer cancel()
					err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, networkV4ErrString)
					if err != nil {
						ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
						defer cancel()
						GinkgoWriter.Printf("Failed to get v4 gateway unreachable event, trying to get v6 gateway, err is %v", err)
						err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, networkV6ErrString)
					}
					Expect(err).To(Succeed(), "Failed to get the event that the gateway is unreachable, error is: %v", err)
				}
				if frame.Info.IpV4Enabled && !frame.Info.IpV6Enabled {
					err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, networkV4ErrString)
					Expect(err).To(Succeed(), "Failed to get the event that the gateway is unreachable, error is: %v", err)
				}
				if !frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
					err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, networkV6ErrString)
					Expect(err).To(Succeed(), "Failed to get the event that the gateway is unreachable, error is: %v", err)
				}
			}

			GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, detectGatewayMultusName)
			Expect(frame.DeleteSpiderMultusInstance(namespace, detectGatewayMultusName)).NotTo(HaveOccurred())
		})
	})

	Context("ip conflict detection (ipv4, ipv6)", func() {
		var v4IpConflict, v6IpConflict string
		var depName, namespace, v4PoolName, v6PoolName, mode, podCidrType string
		var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
		var multusNadName = "test-multus-" + common.GenerateString(10, true)

		BeforeEach(func() {
			depName = "dep-name-" + common.GenerateString(10, true)
			namespace = "ns-" + common.GenerateString(10, true)
			mode = "overlay"
			podCidrType = "cluster"

			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode:        &mode,
						PodCIDRType: &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}
				GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, multusNadName)
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())

				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
			})
		})

		// Add case V00007: spidercoordinator has the lowest priority here.
		// ip conflict detection is turned off in the default spidercoodinator:default,
		// turned on in the new multus configuration and takes effect.
		// Therefore, verifying spidercoodinator has the lowest priority.
		PIt("It should be possible to detect ip conflicts and log output", Label("C00007", "V00007"), func() {
			podAnno := types.AnnoPodIPPoolValue{}

			var vlanID int32 = 200
			Eventually(func() error {
				var smc spiderpoolv2beta1.SpiderMultusConfig
				err := frame.KClient.Get(context.TODO(), apitypes.NamespacedName{
					Namespace: namespace,
					Name:      multusNadName,
				}, &smc)
				if nil != err {
					return err
				}

				Expect(smc.Spec.MacvlanConfig).NotTo(BeNil())
				smc.Spec.MacvlanConfig.VlanID = ptr.To(vlanID)

				err = frame.KClient.Update(context.TODO(), &smc)
				if nil != err {
					return err
				}
				GinkgoWriter.Printf("update SpiderMultusConfig %s/%s with vlanID %d successfully\n", namespace, multusNadName, vlanID)
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeNil())

			if frame.Info.IpV4Enabled {
				spiderPoolIPv4SubnetVlan200, err := common.GetIppoolByName(frame, common.SpiderPoolIPv4SubnetVlan200)
				Expect(err).NotTo(HaveOccurred(), "failed to get v4 ippool, error is %v", err)

				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
				v4PoolObj.Spec.Subnet = spiderPoolIPv4SubnetVlan200.Spec.Subnet
				v4PoolObj.Spec.Gateway = spiderPoolIPv4SubnetVlan200.Spec.Gateway

				// Do not use the gateway address as the conflicting ip, and the use case will remove the IP address in the subsequent steps.
				// If the gateway address is removed, it will affect the connectivity of other use cases.
				var v4RandNum int
				Eventually(func() bool {
					v4RandNum = r.Intn(99) + 1
					v4Ips := strings.Split(spiderPoolIPv4SubnetVlan200.Spec.Subnet, "0/")[0] + strconv.Itoa(v4RandNum)
					return v4Ips != *spiderPoolIPv4SubnetVlan200.Spec.Gateway
				}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
				v4PoolObj.Spec.IPs = []string{strings.Split(spiderPoolIPv4SubnetVlan200.Spec.Subnet, "0/")[0] + strconv.Itoa(v4RandNum)}
				v4IpConflict = v4PoolObj.Spec.IPs[0]
				err = common.CreateIppool(frame, v4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "failed to create v4 ippool, error is %v", err)

				// Add a conflicting v4 IP to the cluster
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				addV4IpCmdString := fmt.Sprintf("ip a add %s/%v dev %s", v4IpConflict, strings.Split(v4PoolObj.Spec.Subnet, "/")[1], common.NIC4)
				GinkgoWriter.Println("add v4 ip conflict command: ", addV4IpCmdString)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, addV4IpCmdString)
				Expect(err).NotTo(HaveOccurred(), "Failed to add v4 conflicting ip of docker container, error is: %v,log: %v.", err, string(output))
				podAnno.IPv4Pools = []string{v4PoolName}
			}

			if frame.Info.IpV6Enabled {
				spiderPoolIPv6SubnetVlan200, err := common.GetIppoolByName(frame, common.SpiderPoolIPv6SubnetVlan200)
				Expect(err).NotTo(HaveOccurred(), "failed to get v6 ippool, error is %v", err)

				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
				v6PoolObj.Spec.Subnet = spiderPoolIPv6SubnetVlan200.Spec.Subnet
				v6PoolObj.Spec.Gateway = spiderPoolIPv6SubnetVlan200.Spec.Gateway
				// Do not use the gateway address as the conflicting ip, and the use case will remove the IP address in the subsequent steps.
				// If the gateway address is removed, it will affect the connectivity of other use cases.
				var v6RandNum int
				Eventually(func() bool {
					v6RandNum = r.Intn(99) + 1
					v6Ips := strings.Split(spiderPoolIPv6SubnetVlan200.Spec.Subnet, "/")[0] + strconv.Itoa(v6RandNum)
					return v6Ips != *spiderPoolIPv6SubnetVlan200.Spec.Gateway
				}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
				v6PoolObj.Spec.IPs = []string{strings.Split(spiderPoolIPv6SubnetVlan200.Spec.Subnet, "/")[0] + strconv.Itoa(v6RandNum)}
				v6IpConflict = v6PoolObj.Spec.IPs[0]
				err = common.CreateIppool(frame, v6PoolObj)
				Expect(err).NotTo(HaveOccurred(), "failed to create v6 ippool, error is %v", err)

				// Add a conflicting v6 IP to the cluster
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				addV6IpCmdString := fmt.Sprintf("ip a add %s/%v dev %s", v6IpConflict, strings.Split(v6PoolObj.Spec.Subnet, "/")[1], common.NIC4)
				GinkgoWriter.Println("add v6 ip conflict command: ", addV6IpCmdString)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, addV6IpCmdString)
				Expect(err).NotTo(HaveOccurred(), "Failed to add v6 conflicting ip of docker container, error is: %v, log: %v.", err, string(output))
				podAnno.IPv6Pools = []string{v6PoolName}
				// Avoid code creating pods too fast, which will be detected by ipv6 addresses dad failed.
				time.Sleep(time.Second * 10)
			}
			podAnnoMarshal, err := json.Marshal(podAnno)
			Expect(err).NotTo(HaveOccurred())

			// multus cni configure MacvlanUnderlayVlan200 ip_conflict is true
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPool] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podList, err := common.CreateDeployUntilExpectedReplicas(frame, deployObject, ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get the Pod creation failure Event
			v4NetworkErrString := fmt.Sprintf("pod's interface %v with an conflicting ip %s", common.NIC2, v4IpConflict)
			v6NetworkErrString := fmt.Sprintf("pod's interface %v with an conflicting ip %s", common.NIC2, v6IpConflict)
			if frame.Info.IpV4Enabled {
				ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
				defer cancel()
				for _, pod := range podList.Items {
					err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, v4NetworkErrString)
					if err != nil {
						ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
						defer cancel()
						err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, v6NetworkErrString)
					}
					Expect(err).To(Succeed(), "Failed to get IP conflict event, error is: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				addV4IpCmdString := fmt.Sprintf("ip a del %s/%v dev %s", v4IpConflict, strings.Split(v4PoolObj.Spec.Subnet, "/")[1], common.NIC4)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, addV4IpCmdString)
				Expect(err).NotTo(HaveOccurred(), "Failed to del v4 conflicting IP of docker container, error is: %v,log: %v.", err, string(output))

				// In a dual-stack environment, v4 IP does not conflict, and v6 IP conflicts can be detected correctly
				if frame.Info.IpV6Enabled {
					ctx, cancel = context.WithTimeout(context.Background(), 3*common.EventOccurTimeout)
					defer cancel()
					for _, pod := range podList.Items {
						err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, v6NetworkErrString)
						Expect(err).To(Succeed(), "Failed to get IP conflict event, error is: %v", err)
					}

					ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					addV6IpCmdString := fmt.Sprintf("ip a del %s/%v dev %s", v6IpConflict, strings.Split(v6PoolObj.Spec.Subnet, "/")[1], common.NIC4)
					output, err = frame.DockerExecCommand(ctx, common.VlanGatewayContainer, addV6IpCmdString)
					Expect(err).NotTo(HaveOccurred(), "Failed to del v6 conflicting IP of docker container, error is: %v,log: %v.", err, string(output))
				}
			}

			// In a single ipv6 environment, conflicting IPv6 addresses can be checked.
			if !frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
				ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
				defer cancel()
				for _, pod := range podList.Items {
					err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, v6NetworkErrString)
					Expect(err).To(Succeed(), "Failed to get IP conflict event, error is: %v", err)
				}

				ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				addV6IpCmdString := fmt.Sprintf("ip a del %s/%v dev %s", v6IpConflict, strings.Split(v6PoolObj.Spec.Subnet, "/")[1], common.NIC4)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, addV6IpCmdString)
				Expect(err).NotTo(HaveOccurred(), "Failed to del v6 conflicting ip of docker container, error is: %v,log: %v.", err, string(output))
			}

			// After there is no conflicting IP, the pod can run normally.
			GinkgoWriter.Printf("After there are no conflicting IPs, restart the pod and wait for the pod to run.")
			Expect(frame.DeletePodList(podList)).NotTo(HaveOccurred())
			Eventually(func() bool {
				newPodList, err := frame.GetPodListByLabel(deployObject.Spec.Template.Labels)
				if err != nil && len(newPodList.Items) == int(*deployObject.Spec.Replicas) {
					return false
				}

				return frame.CheckPodListRunning(newPodList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})

		It("The conflict IPs for stateless Pod should be released, and the conflict IPs for stateful Pod should not be released", Label("C00018", "C00019"), func() {
			ctx := context.TODO()

			// 1. check the spiderpool-agent ENV SPIDERPOOL_ENABLED_RELEASE_CONFLICT_IPS enabled or missed
			const SPIDERPOOL_ENABLED_RELEASE_CONFLICT_IPS = "SPIDERPOOL_ENABLED_RELEASE_CONFLICT_IPS"
			spiderpoolAgentDS, err := frame.GetDaemonSet(constant.SpiderpoolAgent, "kube-system")
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderpoolAgentDS.Spec.Template.Spec.Containers).To(HaveLen(2))

			// the release conflicted IPs feature is default to be true if we do not set the ENV
			isReleaseConflictIPs := true
			for _, env := range spiderpoolAgentDS.Spec.Template.Spec.Containers[0].Env {
				if env.Name == SPIDERPOOL_ENABLED_RELEASE_CONFLICT_IPS {
					parseBool, err := strconv.ParseBool(env.Value)
					Expect(err).NotTo(HaveOccurred())
					isReleaseConflictIPs = parseBool
					break
				}
			}

			if !isReleaseConflictIPs {
				Skip("release conflicted IPs feature is disabled, skip this e2e case")
			}

			podAnno := types.AnnoPodIPPoolValue{}
			// 2. create an IPPool with conflicted IPs
			var conflictV4Pool spiderpoolv2beta1.SpiderIPPool
			var firstConflictV4IP string
			if frame.Info.IpV4Enabled {
				conflictV4PoolName := "conflict-v4-pool"
				spiderPoolIPv4PoolDefault, err := common.GetIppoolByName(frame, common.SpiderPoolIPv4PoolDefault)
				Expect(err).NotTo(HaveOccurred(), "failed to get ippool %s, error is %v", common.SpiderPoolIPv4PoolDefault, err)
				firstDefaultV4IP := strings.Split(spiderPoolIPv4PoolDefault.Spec.IPs[0], "-")[0]
				ipv4Prefix, found := strings.CutSuffix(firstDefaultV4IP, ".40.2")
				Expect(found).To(BeTrue())
				conflictV4IPs := fmt.Sprintf("%s.41.2-%s.41.4", ipv4Prefix, ipv4Prefix)
				GinkgoWriter.Printf("Generate conflict IPv4 IPs: %s\n", conflictV4IPs)
				firstConflictV4IP = strings.Split(conflictV4IPs, "-")[0]

				Eventually(func() error {
					if !frame.Info.SpiderSubnetEnabled {
						return nil
					}

					var v4Subnet spiderpoolv2beta1.SpiderSubnet
					err := frame.KClient.Get(ctx, apitypes.NamespacedName{Name: common.SpiderPoolIPv4SubnetDefault}, &v4Subnet)
					if nil != err {
						if api_errors.IsNotFound(err) {
							return nil
						}
						return err
					}
					GinkgoWriter.Printf("try to add IP %s to SpiderSubnet %s\n", firstConflictV4IP, common.SpiderPoolIPv4SubnetDefault)
					v4Subnet.Spec.IPs = append(v4Subnet.Spec.IPs, firstConflictV4IP)
					err = frame.KClient.Update(ctx, &v4Subnet)
					if nil != err {
						return err
					}
					return nil
				}).WithTimeout(time.Minute * 2).WithPolling(time.Second).Should(BeNil())

				conflictV4Pool.Name = conflictV4PoolName
				conflictV4Pool.Spec.Subnet = spiderPoolIPv4PoolDefault.Spec.Subnet
				conflictV4Pool.Spec.Gateway = spiderPoolIPv4PoolDefault.Spec.Gateway
				conflictV4Pool.Spec.IPs = []string{conflictV4IPs}
				err = frame.KClient.Create(ctx, &conflictV4Pool)
				Expect(err).NotTo(HaveOccurred())

				// set an IP address for NIC to mock IP conflict
				commandV4Str := fmt.Sprintf("ip addr add %s dev eth0", firstConflictV4IP)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, commandV4Str)
				Expect(err).NotTo(HaveOccurred(), "Failed to exec %s for Node %s, error is: %v, log: %v", commandV4Str, common.VlanGatewayContainer, err, string(output))

				podAnno.IPv4Pools = []string{conflictV4PoolName}
			}

			var conflictV6Pool spiderpoolv2beta1.SpiderIPPool
			var firstConflictV6IP string
			if frame.Info.IpV6Enabled {
				conflictV6PoolName := "conflict-v6-pool"

				spiderPoolIPv6PoolDefault, err := common.GetIppoolByName(frame, common.SpiderPoolIPv6PoolDefault)
				Expect(err).NotTo(HaveOccurred(), "failed to get ippool %s, error is %v", common.SpiderPoolIPv6PoolDefault, err)
				firstDefaultV6IP := strings.Split(spiderPoolIPv6PoolDefault.Spec.IPs[0], "-")[0]
				ipv6Prefix, found := strings.CutSuffix(firstDefaultV6IP, ":f::2")
				Expect(found).To(BeTrue())
				conflictV6IPs := fmt.Sprintf("%s:e::2-%s:e::4", ipv6Prefix, ipv6Prefix)
				GinkgoWriter.Printf("Generate conflict IPv6 IPs: %s\n", conflictV6IPs)
				firstConflictV6IP = strings.Split(conflictV6IPs, "-")[0]

				Eventually(func() error {
					if !frame.Info.SpiderSubnetEnabled {
						return nil
					}

					var v6Subnet spiderpoolv2beta1.SpiderSubnet
					err := frame.KClient.Get(ctx, apitypes.NamespacedName{Name: common.SpiderPoolIPv6SubnetDefault}, &v6Subnet)
					if nil != err {
						if api_errors.IsNotFound(err) {
							return nil
						}
						return err
					}
					GinkgoWriter.Printf("try to add IP %s to SpiderSubnet %s\n", firstConflictV6IP, common.SpiderPoolIPv4SubnetDefault)
					v6Subnet.Spec.IPs = append(v6Subnet.Spec.IPs, firstConflictV6IP)
					err = frame.KClient.Update(ctx, &v6Subnet)
					if nil != err {
						return err
					}

					return nil
				}).WithTimeout(time.Minute * 2).WithPolling(time.Second).Should(BeNil())

				conflictV6Pool.Name = conflictV6PoolName
				conflictV6Pool.Spec.Subnet = spiderPoolIPv6PoolDefault.Spec.Subnet
				conflictV6Pool.Spec.Gateway = spiderPoolIPv6PoolDefault.Spec.Gateway
				conflictV6Pool.Spec.IPs = []string{conflictV6IPs}
				err = frame.KClient.Create(ctx, &conflictV6Pool)
				Expect(err).NotTo(HaveOccurred())

				// set an IP address for NIC to mock IP conflict
				commandV6Str := fmt.Sprintf("ip addr add %s dev eth0", firstConflictV6IP)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, commandV6Str)
				Expect(err).NotTo(HaveOccurred(), "Failed to exec %s for Node %s, error is: %v, log: %v", commandV6Str, common.VlanGatewayContainer, err, string(output))

				podAnno.IPv6Pools = []string{conflictV6PoolName}
			}

			podAnnoMarshal, err := json.Marshal(podAnno)
			Expect(err).NotTo(HaveOccurred())

			// 3. create a pod with conflicted IPs
			anno := make(map[string]string)
			anno[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			anno[constant.AnnoPodIPPool] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = anno
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			GinkgoWriter.Println("try to create Pod with conflicted IPs IPPool")
			_, err = common.CreateDeployUntilExpectedReplicas(frame, deployObject, ctx)
			Expect(err).NotTo(HaveOccurred())

			// 4. delete the Deployments
			GinkgoWriter.Println("The Pod finally runs, task done.")
			Expect(frame.DeleteDeploymentUntilFinish(depName, namespace, time.Minute*3)).NotTo(HaveOccurred())

			// 5. create a StatefulSet with conflict IPs and it won't set up successfully.
			stsName := "sts-name-" + common.GenerateString(10, true)
			statefulSetObj := common.GenerateExampleStatefulSetYaml(stsName, namespace, 1)
			statefulSetObj.Spec.Template.Annotations = anno
			err = frame.CreateResource(statefulSetObj)
			Expect(err).NotTo(HaveOccurred())

			// 6. if we meet the conflict IP, the pod won't be set up finally.
			listLabels := &client.ListOptions{
				Raw: &v1.ListOptions{
					TypeMeta:      v1.TypeMeta{Kind: common.OwnerPod},
					FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", stsName+"-0", namespace),
				},
			}
			watchInterface, err := frame.KClient.Watch(context.TODO(), &corev1.EventList{}, listLabels)
			Expect(err).NotTo(HaveOccurred())
			defer watchInterface.Stop()
			tick := time.Tick(time.Minute * 2)

			hasConflictIPs := false
		END:
			for {
				select {
				case <-tick:
					GinkgoWriter.Println("no conflicted IPs found, just skip it")
					break END
				case watchEvent := <-watchInterface.ResultChan():
					event := watchEvent.Object.(*corev1.Event)
					if strings.Contains(event.Message, constant.ErrIPConflict.Error()) {
						hasConflictIPs = true
						GinkgoWriter.Printf("meet the conflicted IPs, the Pod message: %s\n", event.Message)
						break END
					}
				}
			}

			// the pod would not start
			if hasConflictIPs {
				for i := 0; i < 10; i++ {
					statefulSet, err := frame.GetStatefulSet(stsName, namespace)
					Expect(err).NotTo(HaveOccurred())
					Expect(statefulSet.Status.ReadyReplicas).To(BeZero())
					time.Sleep(time.Second * 6)
				}
			}

			// 7. delete the statefulSet
			err = frame.DeleteStatefulSet(stsName, namespace)
			Expect(err).NotTo(HaveOccurred())

			// 8. delete the conflict IPPools
			if frame.Info.IpV4Enabled {
				err := frame.KClient.Delete(ctx, &conflictV4Pool)
				Expect(err).NotTo(HaveOccurred())

				commandV4Str := fmt.Sprintf("ip addr del %s dev eth0", firstConflictV4IP)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, commandV4Str)
				Expect(err).NotTo(HaveOccurred(), "Failed to exec %s for Node %s, error is: %v, log: %v", commandV4Str, common.VlanGatewayContainer, err, string(output))
			}
			if frame.Info.IpV6Enabled {
				err := frame.KClient.Delete(ctx, &conflictV6Pool)
				Expect(err).NotTo(HaveOccurred())

				commandV6Str := fmt.Sprintf("ip addr del %s dev eth0", firstConflictV6IP)
				output, err := frame.DockerExecCommand(ctx, common.VlanGatewayContainer, commandV6Str)
				Expect(err).NotTo(HaveOccurred(), "Failed to exec %s for Node %s, error is: %v, log: %v", commandV6Str, common.VlanGatewayContainer, err, string(output))
			}
		})
	})

	Context("Test ip rule and default route are as expected.", func() {
		var v4PoolName, v6PoolName, namespace, depName, mode, multusNadName string
		var podCidrType string

		BeforeEach(func() {
			// generate some test data
			mode = "overlay"
			namespace = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)
			multusNadName = "test-multus-" + common.GenerateString(10, true)
			podCidrType = "cluster"

			// create namespace and ippool
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
				if frame.Info.IpV4Enabled {
					v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
					gateway := strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
					v4PoolObj.Spec.Gateway = &gateway
					err = common.CreateIppool(frame, v4PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
					gateway := strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
					v6PoolObj.Spec.Gateway = &gateway
					err = common.CreateIppool(frame, v6PoolObj)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode:        &mode,
						PodCIDRType: &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, multusNadName)
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())

				GinkgoWriter.Println("delete namespace: ", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

				if frame.Info.IpV4Enabled {
					GinkgoWriter.Println("delete v4 ippool: ", v4PoolName)
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Println("delete v6 ippool: ", v6PoolName)
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				}
			})
		})

		It("In the default scenario, the `ip rules` should be as expected and the default route should be on eth0", Label("C00011", "C00012"), func() {
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{v6PoolName}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)

			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			depObject, err := frame.WaitDeploymentReady(depName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred(), "waiting for deploy ready failed, error is: %v ", err)
			podList, err := frame.GetPodListByLabel(depObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred(), "failed to get podList, error is: %v ", err)

			// Check the ip rule in the pod and it should be as expected.
			ipv4ServiceSubnet, ipv6ServiceSubnet := getClusterServiceSubnet()
			for _, pod := range podList.Items {
				if frame.Info.IpV4Enabled {
					ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()

					// In the conventional multi-card situation, the NIC where the default route is located is not specified through comments or other methods.
					// Then when accessing the external address of the cluster, it should flow out from the eth0 NIC.
					// ip r get <address outside the cluster>, should flow out from the correct NIC(eth0).
					GinkgoWriter.Println("ip -4 r get <address outside the cluster>")
					runGetIPString := "ip -4 r get '8.8.8.8' "
					executeCommandResult, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)

					// ip r get <IP in eth0 subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -4 r get <IP in eth0 subnet>")
					runGetIPString = fmt.Sprintf("ip -4 r get %v ", ip.NextIP(net.ParseIP(pod.Status.PodIP)).String())
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)

					// ip r get <IP in net1 subnet>, should flow out from net1
					GinkgoWriter.Println("ip -4 r get <IP in net1 subnet>")
					net1IP, err := common.GetPodIPAddressFromIppool(frame, v4PoolName, pod.Namespace, pod.Name)
					Expect(err).NotTo(HaveOccurred(), "Failed to obtain Pod %v/%v IP address from ippool %v ", pod.Namespace, pod.Name, v4PoolName)
					runGetIPString = fmt.Sprintf("ip -4 r get %v ", ip.NextIP(net.ParseIP(net1IP)).String())
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC2), "Expected NIC %v mismatch", common.NIC2)

					// ip r get <IP in service subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -4 r get <IP in service subnet>")
					ips, err := common.GenerateIPs(ipv4ServiceSubnet, 1)
					Expect(err).NotTo(HaveOccurred(), "Failed to generate IPs from subnet %v ", ipv4ServiceSubnet)
					runGetIPString = fmt.Sprintf("ip -4 r get %v ", ips[0])
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)
				}
				if frame.Info.IpV6Enabled {
					ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()

					// In the conventional multi-card situation, the NIC where the default route is located is not specified through comments or other methods.
					// Then when accessing the external address of the cluster, it should flow out from the eth0 NIC.
					// ip r get <address outside the cluster>, should flow out from the correct NIC(eth0).
					GinkgoWriter.Println("ip -6 r get <address outside the cluster>")
					runGetIPString := "ip -6 r get '2401:2401::1' "
					executeCommandResult, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)

					// ip r get <IP in eth0 subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -6 r get <IP in eth0 subnet>")
					if frame.Info.IpV4Enabled {
						// Dual stack
						runGetIPString = fmt.Sprintf("ip r get %v ", ip.NextIP(net.ParseIP(pod.Status.PodIPs[1].IP)).String())
					} else {
						// IPv6
						runGetIPString = fmt.Sprintf("ip r get %v ", ip.NextIP(net.ParseIP(pod.Status.PodIP)).String())
					}
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)

					// ip r get <IP in net1 subnet>, should flow out from net1
					GinkgoWriter.Println("ip -6 r get <IP in net1 subnet>")
					net1IP, err := common.GetPodIPAddressFromIppool(frame, v6PoolName, pod.Namespace, pod.Name)
					Expect(err).NotTo(HaveOccurred(), "Failed to obtain Pod %v/%v IP address from v6 ippool %v ", pod.Namespace, pod.Name, v6PoolName)
					runGetIPString = fmt.Sprintf("ip -6 r get %v ", ip.NextIP(net.ParseIP(net1IP)).String())
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC2), "Expected NIC %v mismatch", common.NIC2)

					// ip r get <IP in service subnet>, should flow out from eth0
					GinkgoWriter.Println("ip -6 r get <IP in service subnet>")
					ips, err := common.GenerateIPs(ipv6ServiceSubnet, 1)
					Expect(err).NotTo(HaveOccurred(), "Failed to generate IPs from subnet %v ", ipv6ServiceSubnet)
					runGetIPString = fmt.Sprintf("ip -6 r get %v ", ips[0])
					executeCommandResult, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPString, ctx)
					GinkgoWriter.Println("Execute ipv6 command result: ", string(executeCommandResult))
					Expect(err).NotTo(HaveOccurred(), "failed to execute ipv6 command, error is: %v ", err)
					Expect(string(executeCommandResult)).Should(ContainSubstring(common.NIC1), "Expected NIC %v mismatch", common.NIC1)
				}
			}
		})
	})

	Context("In overlay mode with two macvlan CNI networks", func() {

		BeforeEach(func() {
			defer GinkgoRecover()
			var annotations = make(map[string]string)

			task = new(kdoctorV1beta1.NetReach)
			targetAgent = new(kdoctorV1beta1.NetReachTarget)
			request = new(kdoctorV1beta1.NetHttpRequest)
			netreach = new(kdoctorV1beta1.AgentSpec)
			schedule = new(kdoctorV1beta1.SchedulePlan)
			condition = new(kdoctorV1beta1.NetSuccessCondition)
			name = "two-macvlan-overlay-" + tools.RandomName()

			// Update netreach.agentSpec to generate test Pods using the macvlan
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s,%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0, common.MultusNs, common.MacvlanVlan100)
			if frame.Info.SpiderSubnetEnabled {
				subnetsAnno := []types.AnnoSubnetItem{
					{
						Interface: common.NIC2,
					},
					{
						Interface: common.NIC6,
					},
				}
				if frame.Info.IpV4Enabled {
					subnetsAnno[0].IPv4 = []string{common.SpiderPoolIPv4SubnetDefault}
					subnetsAnno[1].IPv4 = []string{common.SpiderPoolIPv4SubnetVlan100}
				}
				if frame.Info.IpV6Enabled {
					subnetsAnno[0].IPv6 = []string{common.SpiderPoolIPv6SubnetDefault}
					subnetsAnno[1].IPv6 = []string{common.SpiderPoolIPv6SubnetVlan100}
				}
				subnetsAnnoMarshal, err := json.Marshal(subnetsAnno)
				Expect(err).NotTo(HaveOccurred())
				annotations[pkgconstant.AnnoSpiderSubnets] = string(subnetsAnnoMarshal)
			}
			netreach.Annotation = annotations
			netreach.HostNetwork = false
			task.Spec.AgentSpec = netreach
		})

		It("kdoctor connectivity should be succeed", Serial, Label("C00004"), func() {

			enable := true
			disable := false
			// create task kdoctor crd
			task.Name = name
			GinkgoWriter.Printf("Start the netreach task: %v", task.Name)
			// target
			targetAgent.Ingress = &disable
			targetAgent.Endpoint = &enable
			targetAgent.ClusterIP = &enable
			targetAgent.MultusInterface = &frame.Info.MultusEnabled
			targetAgent.NodePort = &enable
			targetAgent.EnableLatencyMetric = true
			targetAgent.IPv4 = &frame.Info.IpV4Enabled
			if common.CheckCiliumFeatureOn() {
				targetAgent.IPv6 = &disable
			} else {
				targetAgent.IPv6 = &frame.Info.IpV6Enabled
			}

			GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
			task.Spec.Target = targetAgent

			// request
			request.DurationInSecond = 5
			request.QPS = 1
			request.PerRequestTimeoutInMS = 7000
			task.Spec.Request = request

			// Schedule
			crontab := "1 1"
			schedule.Schedule = &crontab
			schedule.RoundNumber = 1
			schedule.RoundTimeoutMinute = 1
			task.Spec.Schedule = schedule

			// success condition
			condition.SuccessRate = &successRate
			condition.MeanAccessDelayInMs = &delayMs
			task.Spec.SuccessCondition = condition

			taskCopy := task
			GinkgoWriter.Printf("kdoctor task: %+v \n", task)
			err := frame.CreateResource(task)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd create failed")

			err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd get failed")

			ctx, cancel := context.WithTimeout(context.Background(), common.KdoctorCheckTime)
			defer cancel()
			for run {
				select {
				case <-ctx.Done():
					run = false
					Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running kdoctor task timeout")
				default:
					err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
					Expect(err).NotTo(HaveOccurred(), "kdoctor nethttp crd get failed, err is %v", err)

					if taskCopy.Status.Finish == true {
						command := fmt.Sprintf("get netreaches.kdoctor.io %s -oyaml", taskCopy.Name)
						netreachesLog, _ := frame.ExecKubectl(command, ctx)
						GinkgoWriter.Printf("kdoctor's netreaches execution result %+v \n", string(netreachesLog))

						for _, v := range taskCopy.Status.History {
							if v.Status != "succeed" {
								err = errors.New("error has occurred")
								run = false
							}
						}
						run = false

						ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
						defer cancel1()
						for {
							select {
							case <-ctx1.Done():
								Expect(errors.New("wait kdoctorreport timeout")).NotTo(HaveOccurred(), "failed to run kdoctor task and wait kdoctorreport timeout")
							default:
								command = fmt.Sprintf("get kdoctorreport %s -oyaml", taskCopy.Name)
								kdoctorreportLog, err := frame.ExecKubectl(command, ctx)
								if err != nil {
									time.Sleep(common.ForcedWaitingTime)
									continue
								}
								GinkgoWriter.Printf("kdoctor's kdoctorreport execution result %+v \n", string(kdoctorreportLog))
							}
							break
						}
					}
					time.Sleep(time.Second * 5)
				}
			}
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("In overlay mode with three macvlan interfaces, connectivity should be normal", func() {
		BeforeEach(func() {
			defer GinkgoRecover()
			var annotations = make(map[string]string)

			task = new(kdoctorV1beta1.NetReach)
			targetAgent = new(kdoctorV1beta1.NetReachTarget)
			request = new(kdoctorV1beta1.NetHttpRequest)
			netreach = new(kdoctorV1beta1.AgentSpec)
			schedule = new(kdoctorV1beta1.SchedulePlan)
			condition = new(kdoctorV1beta1.NetSuccessCondition)
			name = "one-macvlan-overlay-" + tools.RandomName()

			// Update netreach.agentSpec to generate test Pods using the macvlan
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s,%s/%s,%s/%s", common.MultusNs, macvlanVlan0, common.MultusNs, macvlanVlan100, common.MultusNs, macvlanVlan200)
			netreach.Annotation = annotations
			netreach.HostNetwork = false
			GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
			task.Spec.AgentSpec = netreach
		})

		It("kdoctor connectivity should be succeed with three macavlan interfaces", Serial, Label("C00021"), func() {

			enable := true
			disable := false
			// create task kdoctor crd
			task.Name = name
			GinkgoWriter.Printf("Start the netreach task: %v", task.Name)
			// target
			targetAgent.Ingress = &disable
			targetAgent.Endpoint = &enable
			targetAgent.ClusterIP = &enable
			targetAgent.MultusInterface = &enable
			targetAgent.NodePort = &enable
			targetAgent.EnableLatencyMetric = true
			targetAgent.IPv4 = &frame.Info.IpV4Enabled
			if common.CheckCiliumFeatureOn() {
				// TODO(tao.yang), set testIPv6 to false, reference issue: https://github.com/spidernet-io/spiderpool/issues/2007
				targetAgent.IPv6 = &disable
			} else {
				targetAgent.IPv6 = &frame.Info.IpV6Enabled
			}

			GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
			task.Spec.Target = targetAgent

			// request
			request.DurationInSecond = 5
			request.QPS = 1
			request.PerRequestTimeoutInMS = 7000
			task.Spec.Request = request

			// Schedule
			crontab := "1 1"
			schedule.Schedule = &crontab
			schedule.RoundNumber = 1
			schedule.RoundTimeoutMinute = 1
			task.Spec.Schedule = schedule

			// success condition
			condition.SuccessRate = &successRate
			condition.MeanAccessDelayInMs = &delayMs
			task.Spec.SuccessCondition = condition

			taskCopy := task
			GinkgoWriter.Printf("kdoctor task: %+v \n", task)
			err := frame.CreateResource(task)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd create failed")

			err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
			Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd get failed")

			if frame.Info.IpV4Enabled {
				kdoctorIPv4ServiceName := fmt.Sprintf("%s-%s-ipv4", "kdoctor-netreach", task.Name)
				var kdoctorIPv4Service *corev1.Service
				Eventually(func() bool {
					kdoctorIPv4Service, err = frame.GetService(kdoctorIPv4ServiceName, "kube-system")
					if api_errors.IsNotFound(err) {
						return false
					}
					if err != nil {
						return false
					}
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
				kdoctorIPv4Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				kdoctorIPv4Service.Spec.Type = corev1.ServiceTypeNodePort
				Expect(frame.UpdateResource(kdoctorIPv4Service)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				kdoctorIPv6ServiceName := fmt.Sprintf("%s-%s-ipv6", "kdoctor-netreach", task.Name)
				var kdoctorIPv6Service *corev1.Service
				Eventually(func() bool {
					kdoctorIPv6Service, err = frame.GetService(kdoctorIPv6ServiceName, "kube-system")
					if api_errors.IsNotFound(err) {
						return false
					}
					if err != nil {
						return false
					}
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
				kdoctorIPv6Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				kdoctorIPv6Service.Spec.Type = corev1.ServiceTypeNodePort
				Expect(frame.UpdateResource(kdoctorIPv6Service)).NotTo(HaveOccurred())
			}

			// frame.GetService()
			ctx, cancel := context.WithTimeout(context.Background(), common.KdoctorCheckTime)
			defer cancel()
			for run {
				select {
				case <-ctx.Done():
					run = false
					Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running kdoctor task timeout")
				default:
					err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
					Expect(err).NotTo(HaveOccurred(), "kdoctor nethttp crd get failed, err is %v", err)

					if taskCopy.Status.Finish == true {
						command := fmt.Sprintf("get netreaches.kdoctor.io %s -oyaml", taskCopy.Name)
						netreachesLog, _ := frame.ExecKubectl(command, ctx)
						GinkgoWriter.Printf("kdoctor's netreaches execution result %+v \n", string(netreachesLog))

						for _, v := range taskCopy.Status.History {
							if v.Status != "succeed" {
								err = errors.New("error has occurred")
								run = false
							}
						}
						run = false

						ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
						defer cancel1()
						for {
							select {
							case <-ctx1.Done():
								Expect(errors.New("wait kdoctorreport timeout")).NotTo(HaveOccurred(), "failed to run kdoctor task and wait kdoctorreport timeout")
							default:
								command = fmt.Sprintf("get kdoctorreport %s -oyaml", taskCopy.Name)
								kdoctorreportLog, err := frame.ExecKubectl(command, ctx)
								if err != nil {
									time.Sleep(common.ForcedWaitingTime)
									continue
								}
								GinkgoWriter.Printf("kdoctor's kdoctorreport execution result %+v \n", string(kdoctorreportLog))
							}
							break
						}
					}
					time.Sleep(time.Second * 5)
				}
			}
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Validation of different input fields in the coodinator.", func() {
		var namespace, depName, mode, multusNadName string
		var nad *spiderpoolv2beta1.SpiderMultusConfig

		BeforeEach(func() {
			mode = "overlay"
			namespace = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)
			multusNadName = "test-multus-" + common.GenerateString(10, true)

			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			nad = &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode: &mode,
					},
				},
			}

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, multusNadName)
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())

				GinkgoWriter.Println("delete namespace: ", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
			})
		})

		It("Specify the NIC of the default route, but the NIC does not exist", Label("C00014"), func() {
			// 1.Set PodDefaultRouteNIC to a NIC that does not exist in the Pod.
			invalidNicName := common.NIC3
			nad.Spec.CoordinatorConfig.PodDefaultRouteNIC = &invalidNicName
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			// 2. Create a Pod using nad from step 1
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4PoolDefault}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6PoolDefault}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podList, err := common.CreateDeployUntilExpectedReplicas(frame, deployObject, ctx)
			Expect(err).NotTo(HaveOccurred())

			// 3. Pod creation should fail and prompt podDefaultRouteNIC: nic don't exist in pod
			errString := fmt.Sprintf("podDefaultRouteNIC: %s don't exist in pod", invalidNicName)
			ctx1, cancel := context.WithTimeout(context.Background(), common.EventOccurTimeout)
			defer cancel()
			for _, pod := range podList.Items {
				err = frame.WaitExceptEventOccurred(ctx1, common.OwnerPod, pod.Name, pod.Namespace, errString)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("In multi-NIC mode, whether the NIC name is random and pods are created normally", Label("C00015"), func() {
			randomNICName := "nic" + common.GenerateString(3, true)
			// 1. Generate random NIC name
			nad.Spec.CoordinatorConfig.PodDefaultRouteNIC = ptr.To(randomNICName)
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: randomNICName,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4PoolDefault}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6PoolDefault}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)

			// 2.Customize the Pod NIC name through muluts
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s@%s", namespace, multusNadName, randomNICName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			Eventually(func() error {
				podList, err := frame.GetPodListByLabel(deployObject.Spec.Template.Labels)
				if err != nil {
					return err
				}

				if !frame.CheckPodListRunning(podList) {
					return fmt.Errorf("pod not ready")
				}

				runGetIPLinkString := fmt.Sprintf("ip link show %s", randomNICName)
				for _, pod := range podList.Items {
					ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					_, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, runGetIPLinkString, ctx)
					if err != nil {
						GinkgoWriter.Printf("failed to execute ip link show, error is: %v, retrying...", err)
						return err
					}
				}

				return nil
			}).WithTimeout(time.Minute * 2).WithPolling(time.Second).Should(BeNil())
		})

		It("TunePodRoutes If false, no routing will be coordinated", Label("C00017"), func() {
			// 1. Set TunePodRoutes to false
			nad.Spec.CoordinatorConfig.TunePodRoutes = ptr.To(false)
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4PoolDefault}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6PoolDefault}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			// 2. Create a Pod using multus CR where TunePodRoutes is false
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			// 3. Check that the Pod can run, but table 100 should not exist in it
			Eventually(func() error {
				podList, err := frame.GetPodListByLabel(deployObject.Spec.Template.Labels)
				if err != nil {
					return err
				}

				if !frame.CheckPodListRunning(podList) {
					return fmt.Errorf("pod not ready")
				}

				table := "100"
				tableString := fmt.Sprintf("ip rule | grep %s", table)
				for _, pod := range podList.Items {
					ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					_, err := frame.ExecCommandInPod(pod.Name, pod.Namespace, tableString, ctx)
					if err == nil {
						return fmt.Errorf("table %v should not exist, try checking again...", table)
					}
				}
				if err != nil {
					GinkgoWriter.Printf("table 100 does not exist: %v", err)
				}
				return nil
			}).WithTimeout(time.Minute * 2).WithPolling(time.Second).Should(BeNil())
		})

		It("auto clean up the dirty rules(routing\neighborhood) while pod starting", Label("C00010"), func() {
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			// 1. Create an IP pool and set the IP number to 1.
			var v4PoolName, v6PoolName, dirtyV4IPUsed, dirtyV6IPUsed string
			var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
				err = common.CreateIppool(frame, v4PoolObj)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					v4PoolObj, err = common.GetIppoolByName(frame, v4PoolName)
					if api_errors.IsNotFound(err) {
						return err
					}
					if err != nil {
						return err
					}
					dirtyV4IPUsed = v4PoolObj.Spec.IPs[0]
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeNil())
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
				err = common.CreateIppool(frame, v6PoolObj)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					v6PoolObj, err = common.GetIppoolByName(frame, v6PoolName)
					if api_errors.IsNotFound(err) {
						return err
					}
					if err != nil {
						return err
					}
					dirtyV6IPUsed = v6PoolObj.Spec.IPs[0]
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second).Should(BeNil())
			}

			// 2. Create the NIC and set the IP address from step 1 as dirty data for the neighbor table in each node.
			var nicName = "nic" + common.GenerateString(3, true)
			var nicMac string
			for _, node := range frame.Info.KindNodeList {
				var err error
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				ipLinkAdd := fmt.Sprintf("ip link add %v type dummy", nicName)
				_, err = frame.DockerExecCommand(ctx, node, ipLinkAdd)
				Expect(err).NotTo(HaveOccurred())

				ipLinkShow := fmt.Sprintf("ip link show %v | grep link/ether | awk '{print $2}'", nicName)
				nicMacByte, err := frame.DockerExecCommand(ctx, node, ipLinkShow)
				nicMac = string(nicMacByte)
				Expect(err).NotTo(HaveOccurred())

				if frame.Info.IpV4Enabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					ipNaddIP := fmt.Sprintf("ip -4 n add %s dev %v lladdr %s", dirtyV4IPUsed, nicName, nicMac)
					_, err = frame.DockerExecCommand(ctx, node, ipNaddIP)
					Expect(err).NotTo(HaveOccurred())
				}

				if frame.Info.IpV6Enabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					ipNaddIP := fmt.Sprintf("ip -6 n add %s dev %v lladdr %s", dirtyV6IPUsed, nicName, nicMac)
					_, err = frame.DockerExecCommand(ctx, node, ipNaddIP)
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// 3. Create a Pod using the IP address from step 1 and check the Pod status
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{v6PoolName}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)

			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[constant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			// 4.Check that dirty data is automatically cleaned after the Pod is started
			Eventually(func() error {
				podList, err := frame.GetPodListByLabel(deployObject.Spec.Template.Labels)
				if err != nil {
					return err
				}

				if !frame.CheckPodListRunning(podList) {
					return fmt.Errorf("pod not ready")
				}

				var podNicMac []byte
				for _, pod := range podList.Items {
					ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					getIPMac := fmt.Sprintf("ip a | grep %s -A 1 | grep link/ether | awk '{print $2}'", common.NIC1)
					podNicMac, err = frame.ExecCommandInPod(pod.Name, pod.Namespace, getIPMac, ctx)
					if err != nil {
						return err
					}

					if frame.Info.IpV4Enabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
						defer cancel()
						getDirtyIPMac := fmt.Sprintf("ip -4 n | grep %s | grep %s | grep %s", dirtyV4IPUsed, nicName, strings.TrimRight(nicMac, "\n"))
						_, err = frame.DockerExecCommand(ctx, pod.Spec.NodeName, getDirtyIPMac)
						if err == nil {
							return fmt.Errorf("dirty IP Mac %v should not exist, try checking again...", nicMac)
						}

						podNicMacInNode := fmt.Sprintf("ip -4 n | grep %s | grep %s", dirtyV4IPUsed, strings.TrimRight(string(podNicMac), "\n"))
						_, err = frame.DockerExecCommand(ctx, pod.Spec.NodeName, podNicMacInNode)
						if err != nil {
							return fmt.Errorf("pod IP Mac %v should exist, try checking again..., errors %v ", podNicMac, err)
						}
					}

					if frame.Info.IpV6Enabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
						defer cancel()
						getDirtyIPMac := fmt.Sprintf("ip -6 n | grep %s | grep %s | grep %s", dirtyV6IPUsed, nicName, strings.TrimRight(nicMac, "\n"))
						_, err = frame.DockerExecCommand(ctx, pod.Spec.NodeName, getDirtyIPMac)
						if err == nil {
							return fmt.Errorf("dirty IP Mac %v should not exist, try checking again...", nicMac)
						}

						podNicMacInNode := fmt.Sprintf("ip -6 n | grep %s | grep %s", dirtyV6IPUsed, strings.TrimRight(string(podNicMac), "\n"))
						_, err = frame.DockerExecCommand(ctx, pod.Spec.NodeName, podNicMacInNode)
						if err != nil {
							return fmt.Errorf("pod IP Mac %v should exist, try checking again..., errors %v ", podNicMac, err)
						}
					}
				}

				return nil
			}).WithTimeout(time.Minute * 2).WithPolling(time.Second).Should(BeNil())
		})
	})
})

func getClusterServiceSubnet() (ipv4ServiceSubnet, ipv6ServiceSubnet string) {
	ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
	defer cancel()
	getConfigMapString := fmt.Sprintf("get configmap -n %v %v -oyaml | grep serviceSubnet | awk -F ': ' '{print $2}'", common.KubeadmConfigmapNameSpace, common.KubeadmConfigmapName)
	serviceSubnetString, err := frame.ExecKubectl(getConfigMapString, ctx)
	GinkgoWriter.Printf("The serviceSubnet of the cluster is: %v \n", string(serviceSubnetString))
	Expect(err).NotTo(HaveOccurred(), "Failed to obtain configuration mapping using command %v", getConfigMapString)

	if frame.Info.IpV4Enabled && !frame.Info.IpV6Enabled {
		return strings.TrimRight(string(serviceSubnetString), "\n"), ""
	}
	if frame.Info.IpV6Enabled && !frame.Info.IpV4Enabled {
		return "", strings.TrimRight(string(serviceSubnetString), "\n")
	}

	serviceSubnetList := strings.Split(strings.TrimRight(string(serviceSubnetString), "\n"), ",")
	return serviceSubnetList[0], serviceSubnetList[1]
}
