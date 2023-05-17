// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by informer-gen. DO NOT EDIT.

package v2beta1

import (
	internalinterfaces "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// SpiderCoordinators returns a SpiderCoordinatorInformer.
	SpiderCoordinators() SpiderCoordinatorInformer
	// SpiderIPPools returns a SpiderIPPoolInformer.
	SpiderIPPools() SpiderIPPoolInformer
	// SpiderSubnets returns a SpiderSubnetInformer.
	SpiderSubnets() SpiderSubnetInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// SpiderCoordinators returns a SpiderCoordinatorInformer.
func (v *version) SpiderCoordinators() SpiderCoordinatorInformer {
	return &spiderCoordinatorInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// SpiderIPPools returns a SpiderIPPoolInformer.
func (v *version) SpiderIPPools() SpiderIPPoolInformer {
	return &spiderIPPoolInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// SpiderSubnets returns a SpiderSubnetInformer.
func (v *version) SpiderSubnets() SpiderSubnetInformer {
	return &spiderSubnetInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
