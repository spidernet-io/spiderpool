// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	iaasclient "github.com/spidernet-io/spiderpool/pkg/iaas/client"
	iaasutils "github.com/spidernet-io/spiderpool/pkg/iaas/utils"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

// callIaaSAllocate calls the IaaS provider API to allocate IPs
func (i *ipam) callIaaSAllocate(ctx context.Context, pod *corev1.Pod, results []*spiderpooltypes.AllocationResult) (*iaasclient.AllocateIPResponse, error) {
	if i.config.IaaSClient == nil {
		return nil, nil
	}

	logger := logutils.FromContext(ctx).With(
		zap.String("pod", pod.Name),
		zap.String("namespace", pod.Namespace),
	)

	// Build IaaS allocation request
	req := &iaasclient.AllocateIPRequest{
		PodName:      pod.Name,
		PodNamespace: pod.Namespace,
		PodUID:       string(pod.UID),
		NodeName:     pod.Spec.NodeName,
	}

	// Build IP-to-result index while constructing the request, so we can later
	// merge the IaaS response back into results in O(1) per item.
	// result.IP.Address is a CIDR string like "10.0.0.1/24"
	ipToResult := make(map[string]*spiderpooltypes.AllocationResult, len(results))
	for _, result := range results {
		if result == nil || result.IP == nil || result.IP.Address == nil || result.IP.Nic == nil {
			logger.Error("Skipping nil or incomplete allocation result")
			return nil, fmt.Errorf("nil or incomplete allocation result")
		}
		ip, ipNet, err := net.ParseCIDR(*result.IP.Address)
		if err != nil {
			logger.Error("Failed to parse IP address", zap.String("address", *result.IP.Address), zap.Error(err))
			return nil, fmt.Errorf("failed to parse IP address: %w", err)
		}
		subnet := ipNet.String()
		parentMac, err := i.getParentNicMacFromMultus(ctx, pod, *result.IP.Nic, subnet)
		if err != nil {
			logger.Error("Failed to get parent NIC MAC", zap.String("nic", *result.IP.Nic), zap.Error(err))
			return nil, fmt.Errorf("failed to get parent NIC MAC: %w", err)
		}
		ipStr := ip.String()
		ipToResult[ipStr] = result

		req.IaaSIPsAllocationRequest = append(req.IaaSIPsAllocationRequest, iaasclient.IaaSIPAllocationItem{
			IPAddress:    ipStr,
			Subnet:       subnet,
			ParentNicMac: parentMac,
		})
	}

	logger.Debug("Calling IaaS allocate API",
		zap.String("podUID", string(pod.UID)),
		zap.String("nodeName", pod.Spec.NodeName),
		zap.Any("request", req.IaaSIPsAllocationRequest),
	)

	// Call IaaS API
	resp, err := i.config.IaaSClient.AllocateIPs(ctx, req)
	if err != nil {
		logger.Error("IaaS allocate API failed",
			zap.String("podUID", string(pod.UID)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("iaas allocate failed: %w", err)
	}

	logger.Debug("IaaS allocate API succeeded",
		zap.Any("response", resp.IaaSIPsAllocationResponse),
	)

	// Merge IaaS response data (MAC, VLAN) into results via the pre-built index
	for _, iaasResult := range resp.IaaSIPsAllocationResponse {
		result, ok := ipToResult[iaasResult.IPAddress]
		if !ok {
			logger.Error("IaaS response contains unknown IP", zap.String("ip", iaasResult.IPAddress))
			return nil, fmt.Errorf("iaas response contains unknown IP %s", iaasResult.IPAddress)
		}
		if iaasResult.MacAddress != "" {
			result.IP.Mac = iaasResult.MacAddress
		}
		if iaasResult.VlanID != 0 {
			result.IP.Vlan = iaasResult.VlanID
		}
	}

	return resp, nil
}

// callIaaSRelease calls the IaaS provider API to release IPs for all IPv4 addresses in the endpoint.
// It releases each IP individually and aggregates any errors.
func (i *ipam) callIaaSRelease(ctx context.Context, nic string, endpoint *v2beta1.SpiderEndpoint) error {
	if i.config.IaaSClient == nil {
		return nil
	}

	logger := logutils.FromContext(ctx).With(
		zap.String("pod", endpoint.Name),
		zap.String("namespace", endpoint.Namespace),
	)

	var pod *corev1.Pod // lazy-loaded on first cache miss
	var errs []error
	for _, detail := range endpoint.Status.Current.IPs {
		// Only handle IPv4 for now
		if detail.IPv4 == nil {
			continue
		}

		ip, subnetCIDR, err := net.ParseCIDR(*detail.IPv4)
		if err != nil {
			logger.Error("failed to parse CIDR", zap.String("ip", *detail.IPv4), zap.Error(err))
			errs = append(errs, fmt.Errorf("failed to parse CIDR %s: %w", *detail.IPv4, err))
			continue
		}
		subnet := subnetCIDR.String()
		ipStr := ip.String()

		// Fast path: try subnet cache first
		var parentNicMac string
		if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(subnet); ok {
			logger.Debug("parentNicMac cache hit by subnet", zap.String("subnet", subnet))
			parentNicMac = cached
		} else {
			// Get parentNicMac: try subnet cache first, then pod-based lookup
			if pod == nil {
				pod, err = i.podManager.GetPodByName(ctx, endpoint.Namespace, endpoint.Name, true)
				if err != nil {
					logger.Error("failed to get pod for IaaS release", zap.Error(err))
					errs = append(errs, fmt.Errorf("failed to get pod %s/%s: %w", endpoint.Namespace, endpoint.Name, err))
					continue
				}
			}
			parentNicMac, err = i.getParentNicMacFromMultus(ctx, pod, nic, subnet)
			if err != nil {
				logger.Warn("Failed to get parentNicMac for IaaS release, proceeding with empty value",
					zap.String("nic", detail.NIC),
					zap.String("subnet", subnet),
					zap.Error(err))
			}
		}

		req := &iaasclient.ReleaseIPRequest{
			PodName:      endpoint.Name,
			PodNamespace: endpoint.Namespace,
			PodUID:       endpoint.Status.Current.UID,
			NodeName:     endpoint.Status.Current.Node,
			IPAddress:    ipStr,
			Subnet:       subnet,
			ParentNicMac: parentNicMac,
		}

		logger.Debug("Calling IaaS release API",
			zap.String("podUID", endpoint.Status.Current.UID),
			zap.String("nodeName", endpoint.Status.Current.Node),
			zap.String("ipAddress", ipStr),
			zap.String("subnet", subnet),
			zap.String("parentNicMac", parentNicMac),
		)

		if err := i.config.IaaSClient.ReleaseIP(ctx, req); err != nil {
			logger.Error("IaaS release API failed",
				zap.String("podUID", endpoint.Status.Current.UID),
				zap.String("ipAddress", ipStr),
				zap.String("subnet", subnet),
				zap.Error(err),
			)
			errs = append(errs, fmt.Errorf("failed to release IP %s: %w", ipStr, err))
			continue
		}

		logger.Info("IaaS release API succeeded", zap.String("ipAddress", ipStr))
	}

	if len(errs) > 0 {
		return fmt.Errorf("iaas release failed for %d IP(s): %v", len(errs), errs)
	}
	return nil
}

// getParentNicMacFromMultus gets the parent NIC MAC address by:
// 1. Checking the in-memory cache first using subnet as key
// 2. If not cached: parsing pod's Multus annotation to find the NAD for the given NIC
// 3. Checking the cache using SpiderMultusConfig namespace/name as key
// 4. Reading SpiderMultusConfig to get the master interface and resolving its MAC via netlink
// 5. Storing the result in cache keyed by both subnet and SpiderMultusConfig namespace/name
func (i *ipam) getParentNicMacFromMultus(ctx context.Context, pod *corev1.Pod, nic string, subnet string) (string, error) {
	if i.config.APIReader == nil {
		return "", fmt.Errorf("APIReader is not configured")
	}

	if subnet != "" {
		if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(subnet); ok {
			return cached, nil
		}
	}

	// Step 1: find the NAD info for this NIC from Multus annotations
	netInfo, err := iaasutils.GetMultusNetworkForNIC(pod, nic, i.config.AgentNamespace, i.config.MultusClusterNetwork)
	if err != nil {
		return "", fmt.Errorf("failed to get multus network for NIC %s: %w", nic, err)
	}

	// Step 2: check IaaS client cache using SpiderMultusConfig namespace/name as key
	cacheKey := netInfo.Namespace + "/" + netInfo.Name
	if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(cacheKey); ok {
		if subnet != "" {
			i.config.IaaSClient.CacheParentNicMac(subnet, cached)
		}
		return cached, nil
	}

	// Step 3: read SpiderMultusConfig (same name/namespace as the NAD)
	smc := &v2beta1.SpiderMultusConfig{}
	if err := i.config.APIReader.Get(ctx, ctrlclient.ObjectKey{Namespace: netInfo.Namespace, Name: netInfo.Name}, smc); err != nil {
		return "", fmt.Errorf("failed to get SpiderMultusConfig %s/%s: %w", netInfo.Namespace, netInfo.Name, err)
	}

	// Step 4: extract master interface name from CNI config
	masterIface, err := getMasterIfaceFromMultusConfig(smc)
	if err != nil {
		return "", fmt.Errorf("failed to get master interface from SpiderMultusConfig %s/%s: %w", netInfo.Namespace, netInfo.Name, err)
	}

	// Step 5: get MAC address of the master interface via netlink (host netns)
	link, err := netlink.LinkByName(masterIface)
	if err != nil {
		return "", fmt.Errorf("failed to get link %s: %w", masterIface, err)
	}

	mac := link.Attrs().HardwareAddr.String()

	// Step 6: store in IaaS client cache for future lookups
	if subnet != "" {
		i.config.IaaSClient.CacheParentNicMac(subnet, mac)
	}
	i.config.IaaSClient.CacheParentNicMac(cacheKey, mac)

	return mac, nil
}

// prewarmParentNicMacCache lists all vlan-type SpiderMultusConfigs at startup
// and resolves their master interface MAC addresses into the cache keyed by
// SpiderMultusConfig namespace/name only.
func (i *ipam) prewarmParentNicMacCache(ctx context.Context) {
	logger := logutils.FromContext(ctx)
	logger.Info("Prewarming parentNicMac cache from SpiderMultusConfigs")

	if i.config.APIReader == nil {
		logger.Warn("APIReader is not configured, skip prewarming parentNicMac cache")
		return
	}

	smcList := &v2beta1.SpiderMultusConfigList{}
	if err := i.config.APIReader.List(ctx, smcList); err != nil {
		logger.Error("Failed to list SpiderMultusConfigs for cache prewarming", zap.Error(err))
		return
	}

	count := 0
	for idx := range smcList.Items {
		smc := &smcList.Items[idx]
		if smc.Spec.CniType == nil || *smc.Spec.CniType != constant.VlanCNI {
			continue
		}

		masterIface, err := getMasterIfaceFromMultusConfig(smc)
		if err != nil {
			continue
		}

		cacheKey := smc.Namespace + "/" + smc.Name
		// Skip if already cached by SMC key
		if _, ok := i.config.IaaSClient.GetCachedParentNicMac(cacheKey); ok {
			continue
		}

		link, err := netlink.LinkByName(masterIface)
		if err != nil {
			logger.Warn("Failed to get link for master interface during prewarm",
				zap.String("masterIface", masterIface),
				zap.String("smc", cacheKey),
				zap.Error(err))
			continue
		}

		mac := link.Attrs().HardwareAddr.String()
		i.config.IaaSClient.CacheParentNicMac(cacheKey, mac)
		count++
		logger.Debug("Prewarmed parentNicMac cache",
			zap.String("smc", cacheKey),
			zap.String("masterIface", masterIface),
			zap.String("mac", mac))
	}

	logger.Info("Finished prewarming parentNicMac cache", zap.Int("count", count))
}

// getMasterIfaceFromMultusConfig extracts the first master interface name from a SpiderMultusConfig
func getMasterIfaceFromMultusConfig(smc *v2beta1.SpiderMultusConfig) (string, error) {
	if smc.Spec.CniType == nil {
		return "", fmt.Errorf("CniType is nil")
	}
	switch *smc.Spec.CniType {
	case "vlan":
		if smc.Spec.VlanConfig != nil {
			if len(smc.Spec.VlanConfig.Master) == 1 {
				return smc.Spec.VlanConfig.Master[0], nil
			}
			if len(smc.Spec.VlanConfig.Master) == 2 && smc.Spec.VlanConfig.Bond != nil {
				return smc.Spec.VlanConfig.Bond.Name, nil
			}
		}
	default:
		return "", fmt.Errorf("unsupported CniType %s, only support 'vlan'", *smc.Spec.CniType)
	}

	return "", fmt.Errorf("no master interface found for CniType %s", *smc.Spec.CniType)
}
