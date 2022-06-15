// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	frame "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

func (f *Framework) GetIppoolByName( poolName string) ( *spiderpool.IPPool , error) {
	if poolName == "" {
		return nil,ErrWrongInput
	}

	v := apitypes.NamespacedName{Name: poolName}
	existing := &spiderpool.IPPool{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil , e
	}
	return existing , nil
}

func (f *Framework) GetWorkloadByName( namespace , name string) ( *spiderpool.IPPool , error) {
	if name == "" || namespace == "" {
		return nil,ErrWrongInput
	}

	v := apitypes.NamespacedName{Name: name, Namespace: namespace }
	existing := &spiderpool.WorkloadEndpoint{}
	e := f.GetResource(v, existing)
	if e != nil {
		return nil , e
	}
	return existing , nil
}
