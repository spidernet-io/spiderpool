// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package kubevirt_test

import (
	"context"
	"fmt"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test kubevirt", Label("kubevirt"), func() {
	var (
		virtualMachine *kubevirtv1.VirtualMachine
		ctx            context.Context
		namespace      string
	)

	BeforeEach(func() {
		ctx = context.TODO()

		// make sure the vm has the macvlan annotation.
		virtualMachine = vmTemplate.DeepCopy()
		anno := virtualMachine.Spec.Template.ObjectMeta.GetAnnotations()
		anno[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
		virtualMachine.Spec.Template.ObjectMeta.SetAnnotations(anno)

		// create namespace
		namespace = "ns" + utilrand.String(randomLength)
		GinkgoWriter.Printf("create namespace %v. \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Printf("delete namespace %v. \n", namespace)
			Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())
		})
	})

	// TODO(ty-dc): Kubevirt removed support for passt network core binding in v1.3, see https://github.com/kubevirt/kubevirt/pull/11915
	// The reason for removal is: "Network Core Binding has not yet reached GA level",
	// but the feature has not been abandoned? Temporarily pending e2e, follow up later.
	It("Succeed to keep static IP for kubevirt VM/VMI after restarting the VM/VMI pod", Label("F00001"), Pending, func() {
		// VM crash with Passt network mode: https://github.com/kubevirt/kubevirt/issues/10583
		// reference spiderpool CI issue: https://github.com/spidernet-io/spiderpool/issues/2460
		if !frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
			Skip("skip IPv6-only for Passt network mode")
		}

		// 1. create a kubevirt vm with passt network mode
		virtualMachine.Spec.Template.Spec.Networks = []kubevirtv1.Network{
			{
				Name: "default",
				NetworkSource: kubevirtv1.NetworkSource{
					Pod: &kubevirtv1.PodNetwork{},
				},
			},
		}
		virtualMachine.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
			{
				Name: "default",
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Passt: &kubevirtv1.InterfacePasst{},
				},
			},
		}
		virtualMachine.Name = fmt.Sprintf("%s-%s", virtualMachine.Name, utilrand.String(randomLength))
		virtualMachine.Namespace = namespace
		GinkgoWriter.Printf("try to create kubevirt VM: %v \n", virtualMachine)
		err := frame.CreateResource(virtualMachine)
		Expect(err).NotTo(HaveOccurred())

		// 2. wait for the vmi to be ready and record the vmi corresponding vmi pod IP
		vmi, err := waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())

		vmInterfaces := make(map[string][]string)
		for _, vmNetworkInterface := range vmi.Status.Interfaces {
			ips := vmNetworkInterface.IPs
			sort.Strings(ips)
			vmInterfaces[vmNetworkInterface.Name] = ips
		}
		GinkgoWriter.Printf("original VMI NIC allocations: %v \n", vmInterfaces)

		// 3. restart the vmi object and compare the new vmi pod IP whether is same with the previous-recorded IP
		GinkgoWriter.Printf("try to restart VMI %s/%s", vmi.Namespace, vmi.Name)
		err = frame.KClient.Delete(ctx, vmi)
		Expect(err).NotTo(HaveOccurred())
		vmi, err = waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())

		tmpVMInterfaces := make(map[string][]string)
		for _, vmNetworkInterface := range vmi.Status.Interfaces {
			ips := vmNetworkInterface.IPs
			sort.Strings(ips)
			tmpVMInterfaces[vmNetworkInterface.Name] = ips
		}
		GinkgoWriter.Printf("new VMI NIC allocations: %v \n", tmpVMInterfaces)
		Expect(vmInterfaces).Should(Equal(tmpVMInterfaces))
	})

	// TODO (Icarus9913): after migration, the new vm pod try to pull a different tag image which may cause image pull failed.
	PIt("Succeed to keep static IP for the kubevirt VM live migration", Label("F00002"), func() {
		// 1. create a kubevirt vm with masquerade mode (At present, it seems like the live migration only supports masquerade mode)
		virtualMachine.Spec.Template.Spec.Networks = []kubevirtv1.Network{
			{
				Name: "default",
				NetworkSource: kubevirtv1.NetworkSource{
					Pod: &kubevirtv1.PodNetwork{},
				},
			},
		}
		virtualMachine.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
			{
				Name: "default",
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Masquerade: &kubevirtv1.InterfaceMasquerade{},
				},
			},
		}
		virtualMachine.Name = fmt.Sprintf("%s-%s", virtualMachine.Name, utilrand.String(randomLength))
		virtualMachine.Namespace = namespace
		GinkgoWriter.Printf("try to create kubevirt VM: %v \n", virtualMachine)
		err := frame.CreateResource(virtualMachine)
		Expect(err).NotTo(HaveOccurred())

		// 2. record the vmi corresponding vmi pod IP
		_, err = waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())

		var podList corev1.PodList
		err = frame.KClient.List(ctx, &podList, client.MatchingLabels{
			kubevirtv1.VirtualMachineNameLabel: virtualMachine.Name,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(podList.Items).To(HaveLen(1))
		originalPodName := podList.Items[0].Name
		originalPodIPs := podList.Items[0].Status.PodIPs
		GinkgoWriter.Printf("original virt-launcher pod '%s/%s' IP allocations: %v \n", namespace, originalPodName, originalPodIPs)

		// 3. create a vm migration
		vmim := &kubevirtv1.VirtualMachineInstanceMigration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-migration", virtualMachine.Name),
				Namespace: virtualMachine.Namespace,
			},
			Spec: kubevirtv1.VirtualMachineInstanceMigrationSpec{
				VMIName: virtualMachine.Name,
			},
		}
		GinkgoWriter.Printf("try to create VirtualMachineInstanceMigration: %v \n", vmim)
		err = frame.KClient.Create(ctx, vmim)
		Expect(err).NotTo(HaveOccurred())

		// 4. wait for the completion of the migration and compare the new vmi pod IP whether is same with the previous-recorded IP
		Eventually(func() error {
			tmpPod, err := frame.GetPod(originalPodName, virtualMachine.Namespace)
			if nil != err {
				return err
			}
			// After migration the previous pod is in Failed phase with kubevirt v1.1.0 version: https://github.com/kubevirt/kubevirt/issues/10695
			if tmpPod.Status.Phase == corev1.PodSucceeded || tmpPod.Status.Phase == corev1.PodFailed {
				return nil
			}
			return fmt.Errorf("virt-launcher pod %s/%s phase is %s, the vm is still in live migration phase", tmpPod.Namespace, tmpPod.Name, tmpPod.Status.Phase)
		}).WithTimeout(time.Minute * 10).WithPolling(time.Second * 5).Should(BeNil())
		GinkgoWriter.Printf("virt-launcher pod %s/%s is completed\n", namespace, originalPodName)

		var newPodList corev1.PodList
		err = frame.KClient.List(ctx, &newPodList, client.MatchingLabels{
			kubevirtv1.VirtualMachineNameLabel: virtualMachine.Name,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(newPodList.Items).To(HaveLen(2))
		for _, tmpPod := range newPodList.Items {
			if tmpPod.Name == originalPodName {
				continue
			}
			GinkgoWriter.Printf("the new migration virt-launcher pod %s/%s IP allocations: %v \n", tmpPod.Namespace, tmpPod.Name, tmpPod.Status.PodIPs)
			Expect(tmpPod.Status.PodIPs).To(Equal(originalPodIPs))
		}
	})

	It("Succeed to allocation multiple NICs", Label("F00003"), func() {
		// 1. create a kubevirt vm with bridge + multus multiple NIC network mode
		ovs30 := "ovs30"
		ovs40 := "ovs40"
		virtualMachine.Spec.Template.Spec.Networks = []kubevirtv1.Network{
			{
				Name: ovs30,
				NetworkSource: kubevirtv1.NetworkSource{
					Multus: &kubevirtv1.MultusNetwork{
						NetworkName: fmt.Sprintf("%s/%s", common.MultusNs, common.OvsVlan30),
						Default:     true,
					},
				},
			},
			{
				Name: ovs40,
				NetworkSource: kubevirtv1.NetworkSource{
					Multus: &kubevirtv1.MultusNetwork{
						NetworkName: fmt.Sprintf("%s/%s", common.MultusNs, common.OvsVlan40),
					},
				},
			},
		}
		virtualMachine.Spec.Template.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
			{
				Name: ovs30,
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Bridge: &kubevirtv1.InterfaceBridge{},
				},
			},
			{
				Name: ovs40,
				InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
					Bridge: &kubevirtv1.InterfaceBridge{},
				},
			},
		}

		// with virtualMachine.Spec.Template.Spec.Networks set with multus, we don't need to add multus annotations
		anno := map[string]string{
			constant.AnnoDefaultRouteInterface: constant.ClusterDefaultInterfaceName,
		}
		virtualMachine.Spec.Template.ObjectMeta.SetAnnotations(anno)
		virtualMachine.Name = fmt.Sprintf("%s-%s", virtualMachine.Name, utilrand.String(randomLength))
		virtualMachine.Namespace = namespace
		GinkgoWriter.Printf("try to create kubevirt VM: %v \n", virtualMachine)
		err := frame.CreateResource(virtualMachine)
		Expect(err).NotTo(HaveOccurred())

		// 2. wait for the vmi to be ready
		vmi, err := waitVMIUntilRunning(virtualMachine.Namespace, virtualMachine.Name, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("kubevirt VMI '%s/%s' is ready, try to check its IP allocations", vmi.Namespace, vmi.Name)

		// 3. check the SpiderEndpoint resource IP allocations
		var endpoint spiderpoolv2beta1.SpiderEndpoint
		err = frame.KClient.Get(ctx, types.NamespacedName{
			Namespace: vmi.Namespace,
			Name:      vmi.Name,
		}, &endpoint)
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Printf("kubevirt VMI '%s/%s' IP allocations: %s", endpoint.Namespace, endpoint.Name, endpoint.Status.String())
		Expect(endpoint.Status.Current.IPs).To(HaveLen(2))
	})
})

