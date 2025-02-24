// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

// Code generated by lister-gen. DO NOT EDIT.

package v2alpha1

import (
	ciliumiov2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// CiliumBGPNodeConfigOverrideLister helps list CiliumBGPNodeConfigOverrides.
// All objects returned here must be treated as read-only.
type CiliumBGPNodeConfigOverrideLister interface {
	// List lists all CiliumBGPNodeConfigOverrides in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*ciliumiov2alpha1.CiliumBGPNodeConfigOverride, err error)
	// Get retrieves the CiliumBGPNodeConfigOverride from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*ciliumiov2alpha1.CiliumBGPNodeConfigOverride, error)
	CiliumBGPNodeConfigOverrideListerExpansion
}

// ciliumBGPNodeConfigOverrideLister implements the CiliumBGPNodeConfigOverrideLister interface.
type ciliumBGPNodeConfigOverrideLister struct {
	listers.ResourceIndexer[*ciliumiov2alpha1.CiliumBGPNodeConfigOverride]
}

// NewCiliumBGPNodeConfigOverrideLister returns a new CiliumBGPNodeConfigOverrideLister.
func NewCiliumBGPNodeConfigOverrideLister(indexer cache.Indexer) CiliumBGPNodeConfigOverrideLister {
	return &ciliumBGPNodeConfigOverrideLister{listers.New[*ciliumiov2alpha1.CiliumBGPNodeConfigOverride](indexer, ciliumiov2alpha1.Resource("ciliumbgpnodeconfigoverride"))}
}
