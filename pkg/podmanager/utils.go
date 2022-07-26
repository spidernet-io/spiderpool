// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

func GetControllerOwnerType(pod *corev1.Pod) string {
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return constant.OwnerNone
	}

	var ownerType string
	switch owner.Kind {
	case constant.OwnerDeployment:
		ownerType = constant.OwnerDeployment
	case constant.OwnerStatefulSet:
		ownerType = constant.OwnerStatefulSet
	case constant.OwnerDaemonSet:
		ownerType = constant.OwnerDaemonSet
	case constant.OwnerReplicaSet:
		ownerType = constant.OwnerReplicaSet
	case constant.OwnerJob:
		ownerType = constant.OwnerJob
	default:
		ownerType = constant.OwnerUnknown
	}

	return ownerType
}
