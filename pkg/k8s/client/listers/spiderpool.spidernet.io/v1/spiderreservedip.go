// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// SpiderReservedIPLister helps list SpiderReservedIPs.
// All objects returned here must be treated as read-only.
type SpiderReservedIPLister interface {
	// List lists all SpiderReservedIPs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.SpiderReservedIP, err error)
	// SpiderReservedIPs returns an object that can list and get SpiderReservedIPs.
	SpiderReservedIPs(namespace string) SpiderReservedIPNamespaceLister
	SpiderReservedIPListerExpansion
}

// spiderReservedIPLister implements the SpiderReservedIPLister interface.
type spiderReservedIPLister struct {
	indexer cache.Indexer
}

// NewSpiderReservedIPLister returns a new SpiderReservedIPLister.
func NewSpiderReservedIPLister(indexer cache.Indexer) SpiderReservedIPLister {
	return &spiderReservedIPLister{indexer: indexer}
}

// List lists all SpiderReservedIPs in the indexer.
func (s *spiderReservedIPLister) List(selector labels.Selector) (ret []*v1.SpiderReservedIP, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.SpiderReservedIP))
	})
	return ret, err
}

// SpiderReservedIPs returns an object that can list and get SpiderReservedIPs.
func (s *spiderReservedIPLister) SpiderReservedIPs(namespace string) SpiderReservedIPNamespaceLister {
	return spiderReservedIPNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SpiderReservedIPNamespaceLister helps list and get SpiderReservedIPs.
// All objects returned here must be treated as read-only.
type SpiderReservedIPNamespaceLister interface {
	// List lists all SpiderReservedIPs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.SpiderReservedIP, err error)
	// Get retrieves the SpiderReservedIP from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.SpiderReservedIP, error)
	SpiderReservedIPNamespaceListerExpansion
}

// spiderReservedIPNamespaceLister implements the SpiderReservedIPNamespaceLister
// interface.
type spiderReservedIPNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all SpiderReservedIPs in the indexer for a given namespace.
func (s spiderReservedIPNamespaceLister) List(selector labels.Selector) (ret []*v1.SpiderReservedIP, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.SpiderReservedIP))
	})
	return ret, err
}

// Get retrieves the SpiderReservedIP from the indexer for a given namespace and name.
func (s spiderReservedIPNamespaceLister) Get(name string) (*v1.SpiderReservedIP, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("spiderreservedip"), name)
	}
	return obj.(*v1.SpiderReservedIP), nil
}
