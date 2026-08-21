// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func IsAutoCreatedIPPool(pool *spiderpoolv2beta1.SpiderIPPool) bool {
	// only the auto-created IPPool owns the annotation "ipam.spidernet.io/owner-application"
	poolLabels := pool.GetLabels()
	_, ok := poolLabels[constant.LabelIPPoolOwnerApplicationName]
	return ok
}

// IsIaaSPool reports whether the given SpiderIPPool is managed by the IaaS
// provider: it carries the ipam.spidernet.io/iaas-provider label (prewarm /
// node-scoped pools) OR the ipam.spidernet.io/iaas-global marker (global
// pools) — either marker alone is sufficient. Both labels are kept in sync
// with the annotations of the same key by the IPPool mutating webhook (see
// ippool_mutate.go), so checking them is a cheap map lookup with no need to
// parse status.ipMetaData.
func IsIaaSPool(pool *spiderpoolv2beta1.SpiderIPPool) bool {
	if pool == nil {
		return false
	}
	if _, ok := pool.Labels[constant.LabelIPPoolIaasProvider]; ok {
		return true
	}
	return IsGlobalIaaSPool(pool)
}

// IsPairedIaaSPrimaryPool reports whether the given pool is the primary (v4)
// pool of a paired dual-stack IaaS pool set: it carries the iaas-provider
// label, is an IPv4 pool, and references its sibling v6 pool via the
// pair-pool annotation. Only such pools serve AllocateIPPair; the sibling
// v6 pool never allocates on its own (it is filtered out of the Pod's pool
// candidates by the IPAM selection logic).
func IsPairedIaaSPrimaryPool(pool *spiderpoolv2beta1.SpiderIPPool) bool {
	return IsIaaSPool(pool) &&
		pool.Spec.IPVersion != nil && *pool.Spec.IPVersion == constant.IPv4 &&
		pool.Annotations[constant.AnnoIPPoolPairPool] != ""
}

// IsGlobalIaaSPool reports whether the given pool is an IaaS global pool,
// i.e. carries the explicit ipam.spidernet.io/iaas-global label with the
// value "true". The label is kept in sync with the annotation of the same
// key by the IPPool mutating webhook (see ippool_mutate.go), and the
// validating webhook rejects any value other than "true". The marker is
// independent of the iaas-provider annotation and of spec.nodeName. Its
// metadata (schema v2) carries an explicit empty scope and per-entry node
// placement; sub-ENIs are created on first use and stay bound to their node
// as a sticky cache (global-pool-design.md). Node-level (prewarm) pools
// never carry this marker and are never affected by global-pool logic.
func IsGlobalIaaSPool(pool *spiderpoolv2beta1.SpiderIPPool) bool {
	if pool == nil {
		return false
	}
	return pool.Labels[constant.LabelIPPoolIaasGlobal] == "true"
}

// isDetachingEntry reports whether a global-pool metadata entry is in the
// provider's detaching window: still bound to a node but with the VLAN set
// to the -1 sentinel (the reclaim race guard write). Such entries must be
// skipped in both the cache-hit predicate and the cold-path candidate set —
// the cloud reassigns the VLAN on attach, so the cached one is stale.
func isDetachingEntry(entry *spiderpoolv2beta1.IPMetadataEntry) bool {
	return entry != nil && entry.Node != nil && entry.VLAN != nil && *entry.VLAN == -1
}

// isGlobalCacheHitEntry implements the global-pool per-entry half of the
// cache-hit predicate (FR-020): the sub-ENI is bound to the local node and
// its cached VLAN is trustworthy (not the -1 detaching/unknown sentinel).
// Occupancy (ip ∉ status.allocatedIPs) is enforced by the caller through the
// candidate set.
func isGlobalCacheHitEntry(entry *spiderpoolv2beta1.IPMetadataEntry, localNode string) bool {
	if entry == nil || entry.Node == nil || *entry.Node != localNode || localNode == "" {
		return false
	}
	return entry.VLAN == nil || *entry.VLAN != -1
}

