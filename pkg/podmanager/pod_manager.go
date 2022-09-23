// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error)
	ListPods(ctx context.Context, opts ...client.ListOption) (*corev1.PodList, error)
	MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error
	MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type podManager struct {
	config *PodManagerConfig
	client client.Client
}

func NewPodManager(c *PodManagerConfig, client client.Client) (PodManager, error) {
	if c == nil {
		return nil, errors.New("pod manager config must be specified")
	}
	if client == nil {
		return nil, errors.New("k8s client must be specified")
	}

	return &podManager{
		config: c,
		client: client,
	}, nil
}

func (pm *podManager) GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := pm.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}

func (pm *podManager) ListPods(ctx context.Context, opts ...client.ListOption) (*corev1.PodList, error) {
	var podList corev1.PodList
	if err := pm.client.List(ctx, &podList, opts...); err != nil {
		return nil, err
	}

	return &podList, nil
}

func (pm *podManager) MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= pm.config.MaxConflictRetrys; i++ {
		pod, err := pm.GetPodByName(ctx, namespace, podName)
		if err != nil {
			return err
		}

		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}

		for k, v := range annotations {
			pod.Annotations[k] = v
		}
		if err := pm.client.Update(ctx, pod); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == pm.config.MaxConflictRetrys {
				return fmt.Errorf("insufficient retries(<=%d) to merge Pod annotations", pm.config.MaxConflictRetrys)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * pm.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

func (pm *podManager) MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	podList, err := pm.ListPods(
		ctx,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: podName},
	)
	if err != nil {
		return false, err
	}

	if len(podList.Items) == 0 {
		return false, nil
	}

	return true, nil
}
