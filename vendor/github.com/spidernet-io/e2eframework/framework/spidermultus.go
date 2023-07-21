// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"fmt"

	spiderv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) GetSpiderMultusInstance(namespace, name string) (*spiderv2beta1.SpiderMultusConfig, error) {
	obj := &spiderv2beta1.SpiderMultusConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	key := client.ObjectKeyFromObject(obj)
	nad := &spiderv2beta1.SpiderMultusConfig{}
	if err := f.GetResource(key, nad); err != nil {
		return nil, err
	}
	return nad, nil
}

func (f *Framework) ListSpiderMultusInstances(opts ...client.ListOption) (*spiderv2beta1.SpiderMultusConfigList, error) {
	nads := &spiderv2beta1.SpiderMultusConfigList{}
	if err := f.ListResource(nads, opts...); err != nil {
		return nil, err
	}

	return nads, nil
}

func (f *Framework) CreateSpiderMultusInstance(nad *spiderv2beta1.SpiderMultusConfig, opts ...client.CreateOption) error {
	exist, err := f.GetMultusInstance(nad.Namespace, nad.Name)
	if err == nil && exist.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create %s/%s, instance has exists", nad.ObjectMeta.Namespace, nad.ObjectMeta.Name)
	}
	return f.CreateResource(nad, opts...)
}

func (f *Framework) DeleteSpiderMultusInstance(namespace, name string) error {
	return f.DeleteResource(&spiderv2beta1.SpiderMultusConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
}
