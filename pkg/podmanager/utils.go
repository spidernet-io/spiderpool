// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"fmt"

	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/multuscniconfig"
)

func IsPodAlive(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	if pod.DeletionTimestamp != nil {
		return false
	}

	if pod.Status.Phase == corev1.PodSucceeded && pod.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		return false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
		return false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
		return false
	}

	return true
}

// IsStaticIPPod checks the given pod's controller ownerReference whether is StatefulSet or KubevirtVMI
func IsStaticIPPod(enableStatefulSet, enableKubevirtStaticIP bool, pod *corev1.Pod) bool {
	ownerReference := metav1.GetControllerOf(pod)
	if ownerReference == nil {
		return false
	}

	if enableStatefulSet && ownerReference.APIVersion == appsv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindStatefulSet {
		return true
	}

	if enableKubevirtStaticIP && ownerReference.APIVersion == kubevirtv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindKubevirtVMI {
		return true
	}

	return false
}

// podNetworkMutatingWebhook handles the mutating webhook for pod networks.
// It checks if the pod has the required label for mutation, retrieves the corresponding
// SpiderMultusConfigs, and injects the network configuration into the pod.
//
// Parameters:
//   - apiReader: A client.Reader interface for accessing Kubernetes API objects
//   - pod: A pointer to the corev1.Pod object to be mutated
//
// Returns:
//   - An error if any step in the process fails, nil otherwise
func podNetworkMutatingWebhook(spiderClient crdclientset.Interface, pod *corev1.Pod) error {
	multusLabelValue, ok := pod.Annotations[constant.AnnoPodResourceInject]
	if !ok {
		return nil
	}

	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      constant.AnnoPodResourceInject,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{multusLabelValue},
			},
		},
	}

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector: %v", err)
	}

	multusConfigs, err := spiderClient.SpiderpoolV2beta1().SpiderMultusConfigs("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return err
	}

	if len(multusConfigs.Items) == 0 {
		return fmt.Errorf("No spidermultusconfig with label %v:%v found", constant.AnnoPodResourceInject, multusLabelValue)
	}

	return InjectPodNetwork(pod, *multusConfigs)
}

// injectPodNetwork injects network configurations into the pod based on the provided SpiderMultusConfigs.
// It checks for CNI type consistency, updates the pod's network attachment annotations,
// and prepares a map of resources to be injected.
//
// Parameters:
//   - pod: A pointer to the corev1.Pod object to be updated
//   - multusConfigs: A list of SpiderMultusConfig objects to be applied to the pod
//
// Returns:
//   - An error if there's an inconsistency in CNI types, nil otherwise
func InjectPodNetwork(pod *corev1.Pod, multusConfigs v2beta1.SpiderMultusConfigList) error {
	var cniType string
	resourcesMap := make(map[string]bool, len(multusConfigs.Items))
	for _, mc := range multusConfigs.Items {
		// Check the consistency of CNI type
		if cniType != "" && cniType != *mc.Spec.CniType {
			return fmt.Errorf("spidermultusconfig %s/%s cniType %s is not consistent with %s", mc.Namespace, mc.Name, *mc.Spec.CniType, cniType)
		} else {
			// If it's the first time setting, or consistent with the previous
			// type, update cniType
			cniType = *mc.Spec.CniType
		}

		if err := doValidateRdmaResouceAndIPPools(mc); err != nil {
			return err
		}

		// Update the pod's network attachment
		if networks, ok := pod.Annotations[constant.MultusNetworkAttachmentAnnot]; !ok {
			pod.Annotations[constant.MultusNetworkAttachmentAnnot] = fmt.Sprintf("%s/%s", mc.Namespace, mc.Name)
		} else {
			pod.Annotations[constant.MultusNetworkAttachmentAnnot] = networks + "," + fmt.Sprintf("%s/%s", mc.Namespace, mc.Name)
		}

		resourceName := multuscniconfig.ResourceName(&mc)
		if resourceName == "" {
			continue
		}

		if _, ok := resourcesMap[resourceName]; !ok {
			resourcesMap[resourceName] = false
		}
	}
	InjectRdmaResourceToPod(resourcesMap, pod)
	return nil
}

