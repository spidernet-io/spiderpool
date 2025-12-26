// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"fmt"

	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) GetMultusInstance(name, namespace string) (*v1.NetworkAttachmentDefinition, error) {
	obj := &v1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	key := client.ObjectKeyFromObject(obj)
	nad := &v1.NetworkAttachmentDefinition{}
	if err := f.GetResource(key, nad); err != nil {
		return nil, err
	}
	return nad, nil
}

func (f *Framework) ListMultusInstances(opts ...client.ListOption) (*v1.NetworkAttachmentDefinitionList, error) {
	nads := &v1.NetworkAttachmentDefinitionList{}
	if err := f.ListResource(nads, opts...); err != nil {
		return nil, err
	}

	return nads, nil
}

func (f *Framework) CreateMultusInstance(nad *v1.NetworkAttachmentDefinition, opts ...client.CreateOption) error {
	exist, err := f.GetMultusInstance(nad.Name, nad.Namespace)
	if err == nil && exist.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create %s/%s, instance has exists", nad.Namespace, nad.Name)
	}
	return f.CreateResource(nad, opts...)
}

func (f *Framework) DeleteMultusInstance(name, namespace string) error {
	return f.DeleteResource(&v1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
}
