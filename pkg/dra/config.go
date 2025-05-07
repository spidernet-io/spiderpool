package dra

import (
	"encoding/json"
	"fmt"

	resourcev1beta1 "k8s.io/api/resource/v1beta1"
)

type NetworkConfig struct {
	// MultusNamespace is the namespace where the MultusConfig CRs are located
	MultusNamespace string `json:"multusNamespace"`
	// DefaultNic is the default MultusConfig to be used, usually the primary network interface
	// in k8s, the default network interface is usually named "eth0"
	DefaultNic *MultusConfig `json:"defaultNic"`
	// SecondaryNics is the secondary MultusConfig to be used
	// usually the secondary network interface is usually named "net1"
	SecondaryNics *SecondaryNic `json:"secondaryNics"`
}

type SecondaryNic struct {
	// StaticNics is the static MultusConfig to be used
	StaticNics []*MultusConfig `json:"staticNics"`
	// DynamicNics is the dynamic MultusConfig to be used via
	// gpu affinity
	DynamicNics *DynamicNic `json:"dynamicNics"`
}

type DynamicNic struct {
	// GPUAffinityPolicy can be "best" or "all"
	GPUAffinityPolicy string `json:"gpuAffinityPolicy"`
	// PotentialMultusConfigs is a list of MultusConfig names that the dynamic NIC can be allocated to
	// empty means all MultusConfig can be used.
	PotentialMultusConfigs []string `json:"potentialMultusConfigs"`
}

type MultusConfig struct {
	// MultusName is the name of the MultusConfig
	MultusName string `json:"multusName"`
	// DefaultRoute is whether the MultusConfig is the default route
	DefaultRoute bool `json:"defaultRoute"`
}

// ParseNetworkConfig gets the network config from resource claim opaqueConfig
func ParseNetworkConfig(configs []resourcev1beta1.DeviceClaimConfiguration) (*NetworkConfig, error) {
	// parse the resourceclaim network config
	var multusConfig *NetworkConfig
	for _, config := range configs {
		if config.DeviceConfiguration.Opaque.Driver != "OUR_DRADRIVER_NAME" {
			continue
		}
		if config.DeviceConfiguration.Opaque == nil {
			continue
		}

		if err := json.Unmarshal(config.DeviceConfiguration.Opaque.Parameters.Raw, &multusConfig); err != nil {
			return nil, err
		}
		break
	}

	if multusConfig == nil {
		return nil, fmt.Errorf("failed to get network config from resource claim")
	}

	return multusConfig, nil
}

// func ParseToAnnotations(annotations map[string]string) {
// 	if annotations == nil {
// 		annotations = make(map[string]string)
// 	}

// 	if d.DefaultNic != nil {
// 		annotations[constant.MultusDefaultNetAnnot] = MultusAnnotationValue(d.MultusNamespace, d.DefaultNic.MultusName)
// 	}

// 	if d.SecondaryNics != nil {
// 		for idx, nic := range d.SecondaryNics.StaticNics {
// 			if nic == nil {
// 				continue
// 			}
// 			// by default, the default route is locatee at the first nic of the pod(eth0).
// 			// we can configure the default route to the second nics of the pod via annotations
// 			// e.g.
// 			// annotations:
// 			//   ipam.spidernet.io/default-route-nic: net1
// 			// In multus, the multi-nic is formatted as "net1", "net2", etc.
// 			// Note: we expect only one nic to be the default route, if we configure DefaultRoute to
// 			// true for multi-nic, the first nic only be selected.
// 			if nic.DefaultRoute && annotations[constant.AnnoDefaultRouteInterface] == "" {
// 				annotations[constant.AnnoDefaultRouteInterface] = fmt.Sprintf("net%d", idx+1)
// 			}

// 			if idx == 0 {
// 				annotations[constant.MultusNetworkAttachmentAnnot] = MultusAnnotationValue(d.MultusNamespace, nic.MultusName)
// 				continue
// 			}
// 			annotations[constant.MultusNetworkAttachmentAnnot] = annotations[constant.MultusNetworkAttachmentAnnot] + "," + MultusAnnotationValue(d.MultusNamespace, nic.MultusName)
// 		}
// 	}
// }

func (d *NetworkConfig) GetResourceNames() []string {
	return nil
}

func MultusAnnotationValue(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
