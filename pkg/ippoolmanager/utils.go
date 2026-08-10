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
// (prewarm) pool, i.e. carries the ipam.spidernet.io/iaas-pool label. The
// label is kept in sync with the ipam.spidernet.io/iaas-pool annotation by
// the IPPool mutating webhook (see ippool_mutate.go), so checking the label
// is a cheap map lookup with no need to parse status.iaasIPs.
func IsIaaSPool(pool *spiderpoolv2beta1.SpiderIPPool) bool {
	if pool == nil {
		return false
	}
	_, ok := pool.Labels[constant.LabelIPPoolIaas]
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

// IsValidIaasIPAllocation reports whether an IaasReadyIPAllocation ledger
// entry is well-formed enough to be considered for allocation: it MUST have
// at least one of IPv4/IPv6 populated (data-model.md validation rules).
// Malformed entries are skipped rather than failing the whole pool.
func IsValidIaasIPAllocation(entry spiderpoolv2beta1.IaasReadyIPAllocation) bool {
	return (entry.IPv4 != nil && *entry.IPv4 != "") || (entry.IPv6 != nil && *entry.IPv6 != "")
}

// IsIaasIPAllocationClaimed reports whether the ledger entry's address(es)
// already appear as keys in the pool's parsed status.allocatedIPs. Occupancy
// is derived, not stored (per clarification Q4) — this is the single source
// of truth for whether an entry has already been consumed. Note: when
// selection is driven through FindReadyIaasIPAllocation, this check is
// already implied by the candidate-set intersection (candidateIPs is built
// excluding already-allocated addresses); this helper remains available for
// callers that need to check claimed status independently of that flow.
func IsIaasIPAllocationClaimed(entry spiderpoolv2beta1.IaasReadyIPAllocation, allocatedRecords spiderpoolv2beta1.PoolIPAllocations) bool {
	if entry.IPv4 != nil && *entry.IPv4 != "" {
		if _, ok := allocatedRecords[*entry.IPv4]; ok {
			return true
		}
	}
	if entry.IPv6 != nil && *entry.IPv6 != "" {
		if _, ok := allocatedRecords[*entry.IPv6]; ok {
			return true
		}
	}
	return false
}

// PrimaryAddress returns the address used to order IaasReadyIPAllocation
// entries deterministically: IPv4 when present, otherwise IPv6 (per
// clarification Q5, which reuses the existing ascending-address pool
// allocation order).
func PrimaryAddress(entry spiderpoolv2beta1.IaasReadyIPAllocation) string {
	if entry.IPv4 != nil && *entry.IPv4 != "" {
		return *entry.IPv4
	}
	if entry.IPv6 != nil && *entry.IPv6 != "" {
		return *entry.IPv6
	}
	return ""
}

// IaasIPAllocationAddressForVersion returns the ledger entry's address for
// the given IP version (4 or 6), or "" if the entry has no address for that
// family (e.g. a single-family entry, or a mismatched request).
func IaasIPAllocationAddressForVersion(entry spiderpoolv2beta1.IaasReadyIPAllocation, ipVersion types.IPVersion) string {
	switch ipVersion {
	case constant.IPv4:
		if entry.IPv4 != nil {
			return *entry.IPv4
		}
	case constant.IPv6:
		if entry.IPv6 != nil {
			return *entry.IPv6
		}
	}
	return ""
}

// IsIaasLedgerAddress reports whether the given address is present as an
// IPv4 or IPv6 address of any entry in the given IaasReadyIPs ledger. Used
// to correctly report FromIaasLedger when a Pod reuses a previously
// allocated address (the "already assigned" fast path in genRandomIP).
func IsIaasLedgerAddress(iaasReadyIPs []spiderpoolv2beta1.IaasReadyIPAllocation, address string) bool {
	for _, entry := range iaasReadyIPs {
		if (entry.IPv4 != nil && *entry.IPv4 == address) || (entry.IPv6 != nil && *entry.IPv6 == address) {
			return true
		}
	}
	return false
}

// FindReadyIaasIPAllocation implements the revised (Phase 5.1) selection
// model: readiness gating is an INTERSECTION of the normal spec.ips-derived
// candidate set with status.iaasReadyIPs, not a replacement of it. candidateIPs
// MUST already be the pool's normal available-candidate set for its own IP
// family (spec.ips minus excludeIPs/reservedIPs/usedIPs, via the existing
// spiderpoolip.FindAvailableIPs logic) -- this function performs no
// range/exclusion/occupancy computation of its own.
//
// It returns the first entry (ascending address order, per clarification Q5)
// whose address for ipVersion is present in candidateIPs, along with that
// address. Malformed entries (neither IPv4 nor IPv6 set) and entries with no
// address for ipVersion are skipped without failing the whole pool. It
// returns (nil, "", false) when no qualifying entry exists -- including the
// case of a freshly-created pool with an empty ledger.
func FindReadyIaasIPAllocation(iaasReadyIPs []spiderpoolv2beta1.IaasReadyIPAllocation, ipVersion types.IPVersion, candidateIPs []net.IP) (*spiderpoolv2beta1.IaasReadyIPAllocation, string, bool) {
	candidateSet := make(map[string]struct{}, len(candidateIPs))
	for _, ip := range candidateIPs {
		if ip != nil {
			candidateSet[ip.String()] = struct{}{}
		}
	}

	type readyMatch struct {
		entry   spiderpoolv2beta1.IaasReadyIPAllocation
		address string
	}

	var matches []readyMatch
	for _, entry := range iaasReadyIPs {
		if !IsValidIaasIPAllocation(entry) {
			continue
		}

		addr := IaasIPAllocationAddressForVersion(entry, ipVersion)
		if addr == "" {
			continue
		}

		parsed := net.ParseIP(addr)
		if parsed == nil {
			continue
		}

		if _, ok := candidateSet[parsed.String()]; !ok {
			continue
		}

		matches = append(matches, readyMatch{entry: entry, address: parsed.String()})
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
