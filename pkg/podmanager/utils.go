// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"encoding/json"
	"fmt"

	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	k8s_resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/multuscniconfig"
	spidertypes "github.com/spidernet-io/spiderpool/pkg/types"
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
func podNetworkMutatingWebhook(spiderClient crdclientset.Interface, client client.Client, pod *corev1.Pod) error {
	var multusConfigs *v2beta1.SpiderMultusConfigList
	for _, anno := range []string{constant.AnnoPodResourceInject, constant.AnnoNetworkResourceInject} {
		multusLabelValue, ok := pod.Annotations[anno]
		if !ok {
			continue
		}
		labelSelector := metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      anno,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{multusLabelValue},
				},
			},
		}

		selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		if err != nil {
			return fmt.Errorf("failed to create label selector: %v", err)
		}

		multusConfigs, err = spiderClient.SpiderpoolV2beta1().SpiderMultusConfigs("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return err
		}

		if len(multusConfigs.Items) == 0 {
			return fmt.Errorf("No spidermultusconfigs with annotation: %v:%v found", anno, multusLabelValue)
		}

		return InjectPodNetwork(pod, *multusConfigs)

	}
	return nil
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
	resourcesMap := make(map[string]bool, len(multusConfigs.Items))

	multusAnnValue := ""
	for _, mc := range multusConfigs.Items {
		if err := DoValidateRdmaResouce(mc); err != nil {
			return err
		}

		if multusAnnValue == "" {
			multusAnnValue = fmt.Sprintf("%s/%s", mc.Namespace, mc.Name)
		} else {
			multusAnnValue += "," + fmt.Sprintf("%s/%s", mc.Namespace, mc.Name)
		}

		resourceName := multuscniconfig.ResourceName(&mc)
		if resourceName == "" {
			continue
		}

		if _, ok := resourcesMap[resourceName]; !ok {
			resourcesMap[resourceName] = false
		}
	}

	if multusAnnValue != "" {
		pod.Annotations[constant.MultusNetworkAttachmentAnnot] = multusAnnValue
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

func DoValidateRdmaResouce(mc v2beta1.SpiderMultusConfig) error {
	spec := mc.Spec
	switch *spec.CniType {
	case constant.MacvlanCNI:
		return multuscniconfig.ValidateRdmaResouce(mc.Name, mc.Namespace, *spec.MacvlanConfig.RdmaResourceName, spec.MacvlanConfig.SpiderpoolConfigPools)
	case constant.IPVlanCNI:
		return multuscniconfig.ValidateRdmaResouce(mc.Name, mc.Namespace, *spec.IPVlanConfig.RdmaResourceName, spec.IPVlanConfig.SpiderpoolConfigPools)
	case constant.SriovCNI:
		return multuscniconfig.ValidateRdmaResouce(mc.Name, mc.Namespace, *spec.SriovConfig.ResourceName, spec.SriovConfig.SpiderpoolConfigPools)
	case constant.IBSriovCNI:
		return multuscniconfig.ValidateRdmaResouce(mc.Name, mc.Namespace, *spec.IbSriovConfig.ResourceName, spec.IbSriovConfig.SpiderpoolConfigPools)
	case constant.IPoIBCNI:
		return multuscniconfig.ValidateRdmaResouce(mc.Name, mc.Namespace, spec.IpoibConfig.Master, spec.IpoibConfig.SpiderpoolConfigPools)
	default:
		return fmt.Errorf("RDMA resource injection does not support cniType: %s", *spec.CniType)
	}
}

// InjectPodNetworkFromResourceClaim injects network configurations into the pod based on the provided ResourceClaim.
// Note: we expect the ResourceClaim or ResourceClaimTemplate has been created when the pod mutating webhook. Or we
// may hit the "not found" error.
func InjectPodNetworkFromResourceClaim(client client.Client, pod *corev1.Pod) error {
	var multusConfigName []string
	var parameter spidertypes.ParameterConfig
	getStaticNics := func(spec resourcev1beta1.ResourceClaimSpec) error {
		fmt.Printf("ResourceClaimSpec: %v\n", spec)
		for _, req := range spec.Devices.Requests {
			// only care our device class
			if req.DeviceClassName == constant.DRACNIDeviceClass {
				multusConfigName = append(multusConfigName, req.Name)
			}
		}

		if len(multusConfigName) > 0 {
			for _, config := range spec.Devices.Config {
				if config.DeviceConfiguration.Opaque.Driver != constant.DRADriverName {
					continue
				}
				if err := json.Unmarshal(config.DeviceConfiguration.Opaque.Parameters.Raw, &parameter); err != nil {
					return err
				}
				break
			}
		}
		return nil
	}

	for _, resourceClaim := range pod.Spec.ResourceClaims {
		// Exactly one of ResourceClaimName and ResourceClaimTemplateName must be set.
		fmt.Printf("ResourceClaim: %v\n", resourceClaim)
		if resourceClaim.ResourceClaimTemplateName != nil && *resourceClaim.ResourceClaimTemplateName != "" {
			rct := resourcev1beta1.ResourceClaimTemplate{}
			if err := client.Get(context.TODO(), types.NamespacedName{Namespace: pod.Namespace, Name: *resourceClaim.ResourceClaimTemplateName}, &rct); err != nil {
				return err
			}

			if err := getStaticNics(rct.Spec.Spec); err != nil {
				return err
			}

			if len(multusConfigName) > 0 {
				break
			}
		}

		if resourceClaim.ResourceClaimName != nil && *resourceClaim.ResourceClaimName != "" {
			rct := resourcev1beta1.ResourceClaim{}
			if err := client.Get(context.TODO(), types.NamespacedName{Namespace: pod.Namespace, Name: *resourceClaim.ResourceClaimName}, &rct); err != nil {
				return err
			}

			if err := getStaticNics(rct.Spec); err != nil {
				return err
			}

			// found the multus config name
			if len(multusConfigName) > 0 {
				break
			}
		}
	}

	if len(multusConfigName) == 0 {
		return fmt.Errorf("No multus config found from resource claim of pod %s/%s", pod.Namespace, pod.GenerateName)
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	resourcesMap := make(map[string]bool)
	for idx, mc := range multusConfigName {
		// Update the pod's network attachment
		var smc v2beta1.SpiderMultusConfig
		if err := client.Get(context.TODO(), types.NamespacedName{Namespace: parameter.MultusNamaspace, Name: mc}, &smc); err != nil {
			return err
		}

		smcName := smc.Name
		if smc.Annotations[constant.AnnoNetAttachConfName] != "" {
			smcName = smc.Annotations[constant.AnnoNetAttachConfName]
		}

		resourceName := multuscniconfig.ResourceName(&smc)
		if resourceName != "" {
			resourcesMap[resourceName] = false
		}

		if idx == 0 {
			pod.Annotations[constant.MultusDefaultNetAnnot] = fmt.Sprintf("%s/%s", smc.Namespace, smcName)
			continue
		}

		if networks, ok := pod.Annotations[constant.MultusNetworkAttachmentAnnot]; !ok {
			pod.Annotations[constant.MultusNetworkAttachmentAnnot] = fmt.Sprintf("%s/%s", smc.Namespace, smcName)
		} else {
			pod.Annotations[constant.MultusNetworkAttachmentAnnot] = networks + "," + fmt.Sprintf("%s/%s", smc.Namespace, smcName)
		}
	}

	if parameter.PodDefaultRouteNic != "" {
		pod.Annotations[constant.AnnoDefaultRouteInterface] = parameter.PodDefaultRouteNic
	}
	InjectRdmaResourceToPod(resourcesMap, pod)
	return nil
}
