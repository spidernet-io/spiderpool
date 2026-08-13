// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"bytes"
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

// IsIaaSPool reports whether the given SpiderIPPool is an IaaS-managed
// (prewarm) pool, i.e. carries the ipam.spidernet.io/iaas-provider label.
// The label is kept in sync with the annotation of the same key by the
// IPPool mutating webhook (see ippool_mutate.go), so checking the label is
// a cheap map lookup with no need to parse status.ipMetaData.
func IsIaaSPool(pool *spiderpoolv2beta1.SpiderIPPool) bool {
	if pool == nil {
		return false
	}
	_, ok := pool.Labels[constant.LabelIPPoolIaasProvider]
	return ok
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
func FindReadyIPMetadata(metadata map[string]spiderpoolv2beta1.IPMetadataEntry, ipVersion types.IPVersion, candidateIPs []net.IP) (*spiderpoolv2beta1.IPMetadataEntry, string, bool) {
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
func FindReadyIPPairMetadata(metadata map[string]spiderpoolv2beta1.IPMetadataEntry, v4CandidateIPs, v6AvailableIPs []net.IP) (*spiderpoolv2beta1.IPMetadataEntry, net.IP, net.IP, bool) {
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