func NewAutoPoolPodAffinity(podTopController types.PodTopController) *metav1.LabelSelector {
	var group, version string

	first, second, hasGroup := strings.Cut(podTopController.APIVersion, "/")
	if hasGroup {
		group = first
		version = second
	} else {
		version = first
	}

	set := labels.Set{
		constant.AutoPoolPodAffinityAppAPIGroup:   group,
		constant.AutoPoolPodAffinityAppAPIVersion: version,
		constant.AutoPoolPodAffinityAppKind:       podTopController.Kind,
		constant.AutoPoolPodAffinityAppNS:         podTopController.Namespace,
		constant.AutoPoolPodAffinityAppName:       podTopController.Name,
	}

	return metav1.SetAsLabelSelector(set)
}

func IsMatchAutoPoolAffinity(podAffinity *metav1.LabelSelector, podTopController types.PodTopController) bool {
	if podAffinity == nil {
		return false
	}

	group, version, _ := strings.Cut(podTopController.APIVersion, "/")
	tmpSet := labels.Set{
		constant.AutoPoolPodAffinityAppAPIGroup:   group,
		constant.AutoPoolPodAffinityAppAPIVersion: version,
		constant.AutoPoolPodAffinityAppKind:       podTopController.Kind,
		constant.AutoPoolPodAffinityAppNS:         podTopController.Namespace,
		constant.AutoPoolPodAffinityAppName:       podTopController.Name,
	}

	for k, v := range tmpSet {
		if podAffinity.MatchLabels[k] != v {
			return false
		}
	}

	return true
}

// ByPoolPriority implements sort.Interface
var _ sort.Interface = &ByPoolPriority{}

type ByPoolPriority []*spiderpoolv2beta1.SpiderIPPool

func (b ByPoolPriority) Len() int { return len(b) }

func (b ByPoolPriority) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

func (b ByPoolPriority) Less(i, j int) bool {
	// Pod Affinity
	if b[i].Spec.PodAffinity != nil && b[j].Spec.PodAffinity == nil {
		return true
	}
	if b[i].Spec.PodAffinity == nil && b[j].Spec.PodAffinity != nil {
		return false
	}

	// Node Affinity
	if len(b[i].Spec.NodeName) != 0 && len(b[j].Spec.NodeName) == 0 {
		return true
	}
	if len(b[i].Spec.NodeName) == 0 && len(b[j].Spec.NodeName) != 0 {
		return false
	}
	if b[i].Spec.NodeAffinity != nil && b[j].Spec.NodeAffinity == nil {
		return true
	}
	if b[i].Spec.NodeAffinity == nil && b[j].Spec.NodeAffinity != nil {
		return false
	}

	// Namespace Affinity
	if len(b[i].Spec.NamespaceName) != 0 && len(b[j].Spec.NamespaceName) == 0 {
		return true
	}
	if len(b[i].Spec.NamespaceName) == 0 && len(b[j].Spec.NamespaceName) != 0 {
		return false
	}
	if b[i].Spec.NamespaceAffinity != nil && b[j].Spec.NamespaceAffinity == nil {
		return true
	}
	if b[i].Spec.NamespaceAffinity == nil && b[j].Spec.NamespaceAffinity != nil {
		return false
	}

	// Multus Name
	if len(b[i].Spec.MultusName) != 0 && len(b[j].Spec.MultusName) == 0 {
		return true
	}
	if len(b[i].Spec.MultusName) == 0 && len(b[j].Spec.MultusName) != 0 {
		return false
	}

	return false
}

// findAllocatedIPFromRecords try to find pod NIC previous allocated IP from the IPPool.Status.AllocatedIPs
// this function serves for the issue: https://github.com/spidernet-io/spiderpool/issues/2517
func findAllocatedIPFromRecords(allocatedRecords spiderpoolv2beta1.PoolIPAllocations, namespacedName, podUID string) (previousIP string, hasFound bool) {
	for tmpIP, poolIPAllocation := range allocatedRecords {
		if poolIPAllocation.NamespacedName == namespacedName &&
			poolIPAllocation.PodUID == podUID {
			return tmpIP, true
		}
	}

	return "", false
}

