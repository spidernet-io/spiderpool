// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidercoordinator_suite_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/coordinatormanager"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/utils/pointer"
)

var _ = Describe("SpiderCoordinator", Label("spidercoordinator", "overlay"), Serial, func() {

	Context("auto mode of spidercoordinator", func() {
		// This case adaptation runs in different network modes, such as macvlan, calico, and cilium.
		// Prerequisite: The podCIDRType of the spidercoodinator deployed by default in the spiderpool environment is auto mode.
		It("Switch podCIDRType to `auto`, see if it could auto fetch the type", Label("V00001"), func() {

			By("Get the default spidercoodinator.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator,error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information: %+v \n", spc)

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
				calicoCNIConfigName = "10-calico.conflist"
				newCalicoCNIConfigName = "10-calico.conflist-bak"
				mvCNIConfig = fmt.Sprintf("mv /etc/cni/net.d/%s /etc/cni/net.d/%s", calicoCNIConfigName, newCalicoCNIConfigName)
			}

			if common.CheckRunOverlayCNI() && common.CheckCiliumFeatureOn() && !common.CheckCalicoFeatureOn() {
				GinkgoWriter.Println("The environment is cilium mode.")
				ciliumCNIConfigName = "05-cilium.conflist"
				newCiliumCNIConfigName = "05-cilium.conflist-bak"
				mvCNIConfig = fmt.Sprintf("mv /etc/cni/net.d/%s /etc/cni/net.d/%s", ciliumCNIConfigName, newCiliumCNIConfigName)
			}
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

				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				for _, pod := range podList.Items {
					_, err := frame.DockerExecCommand(ctx, pod.Spec.NodeName, mvCNIConfig)
					Expect(err).NotTo(HaveOccurred(), "Failed to execute mv command on the node %s ; error is %v", pod.Spec.NodeName, err)
				}

				Eventually(func() bool {
					By("Get the default spidercoodinator.")
					spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
					Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

					By("After restoring the cni configuration under /etc/cni/net.d, the environment returns to normal.")
					if spc.Status.OverlayPodCIDR == nil || spc.Status.Phase != coordinatormanager.Synced {
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
			})
		})

		It("Switch podCIDRType to `auto` but no cni files in /etc/cni/net.d, Viewing should be consistent with `none`.", Label("V00002"), func() {

			Eventually(func() bool {
				By("Get the default spidercoodinator.")
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				By("Checking status.overlayPodCIDR in automatic mode for pod CIDR type should be nil.")
				if spc.Status.OverlayPodCIDR != nil {
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
				GinkgoWriter.Printf("Display the default spider coordinator information: %+v \n", spc)

				// Switch podCIDRType to `auto`.
				spcCopy := spc.DeepCopy()
				spcCopy.Spec.PodCIDRType = pointer.String(common.PodCIDRTypeAuto)
				Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())

				Eventually(func() bool {
					spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
					Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

					if spc.Status.OverlayPodCIDR == nil || spc.Status.Phase != coordinatormanager.Synced {
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
			GinkgoWriter.Printf("Display the default spider coordinator information: %+v \n", spc)

			// Switch podCIDRType to `calico` or `cilium`.
			// This is a failure scenario where the cluster's default CNI is calico, but the podCIDRType is set to cilium.
			// Instead, when defaulting to Cilium, set podCIDRType to Calico
			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = pointer.String(invalidPodCIDRType)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase == coordinatormanager.Synced {
					GinkgoWriter.Printf("status.Phase and OverlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
					return false
				}

				// status.phase is not-ready, expect the cidr of status to be empty
				if spc.Status.Phase == coordinatormanager.NotReady {
					Expect(spc.Status.OverlayPodCIDR).Should(BeNil())
				}

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
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())

			spc, err = GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information: %+v \n", spc)

			spcCopy = spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = pointer.String(validPodCIDRType)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					GinkgoWriter.Printf("status.Phase status is still synchronizing, status %+v \n", spc.Status.Phase)
					return false
				}
				if spc.Status.OverlayPodCIDR == nil {
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
		})

		It("Switch podCIDRType to `none`, expect the cidr of status to be empty", Label("V00005"), func() {

			By("Get the default spidercoodinator.")
			spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)
			GinkgoWriter.Printf("Display the default spider coordinator information: %+v \n", spc)

			// Switch podCIDRType to `None`.
			spcCopy := spc.DeepCopy()
			spcCopy.Spec.PodCIDRType = pointer.String(common.PodCIDRTypeNone)
			Expect(PatchSpiderCoordinator(spcCopy, spc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				spc, err := GetSpiderCoordinator(common.SpidercoodinatorDefaultName)
				Expect(err).NotTo(HaveOccurred(), "failed to get SpiderCoordinator, error is %v", err)

				if spc.Status.Phase != coordinatormanager.Synced {
					GinkgoWriter.Printf("status.Phase status is still synchronizing, status %+v \n", spc.Status.Phase)
					return false
				}

				if spc.Status.OverlayPodCIDR != nil {
					GinkgoWriter.Printf("status.overlayPodCIDR status is still synchronizing, status %+v \n", spc.Status.OverlayPodCIDR)
					return false
				}

				GinkgoWriter.Println("status.overlayPodCIDR is nil, as expected.")
				return true
			}, common.ExecCommandTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})
	})
})
