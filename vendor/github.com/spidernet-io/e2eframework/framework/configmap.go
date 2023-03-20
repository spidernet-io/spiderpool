// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (f *Framework) GetConfigmap(name, namespace string) (*corev1.ConfigMap, error) {
	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	key := apitypes.NamespacedName{Namespace: namespace, Name: name}
	existing := &corev1.ConfigMap{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) CreateConfigmap(configMap *corev1.ConfigMap, opts ...client.CreateOption) error {
	if configMap == nil {
		return ErrWrongInput
	}

	fake := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: configMap.Namespace,
			Name:      configMap.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &corev1.ConfigMap{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return ErrAlreadyExisted
	}
	t := func() bool {
		existing := &corev1.ConfigMap{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.Log("waiting for a same configmap %v/%v to finish deleting \n", configMap.ObjectMeta.Namespace, configMap.ObjectMeta.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return ErrTimeOut
	}
	return f.CreateResource(configMap, opts...)
}

func (f *Framework) DeleteConfigmap(name, namespace string, opts ...client.DeleteOption) error {
	if name == "" || namespace == "" {
		return ErrWrongInput
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return f.DeleteResource(cm, opts...)
}
