// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

// Config is the defaulted and validated runtime configuration for the ENI slot
// device plugin and webhook resource injection.
type Config struct {
	Enabled               bool
	ResourceName          string
	MaxSlotsPerNode       int
	KubeletRootDir        string
	InjectPodENIResources bool
}

const DefaultKubeletRootDir = "/var/lib/kubelet"

// ApplyDefaultsAndValidate normalizes iaasNetworkProvider.eniDevPlugin and
// validates settings that affect kubelet device plugin registration.
func ApplyDefaultsAndValidate(cfg *spiderpooltypes.IaaSProviderConfig) (*Config, error) {
	if cfg == nil {
		return &Config{
			ResourceName:          constant.DefaultENISlotResourceName,
			KubeletRootDir:        DefaultKubeletRootDir,
			InjectPodENIResources: true,
		}, nil
	}

	if cfg.ENIDevPlugin.ResourceName == "" {
		cfg.ENIDevPlugin.ResourceName = constant.DefaultENISlotResourceName
	}
	if cfg.ENIDevPlugin.InjectPodENIResources == nil {
		defaultInject := true
		cfg.ENIDevPlugin.InjectPodENIResources = &defaultInject
	}
	if cfg.ENIDevPlugin.KubeletRootDir == "" {
		cfg.ENIDevPlugin.KubeletRootDir = DefaultKubeletRootDir
	}

	result := &Config{
		Enabled:               cfg.ENIDevPlugin.Enabled,
		ResourceName:          cfg.ENIDevPlugin.ResourceName,
		MaxSlotsPerNode:       cfg.ENIDevPlugin.MaxSlotsPerNode,
		KubeletRootDir:        filepath.Clean(cfg.ENIDevPlugin.KubeletRootDir),
		InjectPodENIResources: *cfg.ENIDevPlugin.InjectPodENIResources,
	}

	if result.MaxSlotsPerNode < 0 {
		return nil, fmt.Errorf("%s.maxSlotsPerNode must be greater than or equal to 0", constant.IaaSProviderENIDevPluginConfigKey)
	}
	if !filepath.IsAbs(result.KubeletRootDir) {
		return nil, fmt.Errorf("%s.kubeletRootDir must be an absolute path", constant.IaaSProviderENIDevPluginConfigKey)
	}
	if errs := validation.IsQualifiedName(result.ResourceName); len(errs) != 0 {
		return nil, fmt.Errorf("%s.resourceName %q is invalid: %s", constant.IaaSProviderENIDevPluginConfigKey, result.ResourceName, strings.Join(errs, "; "))
	}

	return result, nil
}