// HasWildcardInStr checks whether the wildcard '*', '?', '[]' exists in the given string variable
func HasWildcardInStr(str string) bool {
	switch {
	case strings.Contains(str, "?"):
		return true
	case strings.Contains(str, "*"):
		return true
	case strings.Contains(str, "[") && strings.Contains(str, "]"):
		return true
	}

	return false
}

func HasWildcardInSlice(arr []string) bool {
	for _, str := range arr {
		if HasWildcardInStr(str) {
			return true
		}
	}
	return false
}

// IsIPMetadataAddress reports whether the given address is present in the
// ipMetaData.metadata map, either as a key (primary-family address) or as an
// entry's paired ipv6 value. Used to correctly report FromIPMetadata when a
// Pod reuses a previously allocated address (the "already assigned" fast
// path in genRandomIP).
func IsIPMetadataAddress(metadata map[string]spiderpoolv2beta1.IPMetadataEntry, address string) bool {
	if len(metadata) == 0 {
		return false
	}
	if _, ok := metadata[address]; ok {
		return true
	}
	parsed := net.ParseIP(address)
	for key, entry := range metadata {
		if parsed != nil {
			if keyIP := net.ParseIP(key); keyIP != nil && keyIP.Equal(parsed) {
				return true
			}
		}
		if entry.IPv6 != nil && *entry.IPv6 != "" {
			if v6 := net.ParseIP(*entry.IPv6); v6 != nil && parsed != nil && v6.Equal(parsed) {
				return true
			}
			if *entry.IPv6 == address {
				return true
			}
		}
	}
	return false
}

// FindReadyIPMetadata implements the v5 selection model: readiness gating is
// an INTERSECTION of the normal spec.ips-derived candidate set with the
// addresses present in status.ipMetaData.metadata, not a replacement of it.
// candidateIPs MUST already be the pool's normal available-candidate set for
// ipVersion (spec.ips minus excludeIPs/reservedIPs/usedIPs, via the existing
// spiderpoolip.FindAvailableIPs logic) -- this function performs no
// range/exclusion/occupancy computation of its own.
//
// For the pool's family, candidates are matched against the metadata map
// keys. Paired dual-stack pools never select through this function on the
// v6 side: both families of a pair are selected together from the primary
// pool's metadata by FindReadyIPPairMetadata (via AllocateIPPair), so keys
// of the other family are simply skipped here.
//
// It returns the entry whose address for ipVersion is the first candidate in
// ascending address order, along with that address. Malformed map keys and
// entries with no address for ipVersion are skipped without failing the
// whole pool. It returns (nil, "", false) when the intersection is empty --
// including the case of a freshly-created pool with no metadata entries yet.
func FindReadyIPMetadata(metadata map[string]spiderpoolv2beta1.IPMetadataEntry, ipVersion types.IPVersion, candidateIPs []net.IP, localNode string, global bool) (*spiderpoolv2beta1.IPMetadataEntry, string, bool) {
	if len(metadata) == 0 {
		return nil, "", false
	}

	type readyMatch struct {
		entry   spiderpoolv2beta1.IPMetadataEntry
		address string
	}

	var matches []readyMatch
	addMatch := func(entry spiderpoolv2beta1.IPMetadataEntry, addr string) {
		parsed := net.ParseIP(addr)
		if parsed == nil {
			return
		}
		matches = append(matches, readyMatch{entry: entry, address: parsed.String()})
	}

	candidateSet := make(map[string]struct{}, len(candidateIPs))
	for _, ip := range candidateIPs {
		if ip != nil {
			candidateSet[ip.String()] = struct{}{}
		}
	}

	for key, entry := range metadata {
		keyIP := net.ParseIP(key)
		if keyIP == nil {
			// Malformed map key: skip without failing the whole pool.
			continue
		}

		keyIsV4 := keyIP.To4() != nil
		if (ipVersion == constant.IPv4) != keyIsV4 {
			// Key belongs to the other IP family: not selectable here.
			continue
		}
		if global && !isGlobalCacheHitEntry(&entry, localNode) {
			// Global pool cache-hit predicate (FR-020): only entries bound
			// to the local node with a trustworthy VLAN are zero-RPC hits.
			continue
		}
		if _, ok := candidateSet[keyIP.String()]; ok {
			addMatch(entry, keyIP.String())
		}
	}

	if len(matches) == 0 {
		return nil, "", false
	}

	sort.Slice(matches, func(i, j int) bool {
		ipI := net.ParseIP(matches[i].address)
		ipJ := net.ParseIP(matches[j].address)
		if ipI == nil || ipJ == nil {
			return matches[i].address < matches[j].address
		}
		return bytes.Compare(ipI.To16(), ipJ.To16()) < 0
	})

	return &matches[0].entry, matches[0].address, true
}

