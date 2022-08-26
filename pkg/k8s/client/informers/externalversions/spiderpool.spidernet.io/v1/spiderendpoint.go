// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	spiderpoolspidernetiov1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	versioned "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	internalinterfaces "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/internalinterfaces"
	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// SpiderEndpointInformer provides access to a shared informer and lister for
// SpiderEndpoints.
type SpiderEndpointInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.SpiderEndpointLister
}

type spiderEndpointInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewSpiderEndpointInformer constructs a new informer for SpiderEndpoint type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewSpiderEndpointInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredSpiderEndpointInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredSpiderEndpointInformer constructs a new informer for SpiderEndpoint type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredSpiderEndpointInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SpiderpoolV1().SpiderEndpoints(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SpiderpoolV1().SpiderEndpoints(namespace).Watch(context.TODO(), options)
			},
		},
		&spiderpoolspidernetiov1.SpiderEndpoint{},
		resyncPeriod,
		indexers,
	)
}

func (f *spiderEndpointInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredSpiderEndpointInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *spiderEndpointInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&spiderpoolspidernetiov1.SpiderEndpoint{}, f.defaultInformer)
}

func (f *spiderEndpointInformer) Lister() v1.SpiderEndpointLister {
	return v1.NewSpiderEndpointLister(f.Informer().GetIndexer())
}
