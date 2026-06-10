// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package iaasnetworkprovider_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Label("iaasnetworkprovider")

func newCaseNamespace(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, common.GenerateString(12, true))
}

func deleteNamespaceUntilFinish(namespace string) {
	Expect(deleteNamespaceUntilFinishWithFallback(frame, namespace)).To(Succeed())
}

func deleteNamespaceUntilFinishWithFallback(f *e2e.Framework, namespace string) error {
	clientset, err := kubernetes.NewForConfig(f.KConfig)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
	defer cancel()

	err = clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return forceFinalizeNamespace(f, namespace, err)
	}

	gracePeriod := int64(0)
	if err = clientset.CoreV1().Pods(namespace).DeleteCollection(ctx, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}, metav1.ListOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete pods in namespace %s: %w", namespace, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("wait namespace %s deleted: %w", namespace, err)
		}
		time.Sleep(time.Second)
	}

	return forceFinalizeNamespace(f, namespace, fmt.Errorf("namespace %s still exists after delete", namespace))
}

func forceFinalizeNamespace(f *e2e.Framework, namespace string, deleteErr error) error {
	GinkgoWriter.Printf("namespace %s delete timed out: %v; force-finalizing disposable e2e namespace\n", namespace, deleteErr)

	clientset, err := kubernetes.NewForConfig(f.KConfig)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
	defer cancel()

	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get namespace %s before finalize: %w", namespace, err)
	}

	ns.Spec.Finalizers = nil
	if _, err = clientset.CoreV1().Namespaces().Finalize(ctx, ns, metav1.UpdateOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("finalize namespace %s after delete error %w: %w", namespace, deleteErr, err)
	}

	deadline := time.Now().Add(common.ResourceDeleteTimeout)
	for time.Now().Before(deadline) {
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("wait namespace %s finalized: %w", namespace, err)
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("namespace %s still exists after force finalize", namespace)
}
