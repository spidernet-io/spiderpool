// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_underlay_one_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	kdoctorV1beta1 "github.com/kdoctor-io/kdoctor/pkg/k8s/apis/kdoctor.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("MacvlanUnderlayOne", Serial, Label("underlay", "one-interface", "coordinator"), func() {

	Context("In underlay mode, verify single CNI network", func() {

		BeforeEach(func() {
			defer GinkgoRecover()
			task = new(kdoctorV1beta1.NetReach)
			targetAgent = new(kdoctorV1beta1.NetReachTarget)
			request = new(kdoctorV1beta1.NetHttpRequest)
			netreach = new(kdoctorV1beta1.AgentSpec)
			schedule = new(kdoctorV1beta1.SchedulePlan)
			condition = new(kdoctorV1beta1.NetSuccessCondition)

			name = "one-macvlan-standalone-" + tools.RandomName()

			// get macvlan-standalone multus crd instance by name
			multusInstance, err := frame.GetMultusInstance(common.MacvlanUnderlayVlan0, common.MultusNs)
			Expect(err).NotTo(HaveOccurred())
			Expect(multusInstance).NotTo(BeNil())

			// Update netreach.agentSpec to generate test Pods using the macvlan
			annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
			netreach.Annotation = annotations
			netreach.HostNetwork = false
			GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
			task.Spec.AgentSpec = netreach
		})

		It("kdoctor connectivity should be succeed", Label("C00001", "C00013"), Label("ebpf"), func() {

			enable := true
			disable := false
			// create task kdoctor crd
			task.Name = name
			GinkgoWriter.Printf("Start the netreach task: %v", task.Name)

			// Schedule
			crontab := "1 1"
			schedule.Schedule = &crontab
			// The sporadic test failures in kdoctor were attempted to be reproduced, but couldn't be.
			// By leveraging kdoctor's loop testing, if a failure occurs in the first test,
			// check whether it also fails on the second attempt.
			schedule.RoundNumber = 3
			schedule.RoundTimeoutMinute = 1
			task.Spec.Schedule = schedule

			// target
			targetAgent.Ingress = &disable
			targetAgent.Endpoint = &enable
			targetAgent.ClusterIP = &enable
			targetAgent.MultusInterface = &disable
			targetAgent.NodePort = &enable
			targetAgent.IPv4 = &frame.Info.IpV4Enabled
			targetAgent.IPv6 = &frame.Info.IpV6Enabled
			targetAgent.EnableLatencyMetric = true
			GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
			task.Spec.Target = targetAgent

			// request
			request.DurationInSecond = 10
			request.QPS = 1
			request.PerRequestTimeoutInMS = 7000
			task.Spec.Request = request

			// success condition
			condition.SuccessRate = &successRate
			condition.MeanAccessDelayInMs = &delayMs
			task.Spec.SuccessCondition = condition

			err := frame.CreateResource(task)
			Expect(err).NotTo(HaveOccurred(), "failed to create kdoctor task")
			GinkgoWriter.Printf("succeeded to create kdoctor task: %+v \n", task)

			// update the kdoctor service to use corev1.ServiceExternalTrafficPolicyLocal
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

			// waiting for kdoctor task to finish
			ctx, cancel := context.WithTimeout(context.Background(), common.KDoctorRunTimeout)
			defer cancel()
			for {
				select {
				case <-ctx.Done():
					Expect(errors.New("timeout waiting for kdoctor task to finish")).NotTo(HaveOccurred())
				default:
					taskCopy := task
					err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
					Expect(err).NotTo(HaveOccurred(), "Failed to get kdoctor task")
					if taskCopy.Status.Finish {
						roundFailed := false
						for _, t := range taskCopy.Status.History {
							// No configuration has been changed, The first round of the test is not considered a failure
							if t.RoundNumber != 1 && t.Status == "failed" {
								roundFailed = true
								break
							}
						}
						if roundFailed {
							Fail("kdoctor task is not successful")
						}
						return
					}
					for _, t := range taskCopy.Status.History {
						// If the check is successful, exit directly.
						if t.RoundNumber == 1 && t.Status == "succeed" {
							GinkgoWriter.Println("succeed to run kdoctor task")
							return
						}
						// If the check fails, we should collect the failed Pod network information as soon as possible
						// If the first attempt failed but the second attempt succeeded,
						// we collected network logs and compared the two attempts to see if there were any differences.
						if t.Status == "failed" || (t.RoundNumber != 1 && t.Status == "succeed") {
							GinkgoLogr.Error(fmt.Errorf("Failed to run kdoctor task, round %d, at time %s", t.RoundNumber, time.Now()), "Failed")
							podList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/name": taskCopy.Name})
							Expect(err).NotTo(HaveOccurred(), "Failed to get pod list by label")
							Expect(common.GetPodNetworkInfo(ctx, frame, podList)).NotTo(HaveOccurred(), "Failed to get pod network info")
							Expect(common.GetNodeNetworkInfo(ctx, frame, frame.Info.KindNodeList)).NotTo(HaveOccurred(), "Failed to get node network info")
						}
					}
				}
			}
		})
	})

	Context("Use 'ip r get' to check if the default route is the specified NIC", func() {
		var v4PoolName, v6PoolName, namespace, depName, multusNadName string

		BeforeEach(func() {
			// generate some test data
			namespace = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)
			multusNadName = "test-multus-" + common.GenerateString(10, true)

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
					CniType: ptr.To(pkgconstant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
						VlanID: ptr.To(int32(100)),
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						PodDefaultRouteNIC: &common.NIC2,
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

		It("In underlay mode: specify the NIC (net1) where the default route is located, use 'ip r get 8.8.8.8' to see if default route nic is the specify NIC", Label("C00006"), func() {
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC: common.NIC1,
				},
				types.AnnoIPPoolItem{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4PoolDefault}
				podIppoolsAnno[1].IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6PoolDefault}
				podIppoolsAnno[1].IPv6Pools = []string{v6PoolName}
			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", namespace, multusNadName)
			annotations[pkgconstant.AnnoPodIPPools] = string(podAnnoMarshal)
			deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
			deployObject.Spec.Template.Annotations = annotations
			Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			depObject, err := frame.WaitDeploymentReady(depName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred(), "waiting for deploy ready failed:  %v ", err)
			podList, err := frame.GetPodListByLabel(depObject.Spec.Template.Labels)
			Expect(err).NotTo(HaveOccurred(), "failed to get podList: %v ", err)

			// Check the NIC where the default route of the pod is located
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
