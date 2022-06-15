// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	frame "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetIppoolByName(f frame.Framework, name string) *spiderpool.IPPool {
	if name == "" {
		return nil
	}

	pod := &spiderpool.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	key := client.ObjectKeyFromObject(pod)
	existing := &spiderpool.IPPool{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil
	}
	return existing
}
