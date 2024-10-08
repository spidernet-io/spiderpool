// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package utils

import (
	"context"
	"fmt"
	"os"

	init_cmd "github.com/spidernet-io/spiderpool/cmd/spiderpool-init/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	corev1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	k8s_resource "k8s.io/apimachinery/pkg/api/resource"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetStaticNicsFromSpiderClaimParameter(ctx context.Context, apiReader client.Reader, pod *corev1.Pod) ([]spiderpoolv2beta1.MultusConfig, error) {
	for _, rc := range pod.Spec.ResourceClaims {
		if rc.Source.ResourceClaimTemplateName != nil {
			var rct resourcev1alpha2.ResourceClaimTemplate
			if err := apiReader.Get(ctx, apitypes.NamespacedName{Namespace: pod.Namespace, Name: *rc.Source.ResourceClaimTemplateName}, &rct); err != nil {
				return nil, err
			}

			if rct.Spec.Spec.ResourceClassName == constant.DRADriverName && rct.Spec.Spec.ParametersRef.APIGroup == constant.SpiderpoolAPIGroup &&
				rct.Spec.Spec.ParametersRef.Kind == constant.KindSpiderClaimParameter {

				var scp spiderpoolv2beta1.SpiderClaimParameter
				if err := apiReader.Get(ctx, apitypes.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}, &scp); err != nil {
					return nil, fmt.Errorf("failed to get spiderClaimParameter for pod %s/%s: %v", pod.Namespace, pod.Name, err)
				}

				var multusConfigs []spiderpoolv2beta1.MultusConfig
				if scp.Spec.DefaultNic != nil {
					multusConfigs = append(multusConfigs, *scp.Spec.DefaultNic)
				}
				multusConfigs = append(multusConfigs, scp.Spec.SecondaryNics...)

				return multusConfigs, nil
			}
		}
	}
	return []spiderpoolv2beta1.MultusConfig{}, nil
}

func GetRdmaResourceMapFromStaticNics(ctx context.Context, apiReader client.Reader, staticNics []spiderpoolv2beta1.MultusConfig) (map[string]bool, error) {
	resourceMap := make(map[string]bool)
	for _, nic := range staticNics {
		if nic.Namespace == "" {
			nic.Namespace = os.Getenv(init_cmd.ENVNamespace)
		}

		var smc spiderpoolv2beta1.SpiderMultusConfig
		if err := apiReader.Get(ctx, apitypes.NamespacedName{Namespace: nic.Namespace, Name: nic.MultusName}, &smc); err != nil {
			return nil, fmt.Errorf("failed to get spiderClaimParameter: %v", err)
		}

		resourceName := resourceName(&smc)
		if resourceName == "" {
			continue
		}

		if _, ok := resourceMap[resourceName]; !ok {
			resourceMap[resourceName] = false
		}
	}
	return resourceMap, nil
}

// resourceName return the resourceName for given spiderMultusConfig
func resourceName(smc *spiderpoolv2beta1.SpiderMultusConfig) string {
	switch *smc.Spec.CniType {
	case constant.MacvlanCNI:
		if smc.Spec.MacvlanConfig != nil && smc.Spec.MacvlanConfig.EnableRdma {
			return smc.Spec.MacvlanConfig.RdmaResourceName
		}
	case constant.IPVlanCNI:
		if smc.Spec.IPVlanConfig != nil && smc.Spec.IPVlanConfig.EnableRdma {
			return smc.Spec.IPVlanConfig.RdmaResourceName
		}
	case constant.SriovCNI:
		if smc.Spec.SriovConfig != nil {
			return smc.Spec.SriovConfig.ResourceName
		}
	case constant.IBSriovCNI:
		if smc.Spec.IbSriovConfig != nil {
			return smc.Spec.IbSriovConfig.ResourceName
		}
	}
	return ""
}

func InjectRdmaResourceToPod(resourceMap map[string]bool, pod *corev1.Pod) {
	for _, c := range pod.Spec.Containers {
		for resource := range resourceMap {
			if resourceMap[resource] {
				// the resource has found in pod, skip
				continue
			}

			// try to find the resource in container resources.requests
			if _, ok := c.Resources.Requests[corev1.ResourceName(resource)]; ok {
				resourceMap[resource] = true
			} else {
				if _, ok := c.Resources.Limits[corev1.ResourceName(resource)]; ok {
					resourceMap[resource] = true
				}
			}
		}
	}

	for resource, found := range resourceMap {
		if !found {
			// inject the resource to the pod.containers[0].resources.requests
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(resource)] = k8s_resource.MustParse("1")
		}
	}
}
