// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
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
