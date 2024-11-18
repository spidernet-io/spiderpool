// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

// Code generated by informer-gen. DO NOT EDIT.

package v2alpha1

import (
	"context"
	time "time"

	ciliumiov2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	versioned "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	internalinterfaces "github.com/cilium/cilium/pkg/k8s/client/informers/externalversions/internalinterfaces"
	v2alpha1 "github.com/cilium/cilium/pkg/k8s/client/listers/cilium.io/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CiliumBGPNodeConfigInformer provides access to a shared informer and lister for
// CiliumBGPNodeConfigs.
type CiliumBGPNodeConfigInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v2alpha1.CiliumBGPNodeConfigLister
}

type ciliumBGPNodeConfigInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewCiliumBGPNodeConfigInformer constructs a new informer for CiliumBGPNodeConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCiliumBGPNodeConfigInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCiliumBGPNodeConfigInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredCiliumBGPNodeConfigInformer constructs a new informer for CiliumBGPNodeConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCiliumBGPNodeConfigInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CiliumV2alpha1().CiliumBGPNodeConfigs().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CiliumV2alpha1().CiliumBGPNodeConfigs().Watch(context.TODO(), options)
			},
		},
		&ciliumiov2alpha1.CiliumBGPNodeConfig{},
		resyncPeriod,
		indexers,
	)
}

func (f *ciliumBGPNodeConfigInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCiliumBGPNodeConfigInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *ciliumBGPNodeConfigInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&ciliumiov2alpha1.CiliumBGPNodeConfig{}, f.defaultInformer)
}

func (f *ciliumBGPNodeConfigInformer) Lister() v2alpha1.CiliumBGPNodeConfigLister {
	return v2alpha1.NewCiliumBGPNodeConfigLister(f.Informer().GetIndexer())
}
