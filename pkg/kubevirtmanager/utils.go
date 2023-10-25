// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package kubevirtmanager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

func isVMIControlledByVM(vmi *kubevirtv1.VirtualMachineInstance) bool {
	ownerReference := metav1.GetControllerOf(vmi)
	if ownerReference == nil {
		return false
	}

	if ownerReference.APIVersion == kubevirtv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindKubevirtVM {
		return true
	}

	return false
}
