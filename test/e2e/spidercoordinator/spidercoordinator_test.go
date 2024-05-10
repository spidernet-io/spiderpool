// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidercoordinator_suite_test

import (
	"context"
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
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
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, 3*common.ServiceAccountReadyTimeout)
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

				eventCtx, cancel := context.WithTimeout(context.Background(), common.EventOccurTimeout)
				defer cancel()
				errLog := "spidercoordinator: default no ready"
				for _, pod := range podList.Items {
					err = frame.WaitExceptEventOccurred(eventCtx, common.OwnerPod, pod.Name, pod.Namespace, errLog)
					Expect(err).NotTo(HaveOccurred())
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

	Context("It can get the clusterCIDR from kubeadmConfig and kube-controller-manager pod", func() {

		var spc *spiderpoolv2beta1.SpiderCoordinator
		var cm *corev1.ConfigMap
		var masterNode string
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

			masterNode = fmt.Sprintf("%s-control-plane", frame.Info.ClusterName)
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
					_, err := frame.GetConfigmap("kubeadm-config", "kube-system")
					if err != nil {
						GinkgoWriter.Printf("failed to get kubeadm-config: %v \n", err)
						return false
					}
					return true

				}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())

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
				}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
			})
		})

		It("Prioritize getting ClusterCIDR from kubeadm-config", Label("V00009"), func() {
			GinkgoWriter.Printf("podCIDR and serviceCIDR from spidercoordinator: %v,%v\n", spc.Status.OverlayPodCIDR, spc.Status.ServiceCIDR)

			podCIDR, serviceCIDr := coordinatormanager.ExtractK8sCIDRFromKubeadmConfigMap(cm)
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
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})

		It("Getting clusterCIDR from kube-controller-manager Pod when kubeadm-config does not exist", Label("V00010"), func() {
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

			kcmPodCIDR, kcmServiceCIDR := coordinatormanager.ExtractK8sCIDRFromKCMPod(&allPods.Items[0])
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
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})

		It("status should be NotReady if neither kubeadm-config configMap nor kube-controller-manager pod can be found", Label("V00011"), func() {
			By("update kube-controller-manager labels")
			command := "sed -i 's?component: kube-controller-manager?component: kube-controller-manager1?' /etc/kubernetes/manifests/kube-controller-manager.yaml"
			err = common.ExecCommandOnKindNode(context.TODO(), []string{masterNode}, command)
			Expect(err).NotTo(HaveOccurred(), "failed to update kcm labels: %v\n", err)

			// Delete it manually to speed up reconstruction.
			podList, err := frame.GetPodList(client.MatchingLabels{
				"component": "kube-controller-manager",
			})
			Expect(err).NotTo(HaveOccurred())
			if len(podList.Items) != 0 {
				Expect(frame.DeletePodList(podList)).NotTo(HaveOccurred())
			}

			Eventually(func() bool {
				podList, err := frame.GetPodList(client.MatchingLabels{
					"component": "kube-controller-manager1",
				})
				Expect(err).NotTo(HaveOccurred())

				if len(podList.Items) == 0 {
					return false
				}

				GinkgoWriter.Print("got kube-controller-manage pod's labels: \n", podList.Items[0].Labels)
				value, ok := podList.Items[0].Labels["component"]
				if !ok || value != "kube-controller-manager1" {
					return false
				}

				return true
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// delete the kubeadm-config configMap
			GinkgoWriter.Print("deleting kubeadm-config\n")
			err = frame.DeleteConfigmap("kubeadm-config", "kube-system")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				sp, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred())
				Expect(sp).NotTo(BeNil())

				GinkgoWriter.Print("got spidercoordinator's status: ", sp.Status)
				if sp.Status.Phase != coordinatormanager.NotReady {
					return false
				}

				if sp.Status.Reason != "No kube-controller-manager pod found, unable to get clusterCIDR" {
					return false
				}

				return true
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())

			func() {
				GinkgoWriter.Print("creating back kubeadm-config\n")
				cm.ResourceVersion = ""
				cm.Generation = 0
				err = frame.CreateConfigmap(cm)
				Expect(err).NotTo(HaveOccurred())
			}()

			GinkgoWriter.Print("revert kcm Pod's labels\n")
			commandBack := "sed -i 's?component: kube-controller-manager1?component: kube-controller-manager?' /etc/kubernetes/manifests/kube-controller-manager.yaml"
			err = common.ExecCommandOnKindNode(context.TODO(), []string{masterNode}, commandBack)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				podList, err := frame.GetPodList(client.MatchingLabels{
					"component": "kube-controller-manager",
				})
				Expect(err).NotTo(HaveOccurred())

				if len(podList.Items) == 0 {
					return false
				}

				kcmPod := podList.Items[0]
				GinkgoWriter.Printf("got kube-controller-manage pod's labels: %v\n", kcmPod.Labels)
				value, ok := kcmPod.Labels["component"]
				if !ok || value != "kube-controller-manager" {
					return false
				}

				GinkgoWriter.Printf("got kube-controller-manage pod's status: %v\n", kcmPod.Status.Phase)
				return kcmPod.Status.Phase == corev1.PodRunning
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

		})
	})

	Context("It can get service cidr from k8s serviceCIDR resources", Label("V00010"), func() {
		var spc *spiderpoolv2beta1.SpiderCoordinator
		var err error
		BeforeEach(func() {
			// serviceCIDR feature is available in k8s v1.29, DO NOT RUN this case
			// if the we don't found ServiceCIDRList resource
			var serviceCIDR networkingv1.ServiceCIDRList
			err := frame.ListResource(&serviceCIDR)
			if err != nil {
				GinkgoWriter.Printf("ServiceCIDR is not available, error: %v\n", err)
				Skip("k8s ServiceCIDR feature is not available")
			}

			if !common.CheckRunOverlayCNI() {
				GinkgoWriter.Println("This environment is in underlay mode.")
				Skip("Not applicable to underlay mode")
			}

			if !common.CheckCalicoFeatureOn() {
				GinkgoWriter.Println("The CNI isn't calico.")
				Skip("This case only run in calico")
			}
		})

		It("It can get service cidr from k8s serviceCIDR resources", func() {
			spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

			originalServiceCIDR := spc.Status.ServiceCIDR
			GinkgoWriter.Printf("serviceCIDR from original spidercoordinator: %v\n", spc.Status.ServiceCIDR)

			// create a serviceCIDR resource
			v4Svc := "10.234.0.0/16"
			v6Svc := "fd00:10:234::/116"

			err = CreateServiceCIDR("test", []string{v4Svc, v6Svc})
			Expect(err).NotTo(HaveOccurred(), "failed to create service cidr: %v")

			Eventually(func() bool {
				spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					return false
				}

				v4Found, v6Found := false, false
				for _, cidr := range spc.Status.ServiceCIDR {
					if cidr == v4Svc {
						v4Found = true
					}

					if cidr == v6Svc {
						v6Found = true
					}
				}
				return v4Found && v6Found
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// delete the serviceCIDR resource and see if we can back it
			err = DeleteServiceCIDR("test")
			Expect(err).NotTo(HaveOccurred(), "failed to delete service cidr: %v")

			Eventually(func() bool {
				spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					return false
				}

				if !reflect.DeepEqual(spc.Status.ServiceCIDR, originalServiceCIDR) {
					GinkgoWriter.Printf("Get spidercoordinator ServiceCIDR: %v\n", spc.Status.ServiceCIDR)
					return false
				}

				return true
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})
	})

	Context("Modify the global hostRuleTable and the effect is normal.", func() {
		var depName, nsName string

		BeforeEach(func() {
			nsName = "ns-" + common.GenerateString(10, true)
			depName = "dep-name-" + common.GenerateString(10, true)

			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, 3*common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				spcCopy := spc.DeepCopy()
				spcCopy.Spec.HostRuleTable = ptr.To(500)
				Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

				GinkgoWriter.Println("delete namespace: ", nsName)
				Expect(frame.DeleteNamespace(nsName)).NotTo(HaveOccurred())
			})
		})

		It("The table name can be customized by hostRuleTable ", Label("C00016"), func() {

			By("Get the default spidercoodinator.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

			spcCopy := spc.DeepCopy()
			hostRuleTable := 200
			spcCopy.Spec.HostRuleTable = ptr.To(hostRuleTable)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

			var annotations = make(map[string]string)
			annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
			deployObject := common.GenerateExampleDeploymentYaml(depName, nsName, int32(1))
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

				for _, pod := range podList.Items {
					var err error
					var ipRule []byte
					ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
					defer cancel()
					if frame.Info.IpV4Enabled {
						v4GetIPRuleString := fmt.Sprintf("ip -4 rule | grep %v", hostRuleTable)
						ipRule, err = frame.DockerExecCommand(ctx, pod.Spec.NodeName, v4GetIPRuleString)
					}
					if frame.Info.IpV6Enabled {
						v6GetIPRuleString := fmt.Sprintf("ip -6 rule | grep %v", hostRuleTable)
						ipRule, err = frame.DockerExecCommand(ctx, pod.Spec.NodeName, v6GetIPRuleString)
					}

					if err != nil {
						GinkgoWriter.Printf("failed to execute ip rule, error is: %v\n%v, retrying...", err, string(ipRule))
						return err
					}
				}

				return nil
			}).WithTimeout(time.Minute * 2).WithPolling(time.Second).Should(BeNil())
		})
	})
})
