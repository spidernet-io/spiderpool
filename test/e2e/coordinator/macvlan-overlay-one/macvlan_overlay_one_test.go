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

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
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
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100)
			if frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
				annotations[constant.AnnoPodIPPool] = `{"interface": "net1", "ipv4": ["vlan100-v4"], "ipv6": ["vlan100-v6"]}`
			} else if frame.Info.IpV4Enabled && !frame.Info.IpV6Enabled {
				annotations[constant.AnnoPodIPPool] = `{"interface": "net1", "ipv4": ["vlan100-v4"]}`
			} else {
				annotations[constant.AnnoPodIPPool] = `{"interface": "net1", "ipv6": ["vlan100-v6"]}`
			}
			netreach.Annotation = annotations
			netreach.HostNetwork = false
			GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
			task.Spec.AgentSpec = netreach
		})

		// TODO (TY): kdoctor failed
		PIt("kdoctor connectivity should be succeed", Serial, Label("C00002"), Label("ebpf"), func() {

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
			crontab := "0 1"
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

	Context("coordinator's gateway, macperfix's authentication", func() {
		var v4PoolName, v6PoolName, namespace, macPrefix, depName, mode, multusNadName string
		var v4Gateway, v6Gateway string
		var podCidrType string

		BeforeEach(func() {
			// generate some test data
			macPrefix = "1a:1b"
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
		It("gateway connection detection", Label("V00007", "C00009"), func() {
			detectGatewayMultusName := "test-gateway-multus-" + common.GenerateString(10, true)
			detectGateway := true

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
						Mode:          &mode,
						DetectGateway: &detectGateway,
						PodCIDRType:   &podCidrType,
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
		var ipConflict bool
		var multusNadName = "test-multus-" + common.GenerateString(10, true)

		BeforeEach(func() {
			depName = "dep-name-" + common.GenerateString(10, true)
			namespace = "ns-" + common.GenerateString(10, true)
			mode = "overlay"
			ipConflict = true
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
						VlanID: ptr.To(int32(200)),
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode:             &mode,
						DetectIPConflict: &ipConflict,
						PodCIDRType:      &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			DeferCleanup(func() {
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
		It("It should be possible to detect ip conflicts and log output", Label("C00007", "V00007"), func() {
			podAnno := types.AnnoPodIPPoolValue{}

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
