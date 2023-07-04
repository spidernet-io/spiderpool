// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSpiderMultusConfigs implements SpiderMultusConfigInterface
type FakeSpiderMultusConfigs struct {
	Fake *FakeSpiderpoolV2beta1
	ns   string
}

var spidermultusconfigsResource = schema.GroupVersionResource{Group: "spiderpool.spidernet.io", Version: "v2beta1", Resource: "spidermultusconfigs"}

var spidermultusconfigsKind = schema.GroupVersionKind{Group: "spiderpool.spidernet.io", Version: "v2beta1", Kind: "SpiderMultusConfig"}

// Get takes name of the spiderMultusConfig, and returns the corresponding spiderMultusConfig object, and an error if there is any.
func (c *FakeSpiderMultusConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2beta1.SpiderMultusConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(spidermultusconfigsResource, c.ns, name), &v2beta1.SpiderMultusConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderMultusConfig), err
}

// List takes label and field selectors, and returns the list of SpiderMultusConfigs that match those selectors.
func (c *FakeSpiderMultusConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v2beta1.SpiderMultusConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(spidermultusconfigsResource, spidermultusconfigsKind, c.ns, opts), &v2beta1.SpiderMultusConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2beta1.SpiderMultusConfigList{ListMeta: obj.(*v2beta1.SpiderMultusConfigList).ListMeta}
	for _, item := range obj.(*v2beta1.SpiderMultusConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested spiderMultusConfigs.
func (c *FakeSpiderMultusConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(spidermultusconfigsResource, c.ns, opts))

}

// Create takes the representation of a spiderMultusConfig and creates it.  Returns the server's representation of the spiderMultusConfig, and an error, if there is any.
func (c *FakeSpiderMultusConfigs) Create(ctx context.Context, spiderMultusConfig *v2beta1.SpiderMultusConfig, opts v1.CreateOptions) (result *v2beta1.SpiderMultusConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(spidermultusconfigsResource, c.ns, spiderMultusConfig), &v2beta1.SpiderMultusConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderMultusConfig), err
}

// Update takes the representation of a spiderMultusConfig and updates it. Returns the server's representation of the spiderMultusConfig, and an error, if there is any.
func (c *FakeSpiderMultusConfigs) Update(ctx context.Context, spiderMultusConfig *v2beta1.SpiderMultusConfig, opts v1.UpdateOptions) (result *v2beta1.SpiderMultusConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(spidermultusconfigsResource, c.ns, spiderMultusConfig), &v2beta1.SpiderMultusConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderMultusConfig), err
}

// Delete takes name of the spiderMultusConfig and deletes it. Returns an error if one occurs.
func (c *FakeSpiderMultusConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(spidermultusconfigsResource, c.ns, name, opts), &v2beta1.SpiderMultusConfig{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSpiderMultusConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(spidermultusconfigsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v2beta1.SpiderMultusConfigList{})
	return err
}

// Patch applies the patch and returns the patched spiderMultusConfig.
func (c *FakeSpiderMultusConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2beta1.SpiderMultusConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(spidermultusconfigsResource, c.ns, name, pt, data, subresources...), &v2beta1.SpiderMultusConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderMultusConfig), err
}
