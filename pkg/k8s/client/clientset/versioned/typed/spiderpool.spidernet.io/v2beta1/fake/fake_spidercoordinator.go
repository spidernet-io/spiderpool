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

// FakeSpiderCoordinators implements SpiderCoordinatorInterface
type FakeSpiderCoordinators struct {
	Fake *FakeSpiderpoolV2beta1
}

var spidercoordinatorsResource = schema.GroupVersionResource{Group: "spiderpool.spidernet.io", Version: "v2beta1", Resource: "spidercoordinators"}

var spidercoordinatorsKind = schema.GroupVersionKind{Group: "spiderpool.spidernet.io", Version: "v2beta1", Kind: "SpiderCoordinator"}

// Get takes name of the spiderCoordinator, and returns the corresponding spiderCoordinator object, and an error if there is any.
func (c *FakeSpiderCoordinators) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2beta1.SpiderCoordinator, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(spidercoordinatorsResource, name), &v2beta1.SpiderCoordinator{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderCoordinator), err
}

// List takes label and field selectors, and returns the list of SpiderCoordinators that match those selectors.
func (c *FakeSpiderCoordinators) List(ctx context.Context, opts v1.ListOptions) (result *v2beta1.SpiderCoordinatorList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(spidercoordinatorsResource, spidercoordinatorsKind, opts), &v2beta1.SpiderCoordinatorList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2beta1.SpiderCoordinatorList{ListMeta: obj.(*v2beta1.SpiderCoordinatorList).ListMeta}
	for _, item := range obj.(*v2beta1.SpiderCoordinatorList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested spiderCoordinators.
func (c *FakeSpiderCoordinators) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(spidercoordinatorsResource, opts))
}

// Create takes the representation of a spiderCoordinator and creates it.  Returns the server's representation of the spiderCoordinator, and an error, if there is any.
func (c *FakeSpiderCoordinators) Create(ctx context.Context, spiderCoordinator *v2beta1.SpiderCoordinator, opts v1.CreateOptions) (result *v2beta1.SpiderCoordinator, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(spidercoordinatorsResource, spiderCoordinator), &v2beta1.SpiderCoordinator{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderCoordinator), err
}

// Update takes the representation of a spiderCoordinator and updates it. Returns the server's representation of the spiderCoordinator, and an error, if there is any.
func (c *FakeSpiderCoordinators) Update(ctx context.Context, spiderCoordinator *v2beta1.SpiderCoordinator, opts v1.UpdateOptions) (result *v2beta1.SpiderCoordinator, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(spidercoordinatorsResource, spiderCoordinator), &v2beta1.SpiderCoordinator{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderCoordinator), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeSpiderCoordinators) UpdateStatus(ctx context.Context, spiderCoordinator *v2beta1.SpiderCoordinator, opts v1.UpdateOptions) (*v2beta1.SpiderCoordinator, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(spidercoordinatorsResource, "status", spiderCoordinator), &v2beta1.SpiderCoordinator{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderCoordinator), err
}

// Delete takes name of the spiderCoordinator and deletes it. Returns an error if one occurs.
func (c *FakeSpiderCoordinators) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(spidercoordinatorsResource, name, opts), &v2beta1.SpiderCoordinator{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSpiderCoordinators) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(spidercoordinatorsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v2beta1.SpiderCoordinatorList{})
	return err
}

// Patch applies the patch and returns the patched spiderCoordinator.
func (c *FakeSpiderCoordinators) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2beta1.SpiderCoordinator, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(spidercoordinatorsResource, name, pt, data, subresources...), &v2beta1.SpiderCoordinator{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v2beta1.SpiderCoordinator), err
}
