// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	corev1 "k8s.io/api/core/v1"
)

type DesiredResource struct {
	ResourceName string
	Devices      int
	Interface    string
}

func ComputeDesiredResources(providerEnabled bool, node *corev1.Node, interfaces []string, cfg Config) ([]DesiredResource, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	matched, err := nodeSelectorMatches(node, &cfg.DevicePluginAffinity.NodeSelector)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}

	result := []DesiredResource{}
	subENI := cfg.ResourceAdvertisement.SubENI
	if providerEnabled && len(subENI.Rules) > 0 {
		selectedResourceNames := map[string]struct{}{}
		for _, entry := range subENI.Rules {
			matched, err := nodeSelectorMatches(node, &entry.NodeSelector)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
			if _, ok := selectedResourceNames[entry.ResourceName]; ok {
				continue
			}
			selectedResourceNames[entry.ResourceName] = struct{}{}
			result = append(result, DesiredResource{ResourceName: entry.ResourceName, Devices: entry.DefaultMaxCount})
		}
	}

	nics, err := selectMasterNICs(node, interfaces, cfg.ResourceAdvertisement.MasterNIC)
	if err != nil {
		return nil, err
	}
	for _, nic := range nics {
		result = append(result, DesiredResource{
			ResourceName: masterNICResourceName(nic.Interface),
			Devices:      nic.Devices,
			Interface:    nic.Interface,
		})
	}
	return result, nil
}
