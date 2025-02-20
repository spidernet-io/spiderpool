// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

// import (
// 	"errors"
// 	"fmt"

// 	frame "github.com/spidernet-io/e2eframework/framework"

// 	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	apitypes "k8s.io/apimachinery/pkg/types"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// func ListResourceClaim(f *frame.Framework, opts ...client.ListOption) (*resourcev1alpha2.ResourceClaimList, error) {
// 	list := resourcev1alpha2.ResourceClaimList{}
// 	if err := f.ListResource(&list, opts...); err != nil {
// 		return nil, err
// 	}

// 	return &list, nil
// }

// func GetResourceClaim(f *frame.Framework, name, ns string) (*resourcev1alpha2.ResourceClaim, error) {
// 	if name == "" || f == nil {
// 		return nil, errors.New("wrong input")
// 	}

// 	v := apitypes.NamespacedName{Name: name, Namespace: ns}
// 	existing := &resourcev1alpha2.ResourceClaim{}
// 	e := f.GetResource(v, existing)
// 	if e != nil {
// 		return nil, e
// 	}
// 	return existing, nil
// }

// func CreateResourceClaimTemplate(f *frame.Framework, rct *resourcev1alpha2.ResourceClaimTemplate, opts ...client.CreateOption) error {
// 	if f == nil || rct == nil {
// 		return fmt.Errorf("invalid parameters")
// 	}

// 	return f.CreateResource(rct, opts...)
// }

// func DeleteResourceClaimTemplate(f *frame.Framework, name, ns string, opts ...client.DeleteOption) error {
// 	if name == "" || ns == "" || f == nil {
// 		return errors.New("wrong input")
// 	}

// 	rct := &resourcev1alpha2.ResourceClaimTemplate{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: ns,
// 		},
// 	}
// 	return f.DeleteResource(rct, opts...)
// }
