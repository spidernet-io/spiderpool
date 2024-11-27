// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"fmt"

	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8s_resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionClientv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
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
		return fmt.Errorf("No spidermultusconfigs with annotation: %v:%v found", constant.AnnoPodResourceInject, multusLabelValue)
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
	resourcesMap := make(map[string]bool, len(multusConfigs.Items))
	for _, mc := range multusConfigs.Items {
		if err := DoValidateRdmaResouce(mc); err != nil {
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

// InitPodMutatingWebhook initializes a mutating webhook for pods based on a template webhook.
// It sets up the webhook configuration including name, admission review versions, failure policy,
// object selector, client config, and rules for pod creation and update operations.
//
// Parameters:
//   - from: An admissionregistrationv1.MutatingWebhook object to use as a template
//
// Returns:
//   - A new admissionregistrationv1.MutatingWebhook object configured for pod mutation
func InitPodMutatingWebhook(from admissionregistrationv1.MutatingWebhook, webhookNamespaceInclude []string) admissionregistrationv1.MutatingWebhook {
	wb := admissionregistrationv1.MutatingWebhook{
		Name:                    constant.PodMutatingWebhookName,
		AdmissionReviewVersions: from.AdmissionReviewVersions,
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		NamespaceSelector:       &metav1.LabelSelector{},
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			CABundle: from.ClientConfig.CABundle,
		},
		Rules: []admissionregistrationv1.RuleWithOperations{
			{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		},
		SideEffects: ptr.To(admissionregistrationv1.SideEffectClassNone),
	}

	if from.ClientConfig.Service != nil {
		wb.ClientConfig.Service = &admissionregistrationv1.ServiceReference{
			Name:      from.ClientConfig.Service.Name,
			Namespace: from.ClientConfig.Service.Namespace,
			Port:      from.ClientConfig.Service.Port,
			// format: /mutate-<group>-<apiVersion>-<resource>
			Path: ptr.To("/mutate--v1-pod"),
		}
	}

	if len(PodWebhookExcludeNamespaces) != 0 {
		wb.NamespaceSelector.MatchExpressions = []metav1.LabelSelectorRequirement{
			{
				Key:      corev1.LabelMetadataName,
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   PodWebhookExcludeNamespaces,
			},
		}
	}

	if len(webhookNamespaceInclude) != 0 {
		wb.NamespaceSelector.MatchExpressions = append(wb.NamespaceSelector.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      corev1.LabelMetadataName,
			Operator: metav1.LabelSelectorOpIn,
			Values:   webhookNamespaceInclude,
		})
	}
	return wb
}

// addPodMutatingWebhook updates the MutatingWebhookConfiguration for pods.
// It retrieves the existing configuration, adds a new webhook for pods,
// and updates the configuration in the Kubernetes API server.
func AddPodMutatingWebhook(admissionClient admissionClientv1.AdmissionregistrationV1Interface, mutatingWebhookName string, webhookNamespaceInclude []string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mwc, err := admissionClient.MutatingWebhookConfigurations().Get(context.TODO(), mutatingWebhookName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get MutatingWebhookConfiguration: %v", err)
		}

		if len(mwc.Webhooks) == 0 {
			return fmt.Errorf("no any mutating webhook found in MutatingWebhookConfiguration %s", mutatingWebhookName)
		}

		var newWebhooks []admissionregistrationv1.MutatingWebhook
		for _, wb := range mwc.Webhooks {
			// if the webhook already exists, do nothing
			if wb.Name == constant.PodMutatingWebhookName {
				continue
			}
			newWebhooks = append(newWebhooks, wb)
		}

		podWebhook := InitPodMutatingWebhook(*mwc.Webhooks[0].DeepCopy(), webhookNamespaceInclude)
		newWebhooks = append(newWebhooks, podWebhook)
		mwc.Webhooks = newWebhooks

		_, updateErr := admissionClient.MutatingWebhookConfigurations().Update(context.TODO(), mwc, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		return fmt.Errorf("update MutatingWebhookConfiguration %s failed: %v", mutatingWebhookName, retryErr)
	}

	return nil
}

// RemovePodMutatingWebhook removes the mutating webhook for pods.
// It retrieves the existing configuration, removes the webhook for pods,
// and updates the configuration in the Kubernetes API server.
func RemovePodMutatingWebhook(admissionClient admissionClientv1.AdmissionregistrationV1Interface, mutatingWebhookName string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mwc, err := admissionClient.MutatingWebhookConfigurations().Get(context.TODO(), mutatingWebhookName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		var newWebhooks []admissionregistrationv1.MutatingWebhook
		for _, wb := range mwc.Webhooks {
			if wb.Name != constant.PodMutatingWebhookName {
				newWebhooks = append(newWebhooks, wb)
			}
		}

		if len(newWebhooks) == len(mwc.Webhooks) {
			return nil
		}

		mwc.Webhooks = newWebhooks
		_, err = admissionClient.MutatingWebhookConfigurations().Update(context.TODO(), mwc, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		return fmt.Errorf("removes the mutating webhook for pods: %v", retryErr)
	}
	return nil
}

func DoValidateRdmaResouce(mc v2beta1.SpiderMultusConfig) error {
	spec := mc.Spec
	switch *spec.CniType {
	case constant.MacvlanCNI:
		return multuscniconfig.ValidateRdmaResouce(spec.MacvlanConfig.EnableRdma, mc.Name, mc.Namespace, spec.MacvlanConfig.RdmaResourceName, spec.MacvlanConfig.SpiderpoolConfigPools)
	case constant.IPVlanCNI:
		return multuscniconfig.ValidateRdmaResouce(spec.IPVlanConfig.EnableRdma, mc.Name, mc.Namespace, spec.IPVlanConfig.RdmaResourceName, spec.IPVlanConfig.SpiderpoolConfigPools)
	case constant.SriovCNI:
		return multuscniconfig.ValidateRdmaResouce(spec.SriovConfig.EnableRdma, mc.Name, mc.Namespace, spec.SriovConfig.ResourceName, spec.SriovConfig.SpiderpoolConfigPools)
	case constant.IBSriovCNI:
		return multuscniconfig.ValidateRdmaResouce(true, mc.Name, mc.Namespace, spec.IbSriovConfig.ResourceName, spec.IbSriovConfig.SpiderpoolConfigPools)
	case constant.IPoIBCNI:
		return multuscniconfig.ValidateRdmaResouce(true, mc.Name, mc.Namespace, spec.IpoibConfig.Master, spec.IpoibConfig.SpiderpoolConfigPools)
	default:
		return fmt.Errorf("RDMA resource injection does not support cniType: %s", *spec.CniType)
	}
}
