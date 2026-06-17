// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package networkresourceplugin advertises Spiderpool scheduler-facing network
// resources through the kubelet device plugin API.
package networkresourceplugin

import (
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

const DefaultKubeletRootDir = "/var/lib/kubelet"

type Config struct {
	Enabled               bool
	KubeletRootDir        string
	DevicePluginAffinity  DevicePluginAffinityConfig
	ResourceAdvertisement ResourceAdvertisementConfig
}

type DevicePluginAffinityConfig struct {
	NodeSelector metav1.LabelSelector
}

type ResourceAdvertisementConfig struct {
	SubENI    SubENIAdvertisementConfig
	MasterNIC MasterNICAdvertisementConfig
}

type SubENIAdvertisementConfig struct {
	Rules []SubENIRuleConfig
}

type SubENIRuleConfig struct {
	ResourceName    string
	DefaultMaxCount int
	NodeSelector    metav1.LabelSelector
}

type MasterNICAdvertisementConfig struct {
	Rules []MasterNICRuleConfig
}

type MasterNICRuleConfig struct {
	NodeSelector      metav1.LabelSelector
	DefaultMaxCount   int
	IncludeInterfaces []string
	ExcludeInterfaces []string
}

func ApplyDefaultsAndValidate(cfg *spiderpooltypes.SpiderpoolConfigmapConfig) (*Config, error) {
	result := defaultConfig()
	if cfg != nil {
		applyConfigmap(result, cfg)
	}
	if err := validate(result); err != nil {
		return nil, err
	}
	return result, nil
}

func defaultConfig() *Config {
	return &Config{
		KubeletRootDir: DefaultKubeletRootDir,
	}
}

func applyConfigmap(result *Config, cfg *spiderpooltypes.SpiderpoolConfigmapConfig) {
	nrp := cfg.AgentConfig.NetworkResourcePlugin
	result.Enabled = nrp.Enabled
	if nrp.KubeletRootDir != "" {
		result.KubeletRootDir = nrp.KubeletRootDir
	}
	result.DevicePluginAffinity.NodeSelector = nrp.DevicePluginAffinity.NodeSelector

	result.ResourceAdvertisement.SubENI.Rules = make([]SubENIRuleConfig, 0, len(nrp.ResourceAdvertisement.SubENI.Rules))
	for _, entry := range nrp.ResourceAdvertisement.SubENI.Rules {
		if entry.ResourceName == "" {
			entry.ResourceName = constant.DefaultENISlotResourceName
		}
		result.ResourceAdvertisement.SubENI.Rules = append(result.ResourceAdvertisement.SubENI.Rules, SubENIRuleConfig{
			ResourceName:    entry.ResourceName,
			DefaultMaxCount: entry.DefaultMaxCount,
			NodeSelector:    copyLabelSelector(entry.NodeSelector),
		})
	}

	result.ResourceAdvertisement.MasterNIC.Rules = make([]MasterNICRuleConfig, 0, len(nrp.ResourceAdvertisement.MasterNIC.Rules))
	for _, rule := range nrp.ResourceAdvertisement.MasterNIC.Rules {
		r := MasterNICRuleConfig{
			NodeSelector:      copyLabelSelector(rule.NodeSelector),
			DefaultMaxCount:   rule.DefaultMaxCount,
			IncludeInterfaces: append([]string(nil), rule.IncludeInterfaces...),
			ExcludeInterfaces: append([]string(nil), rule.ExcludeInterfaces...),
		}
		result.ResourceAdvertisement.MasterNIC.Rules = append(result.ResourceAdvertisement.MasterNIC.Rules, r)
	}
}

func validate(cfg *Config) error {
	if cfg.KubeletRootDir == "" {
		cfg.KubeletRootDir = DefaultKubeletRootDir
	}
	cfg.KubeletRootDir = filepath.Clean(cfg.KubeletRootDir)
	if !filepath.IsAbs(cfg.KubeletRootDir) {
		return fmt.Errorf("%s.kubeletRootDir must be an absolute path", constant.NetworkResourcePluginConfigKey)
	}
	if _, err := metav1.LabelSelectorAsSelector(&cfg.DevicePluginAffinity.NodeSelector); err != nil {
		return fmt.Errorf("%s.devicePluginAffinity.nodeSelector is invalid: %w", constant.NetworkResourcePluginConfigKey, err)
	}

	for i := range cfg.ResourceAdvertisement.SubENI.Rules {
		entry := &cfg.ResourceAdvertisement.SubENI.Rules[i]
		if entry.ResourceName == "" {
			entry.ResourceName = constant.DefaultENISlotResourceName
		}
		if errs := validation.IsQualifiedName(entry.ResourceName); len(errs) != 0 {
			return fmt.Errorf("%s.resourceAdvertisement.subENI.rules[%d].resourceName %q is invalid: %s", constant.NetworkResourcePluginConfigKey, i, entry.ResourceName, strings.Join(errs, "; "))
		}
		if entry.DefaultMaxCount < 0 {
			return fmt.Errorf("%s.resourceAdvertisement.subENI.rules[%d].defaultMaxCount must be greater than or equal to 0", constant.NetworkResourcePluginConfigKey, i)
		}
		if _, err := metav1.LabelSelectorAsSelector(&entry.NodeSelector); err != nil {
			return fmt.Errorf("%s.resourceAdvertisement.subENI.rules[%d].nodeSelector is invalid: %w", constant.NetworkResourcePluginConfigKey, i, err)
		}
	}

	for i := range cfg.ResourceAdvertisement.MasterNIC.Rules {
		rule := &cfg.ResourceAdvertisement.MasterNIC.Rules[i]
		if rule.DefaultMaxCount < 0 {
			return fmt.Errorf("%s.resourceAdvertisement.masterNIC.rules[%d].defaultMaxCount must be greater than or equal to 0", constant.NetworkResourcePluginConfigKey, i)
		}
		if rule.DefaultMaxCount == 0 {
			rule.DefaultMaxCount = DefaultMasterNICMaxCount
		}
		if _, err := metav1.LabelSelectorAsSelector(&rule.NodeSelector); err != nil {
			return fmt.Errorf("%s.resourceAdvertisement.masterNIC.rules[%d].nodeSelector is invalid: %w", constant.NetworkResourcePluginConfigKey, i, err)
		}
		for _, pattern := range append(append([]string{}, rule.IncludeInterfaces...), rule.ExcludeInterfaces...) {
			if _, err := filepath.Match(pattern, "eth0"); err != nil {
				return fmt.Errorf("%s.resourceAdvertisement.masterNIC.rules[%d] pattern %q is invalid: %w", constant.NetworkResourcePluginConfigKey, i, pattern, err)
			}
		}
	}

	return nil
}

func copyLabelSelector(input metav1.LabelSelector) metav1.LabelSelector {
	if copied := input.DeepCopy(); copied != nil {
		return *copied
	}
	return metav1.LabelSelector{}
}

func ResourceList(resourceName string, count int) corev1.ResourceList {
	if count <= 0 {
		return nil
	}
	return corev1.ResourceList{corev1.ResourceName(resourceName): resource.MustParse(fmt.Sprintf("%d", count))}
}
