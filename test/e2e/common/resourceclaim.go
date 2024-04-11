// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"errors"
	"fmt"

	frame "github.com/spidernet-io/e2eframework/framework"

	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateResourceClaimTemplate(f *frame.Framework, rct *resourcev1alpha2.ResourceClaimTemplate, opts ...client.CreateOption) error {
	if f == nil || rct == nil {
		return fmt.Errorf("invalid parameters")
	}

	return f.CreateResource(rct, opts...)
}

func DeleteResourceClaimTemplate(f *frame.Framework, name, ns string, opts ...client.DeleteOption) error {
	if name == "" || ns == "" || f == nil {
		return errors.New("wrong input")
	}

	rct := &resourcev1alpha2.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
	return f.DeleteResource(rct, opts...)
}