// injectRdmaResourceToPod injects RDMA resources into the pod's containers.
// It checks each container for existing resource requests/limits and updates
// the resourceMap accordingly. If a resource is not found in any container,
// it is injected into the first container's resource requests.
//
// Parameters:
//   - resourceMap: A map of resource names to boolean values indicating if they've been found
//   - pod: A pointer to the corev1.Pod object to be updated
func InjectRdmaResourceToPod(resourceMap map[string]bool, pod *corev1.Pod) {
	for _, c := range pod.Spec.Containers {
		for resource := range resourceMap {
			if resourceMap[resource] {
				// the resource has found in pod, skip
				continue
			}

			// try to find the resource in container resources.limits
			if _, ok := c.Resources.Limits[corev1.ResourceName(resource)]; ok {
				resourceMap[resource] = true
			}
		}
	}

	for resource, found := range resourceMap {
		if found {
			continue
		}
		if pod.Spec.Containers[0].Resources.Limits == nil {
			pod.Spec.Containers[0].Resources.Limits = make(corev1.ResourceList)
		}
		pod.Spec.Containers[0].Resources.Limits[corev1.ResourceName(resource)] = k8s_resource.MustParse("1")
	}
}

func doValidateRdmaResouceAndIPPools(mc v2beta1.SpiderMultusConfig) error {
	doValidateIPPools := func(name, namespace string, ippools *v2beta1.SpiderpoolPools) error {
		if ippools == nil {
			return fmt.Errorf("no any ippools config for spidermultusconfig %s/%s", namespace, name)
		}

		if len(ippools.IPv4IPPool)+len(ippools.IPv6IPPool) == 0 {
			return fmt.Errorf("no any ippools config for spidermultusconfig %s/%s", namespace, name)
		}
		return nil
	}

	spec := mc.Spec
	switch *spec.CniType {
	case constant.MacvlanCNI:
		if !spec.MacvlanConfig.EnableRdma {
			return fmt.Errorf("spidermultusconfig %s/%s not enable RDMA", mc.Namespace, mc.Name)
		}

		if spec.MacvlanConfig.RdmaResourceName == "" {
			return fmt.Errorf("rdmaResourceName can not empty for spidermultusconfig %s/%s", mc.Namespace, mc.Name)
		}

		return doValidateIPPools(mc.Name, mc.Namespace, spec.MacvlanConfig.SpiderpoolConfigPools)
	case constant.IPVlanCNI:
		if !spec.IPVlanConfig.EnableRdma {
			return fmt.Errorf("spidermultusconfig %s/%s not enable RDMA", mc.Namespace, mc.Name)
		}

		if spec.IPVlanConfig.RdmaResourceName == "" {
			return fmt.Errorf("rdmaResourceName can not empty for spidermultusconfig %s/%s", mc.Namespace, mc.Name)
		}

		return doValidateIPPools(mc.Name, mc.Namespace, spec.IPVlanConfig.SpiderpoolConfigPools)
	case constant.SriovCNI:
		if !spec.SriovConfig.EnableRdma {
			return fmt.Errorf("spidermultusconfig %s/%s not enable RDMA", mc.Namespace, mc.Name)
		}

		if spec.SriovConfig.ResourceName == "" {
			return fmt.Errorf("resourceName can not empty for spidermultusconfig %s/%s", mc.Namespace, mc.Name)
		}

		return doValidateIPPools(mc.Name, mc.Namespace, spec.SriovConfig.SpiderpoolConfigPools)
	case constant.IBSriovCNI:
		if spec.IbSriovConfig.ResourceName == "" {
			return fmt.Errorf("resourceName can not empty for spidermultusconfig %s/%s", mc.Namespace, mc.Name)
		}

		return doValidateIPPools(mc.Name, mc.Namespace, spec.IbSriovConfig.SpiderpoolConfigPools)
	case constant.IPoIBCNI:
		return doValidateIPPools(mc.Name, mc.Namespace, spec.IpoibConfig.SpiderpoolConfigPools)
	default:
		return fmt.Errorf("RDMA resource injection does not support cniType: %s", *spec.CniType)
	}
}