// FindReadyIPPairMetadata selects the metadata entry backing one atomic
// IPv4/IPv6 pair allocation from a paired dual-stack IaaS pool set. An entry
// is selectable only when BOTH sides are currently available: its IPv4 key
// must be in v4CandidateIPs (the primary pool's normal available-candidate
// set) and its paired ipv6 value must be in v6AvailableIPs (the sibling
// pool's own available set: spec.ips minus excludeIPs/reserved/allocated).
// Among selectable entries, the one with the lowest IPv4 key wins, matching
// the existing ascending-order selection convention. Entries with malformed
// keys or missing/malformed ipv6 values are skipped without failing the
// whole pool. It returns (nil, nil, nil, false) when no entry is fully
// available on both sides.
func FindReadyIPPairMetadata(metadata map[string]spiderpoolv2beta1.IPMetadataEntry, v4CandidateIPs, v6AvailableIPs []net.IP, localNode string, global bool) (*spiderpoolv2beta1.IPMetadataEntry, net.IP, net.IP, bool) {
	if len(metadata) == 0 {
		return nil, nil, nil, false
	}

	newIPSet := func(ips []net.IP) map[string]struct{} {
		set := make(map[string]struct{}, len(ips))
		for _, ip := range ips {
			if ip != nil {
				set[ip.String()] = struct{}{}
			}
		}
		return set
	}
	v4Set := newIPSet(v4CandidateIPs)
	v6Set := newIPSet(v6AvailableIPs)

	type pairMatch struct {
		entry spiderpoolv2beta1.IPMetadataEntry
		v4    net.IP
		v6    net.IP
	}

	var matches []pairMatch
	for key, entry := range metadata {
		keyIP := net.ParseIP(key)
		if keyIP == nil || keyIP.To4() == nil {
			// Malformed or non-v4 map key: skip without failing the pool.
			continue
		}
		if global && !isGlobalCacheHitEntry(&entry, localNode) {
			// Global pool cache-hit predicate (FR-020): both families of a
			// pair are a hit only when the entry's sub-ENI is bound to the
			// local node with a trustworthy VLAN.
			continue
		}
		if _, ok := v4Set[keyIP.String()]; !ok {
			continue
		}
		if entry.IPv6 == nil || *entry.IPv6 == "" {
			continue
		}
		v6IP := net.ParseIP(*entry.IPv6)
		if v6IP == nil || v6IP.To4() != nil {
			continue
		}
		if _, ok := v6Set[v6IP.String()]; !ok {
			continue
		}
		matches = append(matches, pairMatch{entry: entry, v4: keyIP, v6: v6IP})
	}

	if len(matches) == 0 {
		return nil, nil, nil, false
	}

	sort.Slice(matches, func(i, j int) bool {
		return bytes.Compare(matches[i].v4.To16(), matches[j].v4.To16()) < 0
	})

	return &matches[0].entry, matches[0].v4, matches[0].v6, true
}

