// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (f *Framework) GetServiceAccount(saName, namespace string) (*corev1.ServiceAccount, error) {
	if saName == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	key := client.ObjectKey{
		Namespace: namespace,
		Name:      saName,
	}
	existing := &corev1.ServiceAccount{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) WaitServiceAccountReady(saName, namespace string, timeout time.Duration) error {
	if saName == "" || namespace == "" {
		return ErrWrongInput
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		default:
			as, _ := f.GetServiceAccount(saName, namespace)
			if as != nil {
				return nil
			}
			time.Sleep(time.Second)
		case <-ctx.Done():
			return ErrTimeOut
		}
	}
}
