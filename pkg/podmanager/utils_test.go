// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
)

var _ = Describe("PodManager utils", Label("pod_manager_utils_test"), func() {
	Describe("Test CheckPodStatus", func() {
		var podT *corev1.Pod

		BeforeEach(func() {
			podT = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "default",
				},
				Spec:   corev1.PodSpec{},
				Status: corev1.PodStatus{},
			}
		})

		It("inputs nil Pod", func() {
			status, allocatable := podmanager.CheckPodStatus(nil)
			Expect(status).To(Equal(constant.PodUnknown))
			Expect(allocatable).To(BeFalse())
		})

		It("checks terminated Pod", func() {
			now := metav1.Now()
			podT.DeletionTimestamp = &now
			podT.DeletionGracePeriodSeconds = pointer.Int64(0)

			status, allocatable := podmanager.CheckPodStatus(podT)
			Expect(status).To(Equal(constant.PodGraceTimeout))
			Expect(allocatable).To(BeFalse())
		})

		It("checks terminating Pod", func() {
			now := metav1.Now()
			podT.DeletionTimestamp = &now
			podT.DeletionGracePeriodSeconds = pointer.Int64(30)

			status, allocatable := podmanager.CheckPodStatus(podT)
			Expect(status).To(Equal(constant.PodTerminating))
			Expect(allocatable).To(BeFalse())
		})

		It("checks succeeded Pod", func() {
			podT.Status.Phase = corev1.PodSucceeded
			podT.Spec.RestartPolicy = corev1.RestartPolicyNever

			status, allocatable := podmanager.CheckPodStatus(podT)
			Expect(status).To(Equal(constant.PodSucceeded))
			Expect(allocatable).To(BeFalse())
		})

		It("checks failed Pod", func() {
			podT.Status.Phase = corev1.PodFailed
			podT.Spec.RestartPolicy = corev1.RestartPolicyNever

			status, allocatable := podmanager.CheckPodStatus(podT)
			Expect(status).To(Equal(constant.PodFailed))
			Expect(allocatable).To(BeFalse())
		})

		It("checks evicted Pod", func() {
			podT.Status.Phase = corev1.PodFailed
			podT.Status.Reason = "Evicted"

			status, allocatable := podmanager.CheckPodStatus(podT)
			Expect(status).To(Equal(constant.PodEvicted))
			Expect(allocatable).To(BeFalse())
		})

		It("checks running Pod", func() {
			status, allocatable := podmanager.CheckPodStatus(podT)
			Expect(status).To(Equal(constant.PodRunning))
			Expect(allocatable).To(BeTrue())
		})
	})
})
