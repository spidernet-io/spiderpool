// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) CreateNamespace(nsName string, opts ...client.CreateOption) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			// Labels: map[string]string{"spiderpool-e2e-ns": "true"},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
	}

	key := client.ObjectKeyFromObject(ns)
	existing := &corev1.Namespace{}
	e := f.GetResource(key, existing)

	if e == nil && existing.Status.Phase == corev1.NamespaceTerminating {
		r := func() bool {
			existing := &corev1.Namespace{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				f.Log("waiting for a same namespace %v to finish deleting \n", nsName)
				return false
			}
			return true
		}
		if !tools.Eventually(r, f.Config.ResourceDeleteTimeout, time.Second) {
			return fmt.Errorf("time out to wait a deleting namespace")
		}
	}
	return f.CreateResource(ns, opts...)
}

func (f *Framework) CreateNamespaceUntilDefaultServiceAccountReady(nsName string, timeoutForSA time.Duration, opts ...client.CreateOption) error {
	if nsName == "" {
		return ErrWrongInput
	}
	err := f.CreateNamespace(nsName, opts...)
	if err != nil {
		return err
	}
	if timeoutForSA != 0 {
		err = f.WaitServiceAccountReady("default", nsName, timeoutForSA)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) GetNamespace(nsName string) (*corev1.Namespace, error) {
	if nsName == "" {
		return nil, ErrWrongInput
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
	}
	key := client.ObjectKeyFromObject(ns)
	namespace := &corev1.Namespace{}
	err := f.GetResource(key, namespace)
	if err != nil {
		return nil, err
	}
	return namespace, nil
}

func (f *Framework) DeleteNamespace(nsName string, opts ...client.DeleteOption) error {

	if nsName == "" {
		return ErrWrongInput
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
	return f.DeleteResource(ns, opts...)
}

func (f *Framework) DeleteNamespaceUntilFinish(nsName string, ctx context.Context, opts ...client.DeleteOption) error {
	if nsName == "" {
		return ErrWrongInput
	}
	err := f.DeleteNamespace(nsName, opts...)
	if err != nil {
		return err
	}
	for {
		select {
		default:
			namespace, _ := f.GetNamespace(nsName)
			if namespace == nil {
				return nil
			}
			time.Sleep(time.Second)
		case <-ctx.Done():
			return ErrTimeOut
		}
	}

}
