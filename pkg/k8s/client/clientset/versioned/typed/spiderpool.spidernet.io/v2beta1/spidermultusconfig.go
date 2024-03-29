// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v2beta1

import (
	"context"
	"time"

	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	scheme "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SpiderMultusConfigsGetter has a method to return a SpiderMultusConfigInterface.
// A group's client should implement this interface.
type SpiderMultusConfigsGetter interface {
	SpiderMultusConfigs(namespace string) SpiderMultusConfigInterface
}

// SpiderMultusConfigInterface has methods to work with SpiderMultusConfig resources.
type SpiderMultusConfigInterface interface {
	Create(ctx context.Context, spiderMultusConfig *v2beta1.SpiderMultusConfig, opts v1.CreateOptions) (*v2beta1.SpiderMultusConfig, error)
	Update(ctx context.Context, spiderMultusConfig *v2beta1.SpiderMultusConfig, opts v1.UpdateOptions) (*v2beta1.SpiderMultusConfig, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v2beta1.SpiderMultusConfig, error)
	List(ctx context.Context, opts v1.ListOptions) (*v2beta1.SpiderMultusConfigList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2beta1.SpiderMultusConfig, err error)
	SpiderMultusConfigExpansion
}

// spiderMultusConfigs implements SpiderMultusConfigInterface
type spiderMultusConfigs struct {
	client rest.Interface
	ns     string
}

// newSpiderMultusConfigs returns a SpiderMultusConfigs
func newSpiderMultusConfigs(c *SpiderpoolV2beta1Client, namespace string) *spiderMultusConfigs {
	return &spiderMultusConfigs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the spiderMultusConfig, and returns the corresponding spiderMultusConfig object, and an error if there is any.
func (c *spiderMultusConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2beta1.SpiderMultusConfig, err error) {
	result = &v2beta1.SpiderMultusConfig{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SpiderMultusConfigs that match those selectors.
func (c *spiderMultusConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v2beta1.SpiderMultusConfigList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v2beta1.SpiderMultusConfigList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested spiderMultusConfigs.
func (c *spiderMultusConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a spiderMultusConfig and creates it.  Returns the server's representation of the spiderMultusConfig, and an error, if there is any.
func (c *spiderMultusConfigs) Create(ctx context.Context, spiderMultusConfig *v2beta1.SpiderMultusConfig, opts v1.CreateOptions) (result *v2beta1.SpiderMultusConfig, err error) {
	result = &v2beta1.SpiderMultusConfig{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(spiderMultusConfig).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a spiderMultusConfig and updates it. Returns the server's representation of the spiderMultusConfig, and an error, if there is any.
func (c *spiderMultusConfigs) Update(ctx context.Context, spiderMultusConfig *v2beta1.SpiderMultusConfig, opts v1.UpdateOptions) (result *v2beta1.SpiderMultusConfig, err error) {
	result = &v2beta1.SpiderMultusConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		Name(spiderMultusConfig.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(spiderMultusConfig).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the spiderMultusConfig and deletes it. Returns an error if one occurs.
func (c *spiderMultusConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *spiderMultusConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched spiderMultusConfig.
func (c *spiderMultusConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2beta1.SpiderMultusConfig, err error) {
	result = &v2beta1.SpiderMultusConfig{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("spidermultusconfigs").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
