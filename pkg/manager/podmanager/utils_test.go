// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/manager/podmanager"
)

var _ = Describe("PodManager utils", Label("pod_manager_utils_test"), func() {
	var podT *corev1.Pod

	BeforeEach(func() {
		podT = &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		}
	})

	Describe("Test IsPodAlive", func() {
		It("inputs nil Pod", func() {
			isAlive := podmanager.IsPodAlive(nil)
			Expect(isAlive).To(BeFalse())
		})

		It("checks terminating Pod", func() {
			now := metav1.Now()
			podT.SetDeletionTimestamp(&now)
			podT.SetDeletionGracePeriodSeconds(ptr.To(int64(30)))

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks succeeded Pod", func() {
			podT.Status.Phase = corev1.PodSucceeded
			podT.Spec.RestartPolicy = corev1.RestartPolicyNever

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks failed Pod", func() {
			podT.Status.Phase = corev1.PodFailed
			podT.Spec.RestartPolicy = corev1.RestartPolicyNever

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks evicted Pod", func() {
			podT.Status.Phase = corev1.PodFailed
			podT.Status.Reason = "Evicted"

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks running Pod", func() {
			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeTrue())
		})
	})
})
