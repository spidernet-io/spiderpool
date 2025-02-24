// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

// Code generated by informer-gen. DO NOT EDIT.

package v2

import (
	context "context"
	time "time"

	apisciliumiov2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	versioned "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	internalinterfaces "github.com/cilium/cilium/pkg/k8s/client/informers/externalversions/internalinterfaces"
	ciliumiov2 "github.com/cilium/cilium/pkg/k8s/client/listers/cilium.io/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CiliumEnvoyConfigInformer provides access to a shared informer and lister for
// CiliumEnvoyConfigs.
type CiliumEnvoyConfigInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() ciliumiov2.CiliumEnvoyConfigLister
}

type ciliumEnvoyConfigInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCiliumEnvoyConfigInformer constructs a new informer for CiliumEnvoyConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCiliumEnvoyConfigInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCiliumEnvoyConfigInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCiliumEnvoyConfigInformer constructs a new informer for CiliumEnvoyConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCiliumEnvoyConfigInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CiliumV2().CiliumEnvoyConfigs(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CiliumV2().CiliumEnvoyConfigs(namespace).Watch(context.TODO(), options)
			},
		},
		&apisciliumiov2.CiliumEnvoyConfig{},
		resyncPeriod,
		indexers,
	)
}

func (f *ciliumEnvoyConfigInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCiliumEnvoyConfigInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *ciliumEnvoyConfigInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apisciliumiov2.CiliumEnvoyConfig{}, f.defaultInformer)
}

func (f *ciliumEnvoyConfigInformer) Lister() ciliumiov2.CiliumEnvoyConfigLister {
	return ciliumiov2.NewCiliumEnvoyConfigLister(f.Informer().GetIndexer())
}
