// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidercoordinator_suite_test

import (
	"context"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/utils"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SpiderCoordinator", Label("spidercoordinator", "overlay"), Serial, func() {

	Context("auto mode of spidercoordinator", func() {
		// This case adaptation runs in different network modes, such as macvlan, calico, and cilium.
		// Prerequisite: The podCIDRType of the spidercoodinator deployed by default in the spiderpool environment is auto mode.
		It("Switch podCIDRType to `auto`, see if it could auto fetch the type", Label("V00001"), func() {

			By("Get the default spidercoodinator.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator,error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

			By("Checking podCIDRType for status.overlayPodCIDR in auto mode is as expected.")
			// Loop through all of the OverlayPodCIDRs to avoid the possibility of a value mismatch.
			for _, cidr := range spc.Status.OverlayPodCIDR {
				if ip.IsIPv4CIDR(cidr) {
					Expect(cidr).To(Equal(v4PodCIDRString))
					GinkgoWriter.Printf("ipv4 podCIDR is as expected, value %v=%v \n", cidr, v4PodCIDRString)
				} else {
					Expect(cidr).To(Equal(v6PodCIDRString))
					GinkgoWriter.Printf("ipv6 podCIDR is as expected, value %v=%v \n", cidr, v6PodCIDRString)
				}
			}
		})
	})

	Context("There is no cni file in /etc/cni/net.d.", func() {
		var calicoCNIConfigName, ciliumCNIConfigName string
		var newCalicoCNIConfigName, newCiliumCNIConfigName string
		var cniMode string

		BeforeEach(func() {
			podList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderpoolController, error is %v", err)

			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			var mvCNIConfig string
			if !common.CheckRunOverlayCNI() && !common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
				GinkgoWriter.Println("This environment is in underlay mode.")
				Skip("Not applicable to underlay mode")
			}

			if common.CheckRunOverlayCNI() && common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
				GinkgoWriter.Println("The environment is calico mode.")
				cniMode = common.PodCIDRTypeCalico
				calicoCNIConfigName = "10-calico.conflist"
				newCalicoCNIConfigName = "10-calico.conflist-bak"
				mvCNIConfig = fmt.Sprintf("mv /etc/cni/net.d/%s /etc/cni/net.d/%s", calicoCNIConfigName, newCalicoCNIConfigName)
			}

			if common.CheckRunOverlayCNI() && common.CheckCiliumFeatureOn() && !common.CheckCalicoFeatureOn() {
				GinkgoWriter.Println("The environment is cilium mode.")
				cniMode = common.PodCIDRTypeCilium
				ciliumCNIConfigName = "05-cilium.conflist"
				newCiliumCNIConfigName = "05-cilium.conflist-bak"
				mvCNIConfig = fmt.Sprintf("mv /etc/cni/net.d/%s /etc/cni/net.d/%s", ciliumCNIConfigName, newCiliumCNIConfigName)
			}

			// Switch podCIDRType to cniType.
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = ptr.To(cniMode)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

			for _, pod := range podList.Items {
				_, err := frame.DockerExecCommand(ctx, pod.Spec.NodeName, mvCNIConfig)
				Expect(err).NotTo(HaveOccurred(), "Failed to execute mv command on the node %s ; error is %v", pod.Spec.NodeName, err)
			}

			DeferCleanup(func() {
				if common.CheckRunOverlayCNI() && common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
					GinkgoWriter.Println("The environment is calico mode.")
					mvCNIConfig = fmt.Sprintf("mv /etc/cni/net.d/%s /etc/cni/net.d/%s", newCalicoCNIConfigName, calicoCNIConfigName)
				}

				if common.CheckRunOverlayCNI() && common.CheckCiliumFeatureOn() && !common.CheckCalicoFeatureOn() {
					GinkgoWriter.Println("The environment is cilium mode.")
					mvCNIConfig = fmt.Sprintf("mv /etc/cni/net.d/%s /etc/cni/net.d/%s", newCiliumCNIConfigName, ciliumCNIConfigName)
				}

				// Switch podCIDRType to cniType.
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
				GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

				spcCopy := spc.DeepCopy()
				spcCopy.Spec.PodCIDRType = ptr.To(cniMode)
				Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				for _, pod := range podList.Items {
					_, err := frame.DockerExecCommand(ctx, pod.Spec.NodeName, mvCNIConfig)
					Expect(err).NotTo(HaveOccurred(), "Failed to execute mv command on the node %s ; error is %v", pod.Spec.NodeName, err)
				}

				// Switch podCIDRType to cniType.
				sPodCIDRTypeCNI, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
				sPodCIDRTypeCNICopy := sPodCIDRTypeCNI.DeepCopy()

				GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *sPodCIDRTypeCNICopy.Spec.PodCIDRType, sPodCIDRTypeCNICopy.Status)
				sPodCIDRTypeCNICopy.Spec.PodCIDRType = ptr.To(common.PodCIDRTypeAuto)
				Expect(PatchSpiderCoordinator(sPodCIDRTypeCNICopy, sPodCIDRTypeCNI)).NotTo(HaveOccurred())

				Eventually(func() bool {
					By("Get the default spidercoodinator.")
					spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
					Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
					GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

					By("Checking podCIDRType if is auto.")
					if *spc.Spec.PodCIDRType != common.PodCIDRTypeAuto {
						return false
					}

					if len(spc.Status.OverlayPodCIDR) == 0 || spc.Status.Phase != coordinatormanager.Synced {
						return false
					}
					for _, cidr := range spc.Status.OverlayPodCIDR {
						if ip.IsIPv4CIDR(cidr) {
							Expect(cidr).To(Equal(v4PodCIDRString))
							GinkgoWriter.Printf("ipv4 podCIDR is as expected, value %v=%v \n", cidr, v4PodCIDRString)
						} else {
							Expect(cidr).To(Equal(v6PodCIDRString))
							GinkgoWriter.Printf("ipv6 podCIDR is as expected, value %v=%v \n", cidr, v6PodCIDRString)
						}
					}
					return true
				}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
			})
		})

		It("Switch podCIDRType to `auto` but no cni files in /etc/cni/net.d, Viewing should be consistent with `none`.", Label("V00002"), func() {
			By("Change podCIDRType to auto, which can trigger the spidercoordinator updated.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = ptr.To(common.PodCIDRTypeAuto)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

			Eventually(func() bool {
				By("Get the default spidercoodinator.")
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				By("Checking podCIDRType is auto.")
				if *spc.Spec.PodCIDRType != common.PodCIDRTypeAuto {
					GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, podCIDRTye: %+v \n", *spc.Spec.PodCIDRType)
					return false
				}

				By("Checking status.overlayPodCIDR in auto mode for podCIDRType should be empty.")
				if len(spc.Status.OverlayPodCIDR) > 0 {
					GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
					return false
				}

				if spc.Status.Phase != coordinatormanager.Synced {
					GinkgoWriter.Printf("status.Phase is still synchronizing, status is %+v \n", spc.Status.Phase)
					return false
				}

				return true
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})
	})

	Context("Switch podCIDRType to `calico` or `cilium`、`none` ", func() {
		var invalidPodCIDRType, validPodCIDRType, depName, namespace string

		BeforeEach(func() {
			if !common.CheckRunOverlayCNI() && !common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
				GinkgoWriter.Println("This environment is in underlay mode.")
				Skip("Not applicable to underlay mode")
			}

			if common.CheckCalicoFeatureOn() && !common.CheckCiliumFeatureOn() {
				GinkgoWriter.Println("The environment is calico mode.")
				invalidPodCIDRType = common.PodCIDRTypeCilium
				validPodCIDRType = common.PodCIDRTypeCalico
			}

			if common.CheckCiliumFeatureOn() && !common.CheckCalicoFeatureOn() {
				GinkgoWriter.Println("The environment is cilium mode.")
				invalidPodCIDRType = common.PodCIDRTypeCalico
				validPodCIDRType = common.PodCIDRTypeCilium
			}

			namespace = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				// The default podCIDRType for all environments is `auto` and should eventually fall back to auto mode in any case.
				// Avoid failure of other use cases.
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
				GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

				// Switch podCIDRType to `auto`.
				spcCopy := spc.DeepCopy()
				spcCopy.Spec.PodCIDRType = ptr.To(common.PodCIDRTypeAuto)
				Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

				Eventually(func() bool {
					spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
					Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
					GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

					if len(spc.Status.OverlayPodCIDR) == 0 || spc.Status.Phase != coordinatormanager.Synced {
						GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
						return false
					}
					for _, cidr := range spc.Status.OverlayPodCIDR {
						if ip.IsIPv4CIDR(cidr) {
							Expect(cidr).To(Equal(v4PodCIDRString))
							GinkgoWriter.Printf("ipv4 podCIDR is as expected, value %v=%v \n", cidr, v4PodCIDRString)
						} else {
							Expect(cidr).To(Equal(v6PodCIDRString))
							GinkgoWriter.Printf("ipv6 podCIDR is as expected, value %v=%v \n", cidr, v6PodCIDRString)
						}
					}
					return true
				}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())

				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
			})
		})

		// This case adaptation runs in different network modes, such as macvlan, calico, and cilium.
		// Prerequisite: The podCIDRType of the spidercoodinator deployed by default in the spiderpool environment is auto mode.
		It("Switch podCIDRType to `calico` or `cilium`, see if it could auto fetch the cidr from calico ippools", Label("V00003", "V00004", "V00006", "V00008"), func() {

			By("Get the default spidercoodinator.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

			// Switch podCIDRType to `calico` or `cilium`.
			// This is a failure scenario where the cluster's default CNI is calico, but the podCIDRType is set to cilium.
			// Instead, when defaulting to Cilium, set podCIDRType to Calico
			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = ptr.To(invalidPodCIDRType)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase == coordinatormanager.Synced {
					GinkgoWriter.Printf("status.Phase and OverlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
					return false
				}

				// status.phase is not-ready, expect the cidr of status to be empty
				Expect(spc.Status.OverlayPodCIDR).Should(BeEmpty())
				GinkgoWriter.Printf("status.Phase status is %+v \n", spc.Status.Phase)

				// Pod creation in the Not Ready state should fail.
				var annotations = make(map[string]string)
				annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
				deployObject := common.GenerateExampleDeploymentYaml(depName, namespace, int32(1))
				deployObject.Spec.Template.Annotations = annotations
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				podList, err := common.CreateDeployUntilExpectedReplicas(frame, deployObject, ctx)
				Expect(err).NotTo(HaveOccurred())
				ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
				defer cancel()
				errLog := "spidercoordinator: default no ready"
				for _, pod := range podList.Items {
					err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, errLog)
					Expect(err).To(Succeed(), "Failed to get 'spidercoordinator not ready', error is: %v", err)
				}

				return true
			}, common.ExecCommandTimeout, common.EventOccurTimeout*2).Should(BeTrue())

			spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

			spcCopy = spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = ptr.To(validPodCIDRType)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
				GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

				if len(spc.Status.OverlayPodCIDR) == 0 || spc.Status.Phase != coordinatormanager.Synced {
					GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
					return false
				}
				for _, cidr := range spc.Status.OverlayPodCIDR {
					if ip.IsIPv4CIDR(cidr) {
						Expect(cidr).To(Equal(v4PodCIDRString))
						GinkgoWriter.Printf("ipv4 podCIDR is as expected, value %v=%v \n", cidr, v4PodCIDRString)
					} else {
						Expect(cidr).To(Equal(v6PodCIDRString))
						GinkgoWriter.Printf("ipv6 podCIDR is as expected, value %v=%v \n", cidr, v6PodCIDRString)
					}
				}
				return true
			}, common.ExecCommandTimeout, common.EventOccurTimeout*2).Should(BeTrue())
		})

		It("Switch podCIDRType to `none`, expect the cidr of status to be empty", Label("V00005"), func() {

			By("Get the default spidercoodinator.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

			// Switch podCIDRType to `None`.
			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = ptr.To(common.PodCIDRTypeNone)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					GinkgoWriter.Printf("status.Phase status is still synchronizing, status %+v \n", spc.Status.Phase)
					return false
				}

				if len(spc.Status.OverlayPodCIDR) > 0 {
					GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
					return false
				}

				GinkgoWriter.Println("status.overlayPodCIDR is nil, as expected.")
				return true
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})
	})

	Context("It can get the clusterCIDR from kubeadmConfig and kube-controller-manager pod", Label("V00009"), func() {
		var spc *spiderpoolv2beta1.SpiderCoordinator
		var cm *corev1.ConfigMap
		var err error
		BeforeEach(func() {
			if !common.CheckRunOverlayCNI() {
				GinkgoWriter.Println("This environment is in underlay mode.")
				Skip("Not applicable to underlay mode")
			}

			if !common.CheckCalicoFeatureOn() {
				GinkgoWriter.Println("The CNI isn't calico.")
				Skip("This case only run in calico")
			}

			cm, err = frame.GetConfigmap("kubeadm-config", "kube-system")
			Expect(err).NotTo(HaveOccurred())

			spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

			// Switch podCIDRType to `cluster`.
			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = ptr.To(common.PodCIDRTypeCluster)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

			DeferCleanup(func() {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
				GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

				// Switch podCIDRType to `auto`.
				spcCopy := spc.DeepCopy()
				spcCopy.Spec.PodCIDRType = ptr.To(common.PodCIDRTypeAuto)
				Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

				Eventually(func() bool {
					spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
					Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
					GinkgoWriter.Printf("Display the default spider coordinator information, podCIDRType: %v, status: %v \n", *spc.Spec.PodCIDRType, spc.Status)

					if len(spc.Status.OverlayPodCIDR) == 0 || spc.Status.Phase != coordinatormanager.Synced {
						GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
						return false
					}

					for _, cidr := range spc.Status.OverlayPodCIDR {
						if ip.IsIPv4CIDR(cidr) {
							if cidr != v4PodCIDRString {
								return false
							}
							GinkgoWriter.Printf("ipv4 podCIDR is as expected, value %v=%v \n", cidr, v4PodCIDRString)
						} else {
							if cidr != v6PodCIDRString {
								return false
							}
							GinkgoWriter.Printf("ipv6 podCIDR is as expected, value %v=%v \n", cidr, v6PodCIDRString)
						}
					}
					return true
				}, common.ExecCommandTimeout, common.EventOccurTimeout*2).Should(BeTrue())
			})
		})

		It("Prioritize getting ClusterCIDR from kubeadm-config", func() {
			GinkgoWriter.Printf("podCIDR and serviceCIDR from spidercoordinator: %v,%v\n", spc.Status.OverlayPodCIDR, spc.Status.ServiceCIDR)

			podCIDR, serviceCIDr, err := utils.ExtractK8sCIDRFromKubeadmConfigMap(cm)
			Expect(err).NotTo(HaveOccurred(), "Failed to extract k8s CIDR from Kubeadm configMap,  error is %v", err)
			GinkgoWriter.Printf("podCIDR and serviceCIDR from kubeadm-config : %v,%v\n", podCIDR, serviceCIDr)

			Eventually(func() bool {
				spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					return false
				}

				if reflect.DeepEqual(podCIDR, spc.Status.OverlayPodCIDR) && reflect.DeepEqual(serviceCIDr, spc.Status.ServiceCIDR) {
					return true
				}

				return false
			}, common.ExecCommandTimeout, common.EventOccurTimeout*2).Should(BeTrue())
		})

		It("Getting clusterCIDR from kube-controller-manager Pod when kubeadm-config does not exist", func() {
			// delete the kubeadm-config configMap
			GinkgoWriter.Print("deleting kubeadm-config\n")
			err = frame.DeleteConfigmap("kubeadm-config", "kube-system")
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cm.ResourceVersion = ""
				cm.Generation = 0
				err = frame.CreateConfigmap(cm)
				Expect(err).NotTo(HaveOccurred())
			}()

			allPods, err := frame.GetPodList(client.MatchingLabels{"component": "kube-controller-manager"})
			Expect(err).NotTo(HaveOccurred())

			kcmPodCIDR, kcmServiceCIDR := utils.ExtractK8sCIDRFromKCMPod(&allPods.Items[0])
			GinkgoWriter.Printf("podCIDR and serviceCIDR from kube-controller-manager pod : %v,%v\n", kcmPodCIDR, kcmServiceCIDR)

			Eventually(func() bool {
				spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					return false
				}

				if reflect.DeepEqual(kcmPodCIDR, spc.Status.OverlayPodCIDR) && reflect.DeepEqual(kcmServiceCIDR, spc.Status.ServiceCIDR) {
					return true
				}

				return false
			}, common.ExecCommandTimeout, common.EventOccurTimeout*2).Should(BeTrue())
		})
	})
})