func waitVMIUntilRunning(namespace, name string, timeout time.Duration) (*kubevirtv1.VirtualMachineInstance, error) {
	tick := time.Tick(timeout)
	var vmi kubevirtv1.VirtualMachineInstance

	for {
		select {
		case <-tick:
			ctx := context.TODO()
			GinkgoWriter.Printf("VMI %s/%s is still in phase %s \n", namespace, name, vmi.Status.Phase)
			vmiEvents, err := frame.GetEvents(ctx, constant.KindKubevirtVMI, name, namespace)
			if nil == err {
				for _, item := range vmiEvents.Items {
					GinkgoWriter.Printf("VMI %s/%s events: %s\n", namespace, name, item.String())
				}
			} else {
				GinkgoWriter.Printf("failed to get VMI %s/%s events, error: %v\n", namespace, name, err)
			}

			vmEvents, err := frame.GetEvents(ctx, constant.KindKubevirtVM, name, namespace)
			if nil == err {
				for _, item := range vmEvents.Items {
					GinkgoWriter.Printf("VM %s/%s events: %s\n", namespace, name, item.String())
				}
			} else {
				GinkgoWriter.Printf("failed to get VM %s/%s events, error: %v\n", namespace, name, err)
			}

			vmPodList, err := frame.GetPodList(client.MatchingLabels{
				kubevirtv1.VirtualMachineNameLabel: name,
			}, client.InNamespace(namespace))
			if nil == err {
				// only one Pod
				for _, tmpPod := range vmPodList.Items {
					podEvents, err := frame.GetEvents(ctx, constant.KindPod, tmpPod.Name, tmpPod.Namespace)
					if err == nil {
						for _, item := range podEvents.Items {
							GinkgoWriter.Printf("vm pod %s/%s events: %s\n", tmpPod.Namespace, tmpPod.Name, item.String())
						}
					}
				}
			}

			return nil, fmt.Errorf("time out to wait VMI %s/%s running", namespace, name)

		default:
			err := frame.GetResource(types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &vmi)
			if nil != err {
				if errors.IsNotFound(err) {
					time.Sleep(time.Second * 5)
					continue
				}

				return nil, err
			}
			if vmi.Status.Phase == kubevirtv1.Running {
				return &vmi, nil
			}
			time.Sleep(time.Second * 5)
		}
	}
}
