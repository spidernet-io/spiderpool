// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) CreateService(service *corev1.Service, opts ...client.CreateOption) error {
	if service == nil {
		return ErrWrongInput
	}
	// try to wait for finish last deleting
	key := types.NamespacedName{
		Name:      service.Name,
		Namespace: service.Namespace,
	}
	existing := &corev1.Service{}
	e := f.GetResource(key, existing)
	if e == nil && existing.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create , a same service %v/%v exists", service.Namespace, service.Name)
	}
	return f.CreateResource(service, opts...)
}

func (f *Framework) GetService(name, namespace string) (*corev1.Service, error) {
	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	service := &corev1.Service{}
	return service, f.GetResource(key, service)
}

func (f *Framework) ListService(options ...client.ListOption) (*corev1.ServiceList, error) {
	services := &corev1.ServiceList{}
	err := f.ListResource(services, options...)
	if err != nil {
		return nil, err
	}
	return services, nil
}

func (f *Framework) DeleteService(name, namespace string, opts ...client.DeleteOption) error {

	if name == "" || namespace == "" {
		return ErrWrongInput
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(service, opts...)
}
