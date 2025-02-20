// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package podmanager_test

// import (
// 	"context"
// 	"testing"

// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"github.com/spidernet-io/spiderpool/pkg/podmanager"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/client-go/rest"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/manager"
// )

// func TestPodWebhook(t *testing.T) {
// 	RegisterFailHandler(Fail)
// 	RunSpecs(t, "Pod Webhook Suite")
// }

// var _ = Describe("Pod Webhook", Label("pod_webhook_test"), func() {
// 	var (
// 		err error
// 		mgr manager.Manager
// 		ctx context.Context
// 		pod *corev1.Pod
// 	)

// 	BeforeEach(func() {
// 		// Create a new manager

// 		mgr, err = manager.New(cfg, manager.Options{
// 			NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
// 				return nil, nil
// 			},
// 		})
// 		Expect(err).NotTo(HaveOccurred())

// 		pod = &corev1.Pod{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Namespace:    "default",
// 				GenerateName: "test-pod",
// 				Annotations:  map[string]string{},
// 			},
// 		}
// 	})

// 	Context("InitPodWebhook", func() {
// 		It("should initialize without error", func() {
// 			err := podmanager.InitPodWebhook(mgr)
// 			Expect(err).NotTo(HaveOccurred())
// 		})
// 	})

// 	Context("Default", func() {
// 		It("should not inject resources if no annotations are present", func() {
// 			pw := &podmanager.PWebhook{}
// 			err := pw.Default(ctx, pod)
// 			Expect(err).NotTo(HaveOccurred())
// 		})

// 		It("should inject resources if annotations are present", func() {
// 			pod.Annotations["spiderpool.io/pod-resource-inject"] = "true"
// 			pw := podmanager.PWebhook{}
// 			err := pw.Default(ctx, pod)
// 			Expect(err).NotTo(HaveOccurred())
// 		})
// 	})
// })
