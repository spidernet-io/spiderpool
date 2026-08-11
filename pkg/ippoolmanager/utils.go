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

// PoolIPMetadata returns the pool's provider-written ipMetaData.metadata
// map, nil-safe. An empty/nil map simply yields an empty intersection.
func PoolIPMetadata(pool *spiderpoolv2beta1.SpiderIPPool) map[string]spiderpoolv2beta1.IPMetadataEntry {
	if pool == nil || pool.Status.IPMetaData == nil {
		return nil
	}
	return pool.Status.IPMetaData.Metadata
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
// For the pool's primary family (the map-key family), candidates are matched
// against the metadata map keys; when ipVersion is IPv6 and the map is keyed
// by IPv4 (a paired pool whose sibling borrows the primary pool's metadata),
// candidates are matched against each entry's paired ipv6 value instead.
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
		switch ipVersion {
		case constant.IPv4:
			if !keyIsV4 {
				continue
			}
			if _, ok := candidateSet[keyIP.String()]; ok {
				addMatch(entry, keyIP.String())
			}
		case constant.IPv6:
			if !keyIsV4 {
				// Pure-v6 single-stack pool: the map key IS the v6 address.
				if _, ok := candidateSet[keyIP.String()]; ok {
					addMatch(entry, keyIP.String())
				}
				continue
			}
			// v4-keyed map consumed from the v6 side (borrowed from the
			// paired primary pool): match against the entry's ipv6 value.
			if entry.IPv6 == nil || *entry.IPv6 == "" {
				continue
			}
			v6 := net.ParseIP(*entry.IPv6)
			if v6 == nil {
				continue
			}
			if _, ok := candidateSet[v6.String()]; ok {
				addMatch(entry, v6.String())
			}
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
