// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v2beta1

import (
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// SpiderMultusConfigLister helps list SpiderMultusConfigs.
// All objects returned here must be treated as read-only.
type SpiderMultusConfigLister interface {
	// List lists all SpiderMultusConfigs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2beta1.SpiderMultusConfig, err error)
	// SpiderMultusConfigs returns an object that can list and get SpiderMultusConfigs.
	SpiderMultusConfigs(namespace string) SpiderMultusConfigNamespaceLister
	SpiderMultusConfigListerExpansion
}

// spiderMultusConfigLister implements the SpiderMultusConfigLister interface.
type spiderMultusConfigLister struct {
	indexer cache.Indexer
}

// NewSpiderMultusConfigLister returns a new SpiderMultusConfigLister.
func NewSpiderMultusConfigLister(indexer cache.Indexer) SpiderMultusConfigLister {
	return &spiderMultusConfigLister{indexer: indexer}
}

// List lists all SpiderMultusConfigs in the indexer.
func (s *spiderMultusConfigLister) List(selector labels.Selector) (ret []*v2beta1.SpiderMultusConfig, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v2beta1.SpiderMultusConfig))
	})
	return ret, err
}

// SpiderMultusConfigs returns an object that can list and get SpiderMultusConfigs.
func (s *spiderMultusConfigLister) SpiderMultusConfigs(namespace string) SpiderMultusConfigNamespaceLister {
	return spiderMultusConfigNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SpiderMultusConfigNamespaceLister helps list and get SpiderMultusConfigs.
// All objects returned here must be treated as read-only.
type SpiderMultusConfigNamespaceLister interface {
	// List lists all SpiderMultusConfigs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2beta1.SpiderMultusConfig, err error)
	// Get retrieves the SpiderMultusConfig from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v2beta1.SpiderMultusConfig, error)
	SpiderMultusConfigNamespaceListerExpansion
}

// spiderMultusConfigNamespaceLister implements the SpiderMultusConfigNamespaceLister
// interface.
type spiderMultusConfigNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all SpiderMultusConfigs in the indexer for a given namespace.
func (s spiderMultusConfigNamespaceLister) List(selector labels.Selector) (ret []*v2beta1.SpiderMultusConfig, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v2beta1.SpiderMultusConfig))
	})
	return ret, err
}

// Get retrieves the SpiderMultusConfig from the indexer for a given namespace and name.
func (s spiderMultusConfigNamespaceLister) Get(name string) (*v2beta1.SpiderMultusConfig, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v2beta1.Resource("spidermultusconfig"), name)
	}
	return obj.(*v2beta1.SpiderMultusConfig), nil
}
