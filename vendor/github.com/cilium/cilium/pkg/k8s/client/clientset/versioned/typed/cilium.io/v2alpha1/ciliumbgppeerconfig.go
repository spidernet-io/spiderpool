// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

// Code generated by client-gen. DO NOT EDIT.

package v2alpha1

import (
	"context"
	"time"

	v2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	scheme "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// CiliumBGPPeerConfigsGetter has a method to return a CiliumBGPPeerConfigInterface.
// A group's client should implement this interface.
type CiliumBGPPeerConfigsGetter interface {
	CiliumBGPPeerConfigs() CiliumBGPPeerConfigInterface
}

// CiliumBGPPeerConfigInterface has methods to work with CiliumBGPPeerConfig resources.
type CiliumBGPPeerConfigInterface interface {
	Create(ctx context.Context, ciliumBGPPeerConfig *v2alpha1.CiliumBGPPeerConfig, opts v1.CreateOptions) (*v2alpha1.CiliumBGPPeerConfig, error)
	Update(ctx context.Context, ciliumBGPPeerConfig *v2alpha1.CiliumBGPPeerConfig, opts v1.UpdateOptions) (*v2alpha1.CiliumBGPPeerConfig, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v2alpha1.CiliumBGPPeerConfig, error)
	List(ctx context.Context, opts v1.ListOptions) (*v2alpha1.CiliumBGPPeerConfigList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.CiliumBGPPeerConfig, err error)
	CiliumBGPPeerConfigExpansion
}

// ciliumBGPPeerConfigs implements CiliumBGPPeerConfigInterface
type ciliumBGPPeerConfigs struct {
	client rest.Interface
}

// newCiliumBGPPeerConfigs returns a CiliumBGPPeerConfigs
func newCiliumBGPPeerConfigs(c *CiliumV2alpha1Client) *ciliumBGPPeerConfigs {
	return &ciliumBGPPeerConfigs{
		client: c.RESTClient(),
	}
}

// Get takes name of the ciliumBGPPeerConfig, and returns the corresponding ciliumBGPPeerConfig object, and an error if there is any.
func (c *ciliumBGPPeerConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.CiliumBGPPeerConfig, err error) {
	result = &v2alpha1.CiliumBGPPeerConfig{}
	err = c.client.Get().
		Resource("ciliumbgppeerconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of CiliumBGPPeerConfigs that match those selectors.
func (c *ciliumBGPPeerConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.CiliumBGPPeerConfigList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v2alpha1.CiliumBGPPeerConfigList{}
	err = c.client.Get().
		Resource("ciliumbgppeerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested ciliumBGPPeerConfigs.
func (c *ciliumBGPPeerConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("ciliumbgppeerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a ciliumBGPPeerConfig and creates it.  Returns the server's representation of the ciliumBGPPeerConfig, and an error, if there is any.
func (c *ciliumBGPPeerConfigs) Create(ctx context.Context, ciliumBGPPeerConfig *v2alpha1.CiliumBGPPeerConfig, opts v1.CreateOptions) (result *v2alpha1.CiliumBGPPeerConfig, err error) {
	result = &v2alpha1.CiliumBGPPeerConfig{}
	err = c.client.Post().
		Resource("ciliumbgppeerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(ciliumBGPPeerConfig).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a ciliumBGPPeerConfig and updates it. Returns the server's representation of the ciliumBGPPeerConfig, and an error, if there is any.
func (c *ciliumBGPPeerConfigs) Update(ctx context.Context, ciliumBGPPeerConfig *v2alpha1.CiliumBGPPeerConfig, opts v1.UpdateOptions) (result *v2alpha1.CiliumBGPPeerConfig, err error) {
	result = &v2alpha1.CiliumBGPPeerConfig{}
	err = c.client.Put().
		Resource("ciliumbgppeerconfigs").
		Name(ciliumBGPPeerConfig.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(ciliumBGPPeerConfig).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the ciliumBGPPeerConfig and deletes it. Returns an error if one occurs.
func (c *ciliumBGPPeerConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("ciliumbgppeerconfigs").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *ciliumBGPPeerConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("ciliumbgppeerconfigs").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched ciliumBGPPeerConfig.
func (c *ciliumBGPPeerConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.CiliumBGPPeerConfig, err error) {
	result = &v2alpha1.CiliumBGPPeerConfig{}
	err = c.client.Patch(pt).
		Resource("ciliumbgppeerconfigs").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
