// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package parentnic reports the physical NICs of the local node to the Node
// annotation ipam.spidernet.io/parent-nics, so that the external IaaS network
// provider can locate the parent port of each NIC by MAC address.
package parentnic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
)

// linkLister and sysClassNetPath are indirections for unit tests.
var (
	linkLister      = netlink.LinkList
	sysClassNetPath = networking.SysClassNetDevicePath
)

// ListPhysicalNics returns a map of physical NIC name to MAC address on the
// local (host) network namespace. Virtual interfaces (veth, bridge, vlan,
// bond, dummy, loopback, etc.) are filtered out by requiring a backing device
// in /sys/class/net/<nic>/device. NICs listed in excludeNics are skipped.
func ListPhysicalNics(excludeNics []string) (map[string]string, error) {
	links, err := linkLister()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	excluded := make(map[string]struct{}, len(excludeNics))
	for _, name := range excludeNics {
		excluded[name] = struct{}{}
	}

	nics := make(map[string]string)
	for _, link := range links {
		attrs := link.Attrs()
		if attrs == nil {
			continue
		}
		if _, ok := excluded[attrs.Name]; ok {
			continue
		}
		if len(attrs.HardwareAddr) == 0 {
			continue
		}
		// Physical NICs (including virtio/SR-IOV in VMs) have a backing
		// device symlink in sysfs; virtual interfaces do not.
		if _, err := os.Stat(path.Join(sysClassNetPath, attrs.Name, "device")); err != nil {
			continue
		}
		nics[attrs.Name] = attrs.HardwareAddr.String()
	}

	return nics, nil
}

// ReportParentNics collects the local physical NICs and writes them to the
// Node annotation ipam.spidernet.io/parent-nics as a JSON map of NIC name to
// MAC address. It is intended to be called once at spiderpool-agent startup
// when the IaaS network provider integration is enabled.
func ReportParentNics(ctx context.Context, clientSet kubernetes.Interface, nodeName string, excludeNics []string, logger *zap.Logger) error {
	nics, err := ListPhysicalNics(excludeNics)
	if err != nil {
		return err
	}
	if len(nics) == 0 {
		return fmt.Errorf("no physical NIC found on node %s (excludeReportNics: %v)", nodeName, excludeNics)
	}

	nicsJSON, err := json.Marshal(nics)
	if err != nil {
		return fmt.Errorf("failed to marshal parent NICs: %w", err)
	}

	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				constant.AnnoNodeParentNics: string(nicsJSON),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal node annotation patch: %w", err)
	}

	if _, err := clientSet.CoreV1().Nodes().Patch(ctx, nodeName, apitypes.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("failed to patch annotation %s of Node %s: %w", constant.AnnoNodeParentNics, nodeName, err)
	}

	logger.Sugar().Infof("Reported parent NICs to annotation %s of Node %s: %s", constant.AnnoNodeParentNics, nodeName, string(nicsJSON))
	return nil
}
