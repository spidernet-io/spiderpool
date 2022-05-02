// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"fmt"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
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
				f.t.Logf("waiting for a same namespace %v to finish deleting \n", nsName)
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

func (f *Framework) DeleteNamespace(nsName string, opts ...client.DeleteOption) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
	return f.DeleteResource(ns, opts...)
}
