// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type InterfaceDiscoverer interface {
	Interfaces(includeVirtual bool) ([]string, error)
}

type MasterNICSelection struct {
	Interface string
	Devices   int
}

type NetInterfaceDiscoverer struct{}

func (NetInterfaceDiscoverer) Interfaces(includeVirtual bool) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if isMasterInterfaceCandidate(iface.Name, iface.Flags) && (includeVirtual || hasPhysicalDevice(iface.Name)) {
			names = append(names, iface.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func isMasterInterfaceCandidate(name string, flags net.Flags) bool {
	if name == "" || flags&net.FlagLoopback != 0 {
		return false
	}
	for _, prefix := range []string{"cni", "flannel", "veth", "docker", "br-", "virbr", "tun", "tap"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

func hasPhysicalDevice(name string) bool {
	if _, err := os.Stat(filepath.Join("/sys/class/net", name, "device")); err != nil {
		return false
	}
	return true
}

func masterNICRulesUseExplicitIncludes(cfg MasterNICAdvertisementConfig) bool {
	for i := range cfg.Rules {
		if len(cfg.Rules[i].IncludeInterfaces) > 0 {
			return true
		}
	}
	return false
}

func selectMasterNICs(node *corev1.Node, interfaces []string, cfg MasterNICAdvertisementConfig) ([]MasterNICSelection, error) {
	if len(cfg.Rules) == 0 {
		return nil, nil
	}

	selected := map[string]int{}
	matched := false
	for i := range cfg.Rules {
		rule := cfg.Rules[i]
		nodeMatched, err := nodeSelectorMatches(node, &rule.NodeSelector)
		if err != nil {
			return nil, err
		}
		if !nodeMatched {
			continue
		}
		matched = true
		defaultMaxCount := rule.DefaultMaxCount
		if defaultMaxCount == 0 {
			defaultMaxCount = DefaultMasterNICMaxCount
		}
		included := map[string]struct{}{}
		for _, iface := range interfaces {
			if len(rule.IncludeInterfaces) == 0 || matchAny(rule.IncludeInterfaces, iface) {
				included[iface] = struct{}{}
			}
		}
		for iface := range included {
			if matchAny(rule.ExcludeInterfaces, iface) {
				continue
			}
			if selected[iface] < defaultMaxCount {
				selected[iface] = defaultMaxCount
			}
		}
	}
	if !matched {
		return nil, nil
	}

	selectedInterfaces := make([]string, 0, len(selected))
	for iface := range selected {
		selectedInterfaces = append(selectedInterfaces, iface)
	}
	sort.Strings(selectedInterfaces)
	result := make([]MasterNICSelection, 0, len(selectedInterfaces))
	for _, iface := range selectedInterfaces {
		result = append(result, MasterNICSelection{Interface: iface, Devices: selected[iface]})
	}
	return result, nil
}

func nodeSelectorMatches(node *corev1.Node, selector *metav1.LabelSelector) (bool, error) {
	if selector == nil {
		return true, nil
	}
	compiled, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false, err
	}
	nodeLabels := labels.Set{}
	if node != nil {
		nodeLabels = labels.Set(node.Labels)
	}
	return compiled.Matches(nodeLabels), nil
}

func matchAny(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
	}
	return false
}

func masterNICResourceName(iface string) string {
	return constant.SpiderpoolResourceDomain + "/" + iface + constant.MasterNICResourceSuffix
}
