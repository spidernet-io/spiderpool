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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	iaasclient "github.com/spidernet-io/spiderpool/pkg/iaas/client"
	iaasutils "github.com/spidernet-io/spiderpool/pkg/iaas/utils"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

// callIaaSAllocate calls the IaaS provider API to create sub-ENIs. Each
// request item describes one sub-network-interface and carries the addresses
// actually allocated for that NIC: IPv4-only, IPv6-only, or a dual-stack
// pair provisioned atomically. Spiderpool passes the allocated families
// through as-is; any family requirement is enforced by the provider itself.
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

	// Group provider-eligible results by NIC so the v4 and v6 allocations of
	// one Pod interface land in a single sub-ENI request item.
	type subEniGroup struct {
		parentNicMac string
		v4Result     *spiderpooltypes.AllocationResult
		v6Result     *spiderpooltypes.AllocationResult
		v4IP         string
		v6IP         string
		v4Subnet     string
		v6Subnet     string
	}
	groupsByNic := make(map[string]*subEniGroup, len(results))
	nicOrder := make([]string, 0, len(results))

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
		parentMac, eligible, err := i.getProviderParentNicMacFromMultus(ctx, pod, *result.IP.Nic, subnet)
		if err != nil {
			logger.Error("Failed to get parent NIC MAC", zap.String("nic", *result.IP.Nic), zap.Error(err))
			return nil, fmt.Errorf("failed to get parent NIC MAC: %w", err)
		}
		if !eligible {
			logger.Debug("Skipping IaaS allocation for non-provider network", zap.String("nic", *result.IP.Nic))
			continue
		}

		nic := *result.IP.Nic
		group, ok := groupsByNic[nic]
		if !ok {
			group = &subEniGroup{parentNicMac: parentMac}
			groupsByNic[nic] = group
			nicOrder = append(nicOrder, nic)
		}
		if ip.To4() != nil {
			group.v4Result = result
			group.v4IP = ip.String()
			group.v4Subnet = subnet
		} else {
			group.v6Result = result
			group.v6IP = ip.String()
			group.v6Subnet = subnet
		}
	}

	// Build one request item per NIC carrying whatever address families were
	// allocated (v4-only, v6-only, or both). The subnet identifies the cloud
	// subnet: prefer the IPv4 CIDR, which for a dual-stack sub-ENI is shared
	// by both families.
	for _, nic := range nicOrder {
		group := groupsByNic[nic]
		subnet := group.v4Subnet
		if subnet == "" {
			subnet = group.v6Subnet
		}
		req.SubEniRequests = append(req.SubEniRequests, iaasclient.SubEniRequest{
			ParentNicMac: group.parentNicMac,
			Subnet:       subnet,
			IPv4Address:  group.v4IP,
			IPv6Address:  group.v6IP,
		})
	}

	if len(req.SubEniRequests) == 0 {
		logger.Debug("No provider VLAN IPs require IaaS allocation")
		return nil, nil
	}

	logger.Debug(
		"Calling IaaS allocate API",
		zap.String("podUID", string(pod.UID)),
		zap.String("nodeName", pod.Spec.NodeName),
		zap.Any("request", req.SubEniRequests),
	)

	// Call IaaS API
	resp, err := i.config.IaaSClient.AllocateIPs(ctx, req)
	if err != nil {
		logger.Error(
			"IaaS allocate API failed",
			zap.String("podUID", string(pod.UID)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("iaas allocate failed: %w", err)
	}

	logger.Debug(
		"IaaS allocate API succeeded",
		zap.Any("response", resp.SubEniResponses),
	)

	// Merge IaaS response data (MAC, VLAN) back into every family result of
	// each sub-ENI: a dual-stack pair shares one MAC address and VLAN ID.
	// Sub-ENIs are matched by their IPv4 address, or by IPv6 for a v6-only item.
	groupsByIP := make(map[string]*subEniGroup, len(groupsByNic))
	for _, group := range groupsByNic {
		if group.v4IP != "" {
			groupsByIP[group.v4IP] = group
		} else {
			groupsByIP[group.v6IP] = group
		}
	}
	for _, subEni := range resp.SubEniResponses {
		key := subEni.IPv4Address
		if key == "" {
			key = subEni.IPv6Address
		}
		group, ok := groupsByIP[key]
		if !ok {
			logger.Error("IaaS response contains unknown sub-ENI", zap.String("ipv4Address", subEni.IPv4Address), zap.String("ipv6Address", subEni.IPv6Address))
			return nil, fmt.Errorf("iaas response contains unknown sub-ENI with address %s", key)
		}
		if group.v6IP != "" && subEni.IPv6Address != "" && group.v6IP != subEni.IPv6Address {
			logger.Error("IaaS response IPv6 address mismatch",
				zap.String("ipv4Address", subEni.IPv4Address),
				zap.String("requestedIPv6", group.v6IP),
				zap.String("respondedIPv6", subEni.IPv6Address),
			)
			return nil, fmt.Errorf("iaas response IPv6 address %s does not match requested %s for sub-ENI %s", subEni.IPv6Address, group.v6IP, subEni.IPv4Address)
		}
		for _, result := range []*spiderpooltypes.AllocationResult{group.v4Result, group.v6Result} {
			if result == nil {
				continue
			}
			if subEni.MacAddress != "" {
				result.IP.Mac = subEni.MacAddress
			}
			if subEni.VlanID != 0 {
				result.IP.Vlan = subEni.VlanID
			}
		}
	}

	return resp, nil
}

// callIaaSRelease calls the IaaS provider API to release the sub-ENI of each
// endpoint IP detail. Releasing either address of a dual-stack sub-ENI deletes
// the whole sub-ENI on the cloud side, so one release call per detail (by its
// IPv4 address, or IPv6 for a v6-only allocation) tears down all its addresses.
// It releases each IP individually and aggregates any errors. IPs whose pool is IaaS-managed
// (labeled ipam.spidernet.io/iaas-provider) are skipped: they stay reserved on the cloud side and are
// only unclaimed internally (status.allocatedIPs) so they can be handed out again quickly.
func (i *ipam) callIaaSRelease(ctx context.Context, endpoint *v2beta1.SpiderEndpoint) error {
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
		// Release the sub-ENI by its IPv4 address, or by IPv6 for a
		// v6-only allocation. Either address tears down the whole sub-ENI.
		address := detail.IPv4
		poolName := detail.IPv4Pool
		if address == nil {
			address = detail.IPv6
			poolName = detail.IPv6Pool
		}
		if address == nil {
			continue
		}

		ip, subnetCIDR, err := net.ParseCIDR(*address)
		if err != nil {
			logger.Error("failed to parse CIDR", zap.String("ip", *address), zap.Error(err))
			errs = append(errs, fmt.Errorf("failed to parse CIDR %s: %w", *address, err))
			continue
		}
		subnet := subnetCIDR.String()
		ipStr := ip.String()

		// IPs sourced from an IaaS-managed prewarm pool (labeled
		// ipam.spidernet.io/iaas-provider) are owned by the external IaaS
		// provider controller and must remain reserved on the cloud side
		// across Pod lifecycles for fast reuse. Spiderpool only releases
		// its own internal claim (status.allocatedIPs) for these addresses
		// and must NOT call the IaaS release API, otherwise the
		// cloud-side reservation would be torn down and the prewarm
		// benefit lost.
		if poolName != nil {
			ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, *poolName, constant.UseCache)
			if err != nil {
				logger.Warn("Failed to get IPPool for IaaS-pool release check, proceeding with IaaS release",
					zap.String("pool", *poolName), zap.String("ip", ipStr), zap.Error(err))
			} else if ippoolmanager.IsIaaSPool(ipPool) {
				logger.Debug("Skipping IaaS release for IaaS-managed pool, keeping cloud-side reservation",
					zap.String("pool", *poolName), zap.String("ip", ipStr))
				continue
			}
		}

		var parentNicMac string
		if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(subnet); ok {
			logger.Debug("parentNicMac cache hit by subnet", zap.String("subnet", subnet))
			parentNicMac = cached
		} else {
			if pod == nil {
				pod, err = i.podManager.GetPodByName(ctx, endpoint.Namespace, endpoint.Name, true)
				if err != nil {
					logger.Error("Failed to get pod for IaaS release eligibility check",
						zap.String("nic", detail.NIC), zap.String("subnet", subnet), zap.Error(err))
					errs = append(errs, fmt.Errorf("failed to get pod %s/%s: %w", endpoint.Namespace, endpoint.Name, err))
					continue
				}
				if pod == nil {
					logger.Warn("Pod is unavailable for IaaS release eligibility check, skipping non-cached IP",
						zap.String("nic", detail.NIC), zap.String("subnet", subnet))
					continue
				}
			}
			var eligible bool
			parentNicMac, eligible, err = i.getProviderParentNicMacFromMultus(ctx, pod, detail.NIC, subnet)
			if err != nil {
				logger.Warn("Failed to determine IaaS release eligibility, skipping IP",
					zap.String("nic", detail.NIC),
					zap.String("subnet", subnet),
					zap.Error(err))
				continue
			}
			if !eligible {
				logger.Debug("Skipping IaaS release for non-provider network",
					zap.String("nic", detail.NIC),
					zap.String("subnet", subnet))
				continue
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

		logger.Debug(
			"Calling IaaS release API",
			zap.String("podUID", endpoint.Status.Current.UID),
			zap.String("nodeName", endpoint.Status.Current.Node),
			zap.String("ipAddress", ipStr),
			zap.String("subnet", subnet),
			zap.String("parentNicMac", parentNicMac),
		)

		if err := i.config.IaaSClient.ReleaseIP(ctx, req); err != nil {
			logger.Error(
				"IaaS release API failed",
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

// getProviderParentNicMacFromMultus determines whether the NIC uses a
// provider-managed VLAN SpiderMultusConfig and, if so, resolves the parent NIC
// MAC. Eligibility is checked before consulting the subnet cache so a
// non-provider network can never be selected by a shared subnet cache entry.
func (i *ipam) getProviderParentNicMacFromMultus(ctx context.Context, pod *corev1.Pod, nic string, subnet string) (string, bool, error) {
	if i.config.APIReader == nil {
		return "", false, fmt.Errorf("APIReader is not configured")
	}

	// Step 1: find the NAD info for this NIC from Multus annotations
	netInfo, err := iaasutils.GetMultusNetworkForNIC(pod, nic, i.config.AgentNamespace, i.config.MultusClusterNetwork)
	if err != nil {
		return "", false, fmt.Errorf("failed to get multus network for NIC %s: %w", nic, err)
	}

	// Step 2: read SpiderMultusConfig and determine whether this NIC belongs to
	// a provider-managed VLAN network before consulting any MAC cache.
	smc := &v2beta1.SpiderMultusConfig{}
	if err := i.config.APIReader.Get(ctx, ctrlclient.ObjectKey{Namespace: netInfo.Namespace, Name: netInfo.Name}, smc); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to get SpiderMultusConfig %s/%s: %w", netInfo.Namespace, netInfo.Name, err)
	}
	if !isProviderVLANSpiderMultusConfig(smc) {
		return "", false, nil
	}

	if subnet != "" {
		if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(subnet); ok {
			return cached, true, nil
		}
	}

	// Step 3: check IaaS client cache using SpiderMultusConfig namespace/name as key
	cacheKey := netInfo.Namespace + "/" + netInfo.Name
	if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(cacheKey); ok {
		if subnet != "" {
			i.config.IaaSClient.CacheParentNicMac(subnet, cached)
		}
		return cached, true, nil
	}

	// Step 4: extract master interface name from CNI config
	masterIface, err := getMasterIfaceFromMultusConfig(smc)
	if err != nil {
		return "", false, fmt.Errorf("failed to get master interface from SpiderMultusConfig %s/%s: %w", netInfo.Namespace, netInfo.Name, err)
	}

	// Step 5: get MAC address of the master interface via netlink (host netns)
	link, err := netlink.LinkByName(masterIface)
	if err != nil {
		return "", false, fmt.Errorf("failed to get link %s: %w", masterIface, err)
	}

	mac := link.Attrs().HardwareAddr.String()

	// Step 6: store in IaaS client cache for future lookups
	if subnet != "" {
		i.config.IaaSClient.CacheParentNicMac(subnet, mac)
	}
	i.config.IaaSClient.CacheParentNicMac(cacheKey, mac)

	return mac, true, nil
}

func isProviderVLANSpiderMultusConfig(smc *v2beta1.SpiderMultusConfig) bool {
	return smc != nil &&
		smc.Spec.CniType != nil &&
		*smc.Spec.CniType == constant.VlanCNI &&
		smc.Spec.VlanConfig != nil &&
		smc.Spec.VlanConfig.VlanMode != nil &&
		*smc.Spec.VlanConfig.VlanMode == constant.VlanModeAuto
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
		if !isProviderVLANSpiderMultusConfig(smc) {
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
