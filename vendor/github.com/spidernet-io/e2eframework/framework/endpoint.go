// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
// nolint:staticcheck
package framework

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) GetEndpoint(name, namespace string) (*corev1.Endpoints, error) {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	ep := &corev1.Endpoints{}
	return ep, f.GetResource(key, ep)
}

func (f *Framework) ListEndpoint(options ...client.ListOption) (*corev1.EndpointsList, error) {
	eps := &corev1.EndpointsList{}
	err := f.ListResource(eps, options...)
	if err != nil {
		return nil, err
	}
	return eps, nil
}

// CreateEndpoint create an endpoint to testing GetEndpoint/ListEndpoint
func (f *Framework) CreateEndpoint(ep *corev1.Endpoints, opts ...client.CreateOption) error {
	key := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ep.Name,
			Namespace: ep.Namespace,
		},
	}
	fake := client.ObjectKeyFromObject(key)
	existing := &corev1.Endpoints{}
	e := f.GetResource(fake, existing)
	if e == nil && existing.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create , a same endpoint %v/%v exists", ep.Namespace, ep.Name)
	}
	return f.CreateResource(ep, opts...)
}

func (f *Framework) DeleteEndpoint(name, namespace string, opts ...client.DeleteOption) error {
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(ep, opts...)
}
