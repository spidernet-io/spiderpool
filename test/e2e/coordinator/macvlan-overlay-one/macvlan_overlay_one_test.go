// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_overlay_one_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"k8s.io/utils/pointer"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	spiderdoctorV1 "github.com/spidernet-io/spiderdoctor/pkg/k8s/apis/spiderdoctor.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

var _ = Describe("MacvlanOverlayOne", Label("overlay", "one-nic", "coordinator"), func() {

	Context("In overlay mode with macvlan connectivity should be normal", func() {

		BeforeEach(func() {
			defer GinkgoRecover()
			var annotations = make(map[string]string)

			task = new(spiderdoctorV1.Nethttp)
			plan = new(spiderdoctorV1.SchedulePlan)
			target = new(spiderdoctorV1.NethttpTarget)
			targetAgent = new(spiderdoctorV1.TargetAgentSepc)
			request = new(spiderdoctorV1.NethttpRequest)
			condition = new(spiderdoctorV1.NetSuccessCondition)
			name = "one-macvlan-overlay-" + tools.RandomName()

			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanOverlayVlan100)
			if frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
				annotations[constant.AnnoPodIPPool] = `{"interface": "net1", "ipv4": ["vlan100-v4"], "ipv6": ["vlan100-v6"]}`
			} else if frame.Info.IpV4Enabled && !frame.Info.IpV6Enabled {
				annotations[constant.AnnoPodIPPool] = `{"interface": "net1", "ipv4": ["vlan100-v4"]}`
			} else {
				annotations[constant.AnnoPodIPPool] = `{"interface": "net1", "ipv6": ["vlan100-v6"]}`
			}

			GinkgoWriter.Printf("update spiderdoctoragent annotation: %v/%v annotation: %v \n", common.SpiderDoctorAgentNs, common.SpiderDoctorAgentDSName, annotations)
			spiderDoctorAgent, err = frame.GetDaemonSet(common.SpiderDoctorAgentDSName, common.SpiderDoctorAgentNs)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderDoctorAgent).NotTo(BeNil())

			GinkgoWriter.Printf("remove old spiderdoctor %v/%v \n", common.SpiderDoctorAgentNs, common.SpiderDoctorAgentDSName)
			err = frame.DeleteDaemonSet(common.SpiderDoctorAgentDSName, common.SpiderDoctorAgentNs)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				spiderDoctorAgentPodList, err := frame.GetPodListByLabel(spiderDoctorAgent.Spec.Template.Labels)
				if err != nil {
					GinkgoWriter.Printf("failed to get pod list %v,error is: %v \n", err)
					return false
				}
				if len(spiderDoctorAgentPodList.Items) != 0 {
					return false
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// issue: the object has been modified; please apply your changes to the latest version and try again
			spiderDoctorAgent.ResourceVersion = ""
			spiderDoctorAgent.CreationTimestamp = v1.Time{}
			spiderDoctorAgent.UID = apitypes.UID("")
			spiderDoctorAgent.Spec.Template.Annotations = annotations

			GinkgoWriter.Printf("create spiderdoctor %v/%v \n", common.SpiderDoctorAgentNs, common.SpiderDoctorAgentDSName)
			err = frame.CreateDaemonSet(spiderDoctorAgent)
			Expect(err).NotTo(HaveOccurred())

			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel()
			err = frame.WaitPodListRunning(spiderDoctorAgent.Spec.Selector.MatchLabels, len(nodeList.Items), ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("spiderdoctor connectivity should be succeed", Serial, Label("C00002"), func() {

			// create task spiderdoctor crd
			task.Name = name
			// schedule
			plan.StartAfterMinute = 0
			plan.RoundNumber = 2
			plan.IntervalMinute = 2
			plan.TimeoutMinute = 2
			task.Spec.Schedule = plan
			// target
			targetAgent.TestIngress = false
			targetAgent.TestEndpoint = true
			targetAgent.TestClusterIp = true
			targetAgent.TestMultusInterface = frame.Info.MultusEnabled
			targetAgent.TestNodePort = true

			targetAgent.TestIPv4 = &frame.Info.IpV4Enabled
			if common.CheckCiliumFeatureOn() {
				// TODO(tao.yang), set testIPv6 to false, reference issue: https://github.com/spidernet-io/spiderpool/issues/2007
				testIPv6 := false
				targetAgent.TestIPv6 = &testIPv6
			} else {
				targetAgent.TestIPv6 = &frame.Info.IpV6Enabled
			}

			GinkgoWriter.Printf("targetAgent for spiderdoctor %+v", targetAgent)
			target.TargetAgent = targetAgent
			task.Spec.Target = target

			// request
			request.DurationInSecond = 5
			request.QPS = 1
			request.PerRequestTimeoutInMS = 15000
			task.Spec.Request = request

			// success condition
			condition.SuccessRate = &successRate
			condition.MeanAccessDelayInMs = &delayMs
			task.Spec.SuccessCondition = condition

			taskCopy := task
			GinkgoWriter.Printf("spiderdoctor task: %+v", task)
			err := frame.CreateResource(task)
			Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd create failed")

			err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
			Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd get failed")

			ctx, cancel := context.WithTimeout(context.Background(), common.KdoctorCheckTime)
			defer cancel()

			for run {
				select {
				case <-ctx.Done():
					run = false
					Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running spiderdoctor task timeout")
				default:
					err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
					Expect(err).NotTo(HaveOccurred(), "spiderdoctor nethttp crd get failed,err is %v", err)

					if taskCopy.Status.Finish == true {
						GinkgoWriter.Printf("spiderdoctor's nethttp execution result %+v", taskCopy)
						for _, v := range taskCopy.Status.History {
							if v.Status != "succeed" {
								err = errors.New("error has occurred")
								run = false
							}
						}
						run = false
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

			var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
				gateway := strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
				v4PoolObj.Spec.Gateway = &gateway
				err = common.CreateIppool(frame, v4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "failed to create v4 ippool, error is: %v", err)
				v4Gateway = *v4PoolObj.Spec.Gateway
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(1)
				gateway := strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
				v6PoolObj.Spec.Gateway = &gateway
				err = common.CreateIppool(frame, v6PoolObj)
				Expect(err).NotTo(HaveOccurred(), "failed to create v6 ippool, error is: %v", err)
				v6Gateway = *v6PoolObj.Spec.Gateway
			}

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: "macvlan",
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

		It("the prefix of the pod mac address should be overridden and the default route should be on the specified NIC", Label("C00007", "C00005"), func() {
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

			// Check pod's mac address prefix
			commandString := fmt.Sprintf("ip link show dev %s | awk '/ether/ {print substr($2,0,5)}'", common.NIC2)
			ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			data, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, commandString, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)

			// the prefix of the pod mac address should be overridden.
			Expect(strings.TrimRight(string(data), "\n")).To(Equal(macPrefix), "macperfix is not covered, %s != %s", string(data), macPrefix)

			// Check the network card where the default route of the pod is located
			if frame.Info.IpV4Enabled {
				routeCommandString := "ip r show main | grep 'default via' | awk '{print $5}'"
				ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				interfaceData, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, routeCommandString, ctx)
				Expect(err).NotTo(HaveOccurred(), "failed to execute command %v , error is: %v ", routeCommandString, err)

				// The default route should be on the specified interface common.NIC2
				Expect(strings.TrimRight(string(interfaceData), "\n")).To(Equal(common.NIC2), "the default route is not in the specified interface %s ", common.NIC2)
			}
			if frame.Info.IpV6Enabled {
				routeCommandString := fmt.Sprintf("ip -6 r show main | grep 'default via' | grep %s | awk '{print $5}'", common.NIC2)
				ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				interfaceData, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, routeCommandString, ctx)
				Expect(err).NotTo(HaveOccurred(), "failed to execute command %v , error is: %v ", routeCommandString, err)

				// The default route should be on the specified interface common.NIC2
				Expect(strings.TrimRight(string(interfaceData), "\n")).To(Equal(common.NIC2), "the default route is not in the specified interface %s ", common.NIC2)
			}
		})

		It("gateway connection detection", Label("C00008"), func() {
			detectGatewayMultusName := "test-gateway-multus-" + common.GenerateString(10, true)
			detectGateway := true

			// Define multus cni NetworkAttachmentDefinition and set DetectGateway to true
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      detectGatewayMultusName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: "macvlan",
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
					CniType: "macvlan",
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
						VlanID: pointer.Int32(200),
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

		It("It should be possible to detect ip conflicts and log output", Label("C00006"), func() {
			podAnno := types.AnnoPodIPPoolValue{}

			if frame.Info.IpV4Enabled {
				spiderPoolIPv4SubnetVlan200, err := common.GetIppoolByName(frame, common.SpiderPoolIPv4SubnetVlan200)
				Expect(err).NotTo(HaveOccurred(), "failed to get v4 ippool, error is %v", err)

				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(1)
				v4PoolObj.Spec.Subnet = spiderPoolIPv4SubnetVlan200.Spec.Subnet
				v4PoolObj.Spec.Gateway = spiderPoolIPv4SubnetVlan200.Spec.Gateway
				v4PoolObj.Spec.Vlan = spiderPoolIPv4SubnetVlan200.Spec.Vlan

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
				v6PoolObj.Spec.Vlan = spiderPoolIPv6SubnetVlan200.Spec.Vlan
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
})
