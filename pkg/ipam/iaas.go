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
	if i.config.APIReader == nil {
		return nil, fmt.Errorf("APIReader is not configured")
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
		v4Pool       string
		v6Pool       string
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
		if result.IP.IPPool == "" {
			logger.Error("Allocation result carries no pool name", zap.String("nic", *result.IP.Nic))
			return nil, fmt.Errorf("allocation result for NIC %s carries no pool name", *result.IP.Nic)
		}
		ipPool := &v2beta1.SpiderIPPool{}
		if err := i.config.APIReader.Get(ctx, ctrlclient.ObjectKey{Name: result.IP.IPPool}, ipPool); err != nil {
			logger.Error("Failed to get IPPool for IaaS eligibility check", zap.String("pool", result.IP.IPPool), zap.Error(err))
			return nil, fmt.Errorf("failed to get IPPool %q: %w", result.IP.IPPool, err)
		}
		// IaaS eligibility is decided by the pool marker alone: only
		// addresses allocated from an IaaS-managed pool (iaas-provider or
		// iaas-global) involve the provider. The parent NIC comes from the
		// pool's own provider-written metadata ("parentNic"), so no
		// SpiderMultusConfig or Multus annotation is consulted. A missing
		// or unresolvable parent NIC is a hard error (fail-closed): the
		// provider writes the metadata skeleton right after pool creation,
		// so this only fails during that short window (CNI retries cover
		// it) or on real misconfiguration.
		if !ippoolmanager.IsIaaSPool(ipPool) {
			logger.Debug("Skipping IaaS allocation for non-IaaS pool", zap.String("pool", result.IP.IPPool), zap.String("nic", *result.IP.Nic))
			continue
		}
		parentMac, err := i.resolveParentNicMacFromPool(ipPool, subnet)
		if err != nil {
			logger.Error("Failed to resolve parent NIC MAC for IaaS pool", zap.String("pool", result.IP.IPPool), zap.String("nic", *result.IP.Nic), zap.Error(err))
			return nil, fmt.Errorf("IP allocated from IaaS pool %q but its parent NIC cannot be resolved: %w", result.IP.IPPool, err)
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
			group.v4Pool = result.IP.IPPool
		} else {
			group.v6Result = result
			group.v6IP = ip.String()
			group.v6Subnet = subnet
			group.v6Pool = result.IP.IPPool
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
			IPv4PoolName: group.v4Pool,
			IPv6PoolName: group.v6Pool,
		})
	}

	if len(req.SubEniRequests) == 0 {
		logger.Debug("No IaaS-pool IPs require IaaS allocation")
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
// (marked iaas-provider or iaas-global) are skipped: they stay reserved on the cloud side and are
// only unclaimed internally (status.allocatedIPs) so they can be handed out again quickly.
// IPs from non-IaaS pools are skipped too, as they never involved the provider.
func (i *ipam) callIaaSRelease(ctx context.Context, endpoint *v2beta1.SpiderEndpoint) error {
	if i.config.IaaSClient == nil {
		return nil
	}

	logger := logutils.FromContext(ctx).With(
		zap.String("pod", endpoint.Name),
		zap.String("namespace", endpoint.Namespace),
	)

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

		// IaaS involvement is decided by the pool marker. IPs sourced from an
		// IaaS-managed pool (iaas-provider prewarm pools and iaas-global
		// pools) are owned by the external IaaS provider controller and must
		// remain reserved on the cloud side across Pod lifecycles for fast
		// reuse: spiderpool only releases its own internal claim
		// (status.allocatedIPs) and must NOT call the IaaS release API. IPs
		// from a non-IaaS pool never involved the provider at allocation
		// time, so there is nothing to release either. Only when the pool
		// can no longer be inspected (lookup failure, e.g. already deleted)
		// does spiderpool fall through and attempt a best-effort release.
		if poolName != nil {
			ipPool, err := i.ipPoolManager.GetIPPoolByName(ctx, *poolName, constant.UseCache)
			if err != nil {
				logger.Warn("Failed to get IPPool for IaaS-pool release check, proceeding with IaaS release",
					zap.String("pool", *poolName), zap.String("ip", ipStr), zap.Error(err))
			} else if ippoolmanager.IsIaaSPool(ipPool) {
				logger.Debug("Skipping IaaS release for IaaS-managed pool, keeping cloud-side reservation",
					zap.String("pool", *poolName), zap.String("ip", ipStr))
				continue
			} else {
				logger.Debug("Skipping IaaS release for non-IaaS pool",
					zap.String("pool", *poolName), zap.String("ip", ipStr))
				continue
			}
		}

		// Best-effort release (pool no longer inspectable): the parent NIC
		// MAC comes from the subnet-keyed cache when warm; otherwise the
		// release request is sent without it (the field is optional).
		var parentNicMac string
		if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(subnet); ok {
			logger.Debug("parentNicMac cache hit by subnet", zap.String("subnet", subnet))
			parentNicMac = cached
		} else {
			logger.Debug("parentNicMac unavailable for best-effort release, sending release without it",
				zap.String("nic", detail.NIC), zap.String("subnet", subnet))
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
		if poolName != nil {
			req.PoolName = *poolName
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

// resolveParentNicMacFromPool resolves the parent NIC MAC address for an
// IaaS-managed pool from the pool's own provider-written metadata: the
// pool-level "parentNic" interface name is looked up on the local node via
// netlink. No SpiderMultusConfig or Multus annotation is involved. The MAC
// is cached by interface name (and by subnet, for the release fast path),
// so the hot path costs a single in-memory lookup. An empty parentNic
// (provider has not written the metadata skeleton yet) or a netlink failure
// is returned as an error so the caller can fail closed; CNI retries cover
// the short post-creation window.
func (i *ipam) resolveParentNicMacFromPool(pool *v2beta1.SpiderIPPool, subnet string) (string, error) {
	parentNic, err := ippoolmanager.ParentNicFromPool(pool)
	if err != nil {
		return "", err
	}
	if parentNic == "" {
		return "", fmt.Errorf("pool %s metadata carries no parentNic (provider skeleton not written yet?)", pool.Name)
	}

	var mac string
	if cached, ok := i.config.IaaSClient.GetCachedParentNicMac(parentNic); ok {
		mac = cached
	} else {
		link, err := netlink.LinkByName(parentNic)
		if err != nil {
			return "", fmt.Errorf("failed to get link %s: %w", parentNic, err)
		}
		mac = link.Attrs().HardwareAddr.String()
		i.config.IaaSClient.CacheParentNicMac(parentNic, mac)
	}
	if subnet != "" {
		i.config.IaaSClient.CacheParentNicMac(subnet, mac)
	}
	return mac, nil
}