// FindGlobalColdPathIP selects the cold-path candidate for a global pool
// when no local cache hit exists (FR-021). candidateIPs MUST already be the
// pool's normal available set (spec.ips minus excludeIPs/reserved/used).
// Ordering minimizes cloud calls per global-pool-design.md §4.2:
//  1. unbound addresses — no metadata entry, or an entry without node
//     (sub-ENI absent or detached): one cloud call (create/attach);
//  2. addresses idle on another node: two cloud calls (detach+attach steal).
//
// Detaching entries (node present, vlan == -1: the provider's reclaim race
// guard) are skipped entirely. Within each tier the lowest address wins,
// matching the existing ascending-order convention.
func FindGlobalColdPathIP(metadata map[string]spiderpoolv2beta1.IPMetadataEntry, candidateIPs []net.IP) (net.IP, bool) {
	sorted := make([]net.IP, 0, len(candidateIPs))
	for _, ip := range candidateIPs {
		if ip != nil {
			sorted = append(sorted, ip)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].To16(), sorted[j].To16()) < 0
	})

	var steal net.IP
	for _, ip := range sorted {
		entry, ok := metadata[ip.String()]
		if !ok || entry.Node == nil {
			// Tier 1: unbound — includes entries that kept vlan == -1
			// after a completed detach; the provider Allocate RPC response
			// supplies the authoritative VLAN.
			return ip, true
		}
		if isDetachingEntry(&entry) {
			continue
		}
		if steal == nil {
			// Tier 2: idle on another node (or a local entry that raced
			// past the hit path); keep the lowest as the steal fallback.
			steal = ip
		}
	}
	if steal != nil {
		return steal, true
	}
	return nil, false
}

// MetadataReferencedIPv6Set returns the set of canonical IPv6 addresses
// referenced by any metadata entry's ipv6 value. In global mode a cached
// sub-ENI locks its dynamically-paired v6 for the sub-ENI's lifetime even
// while no Pod uses it (FR-024), so these addresses must be excluded from
// cold-path v6 candidate selection.
func MetadataReferencedIPv6Set(metadata map[string]spiderpoolv2beta1.IPMetadataEntry) map[string]struct{} {
	set := make(map[string]struct{}, len(metadata))
	for _, entry := range metadata {
		if entry.IPv6 == nil || *entry.IPv6 == "" {
			continue
		}
		if v6 := net.ParseIP(*entry.IPv6); v6 != nil {
			set[v6.String()] = struct{}{}
		}
	}
	return set
}

// FindGlobalColdPathIPv6 selects the sticky-pair v6 side for a global-pool
// cold-path allocation: the lowest address in v6AvailableIPs that is not
// already referenced by an existing metadata entry's ipv6 (FR-024). The
// provider creates one dual-stack sub-ENI from the {v4, v6} pair and the
// entry's ipv6 becomes sticky for the sub-ENI's lifetime.
func FindGlobalColdPathIPv6(referencedV6 map[string]struct{}, v6AvailableIPs []net.IP) (net.IP, bool) {
	sorted := make([]net.IP, 0, len(v6AvailableIPs))
	for _, ip := range v6AvailableIPs {
		if ip != nil {
			sorted = append(sorted, ip)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].To16(), sorted[j].To16()) < 0
	})
	for _, ip := range sorted {
		if _, ok := referencedV6[ip.String()]; ok {
			continue
		}
		return ip, true
	}
	return nil, false
}

// ParentNicFromPool extracts the pool-level parent NIC name from the
// provider-written status.ipMetaData.metadata envelope (the reserved
// "parentNic" key). The parent NIC is written together with the metadata
// skeleton right after pool creation and stays stable for the pool's
// lifetime, so no generation gating is applied. An empty string with a nil
// error means the provider has not written the skeleton yet.
func ParentNicFromPool(pool *spiderpoolv2beta1.SpiderIPPool) (string, error) {
	if pool == nil || pool.Status.IPMetaData == nil || pool.Status.IPMetaData.Metadata == nil {
		return "", nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(*pool.Status.IPMetaData.Metadata), &top); err != nil {
		return "", fmt.Errorf("pool %s metadata is malformed: %w", pool.Name, err)
	}
	rawNic, ok := top[constant.IPPoolMetadataParentNicKey]
	if !ok {
		return "", nil
	}
	var parentNic string
	if err := json.Unmarshal(rawNic, &parentNic); err != nil {
		return "", fmt.Errorf("pool %s metadata parentNic is malformed: %w", pool.Name, err)
	}
	return parentNic, nil
}
