// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

var ConstApiTimeOut = time.Second * 30

func InitK8sClient() client.Client {

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		logger.Sugar().Fatalf("failed to add corev1 runtime Scheme : %+v ", err)
	}
	if err := spiderpoolv1.AddToScheme(scheme); err != nil {
		logger.Sugar().Fatalf("failed to add spiderpoolv1 runtime Scheme : %+v ", err)
	}

	runtimeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if nil != err {
		logger.Sugar().Fatalf("failed to generate clientset , %+v", err)
	}

	return runtimeClient
}

func k8sCheckEndpointAvailable(runtimeClient client.Client, name, namespace string) (bool, error) {

	v := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	key := client.ObjectKeyFromObject(v)
	existing := &corev1.Endpoints{}

	ctx4, cancel4 := context.WithTimeout(context.Background(), ConstApiTimeOut)
	defer cancel4()

	e := runtimeClient.Get(ctx4, key, existing)
	if e != nil {
		if apierrors.IsNotFound(e) {
			return false, nil
		}
		return false, e
	}
	logger.Sugar().Infof("endpoints: %+v ", existing)

	if len(existing.Subsets) > 0 {
		return true, nil
	}
	return false, nil
}

func k8sCreateIppool(runtimeClient client.Client, pool *spiderpoolv1.SpiderIPPool) error {

	ctx4, cancel4 := context.WithTimeout(context.Background(), ConstApiTimeOut)
	defer cancel4()
	return runtimeClient.Create(ctx4, pool)

}

func k8sCreateSubnet(runtimeClient client.Client, subnet *spiderpoolv1.SpiderSubnet) error {

	ctx4, cancel4 := context.WithTimeout(context.Background(), ConstApiTimeOut)
	defer cancel4()
	return runtimeClient.Create(ctx4, subnet)

}

func k8sCheckIppoolExisted(runtimeClient client.Client, name string) (*spiderpoolv1.SpiderIPPool, error) {
	v := apitypes.NamespacedName{Name: name}
	existing := &spiderpoolv1.SpiderIPPool{}

	ctx4, cancel4 := context.WithTimeout(context.Background(), ConstApiTimeOut)
	defer cancel4()

	e := runtimeClient.Get(ctx4, v, existing)
	if e != nil {
		if apierrors.IsNotFound(e) {
			return nil, nil
		}
		return nil, e
	}
	return existing, nil
}

func k8sCheckSubnetExisted(runtimeClient client.Client, name string) (*spiderpoolv1.SpiderSubnet, error) {
	v := apitypes.NamespacedName{Name: name}
	existing := &spiderpoolv1.SpiderSubnet{}

	ctx4, cancel4 := context.WithTimeout(context.Background(), ConstApiTimeOut)
	defer cancel4()

	e := runtimeClient.Get(ctx4, v, existing)
	if e != nil {
		if apierrors.IsNotFound(e) {
			return nil, nil
		}
		return nil, e
	}
	return existing, nil
}
