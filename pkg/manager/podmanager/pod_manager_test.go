// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kruiseapi "github.com/openkruise/kruise-api"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/manager/podmanager"
)

var _ = Describe("PodManager", Label("pod_manager_test"), func() {
	Describe("New PodManager", func() {
		It("inputs nil client", func() {
			manager, err := podmanager.NewPodManager(false, nil, fakeAPIReader, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := podmanager.NewPodManager(false, fakeClient, nil, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test PodManager's method", func() {
		var ctx context.Context

		var count uint64
		var namespace string
		var podName string
		var labels map[string]string
		var podT *corev1.Pod

		BeforeEach(func() {
			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			namespace = "default"
			podName = fmt.Sprintf("pod-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			podT = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: ptr.To(int64(0)),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, podT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    corev1.GroupName,
					Version:  corev1.SchemeGroupVersion.Version,
					Resource: "pods",
				},
				podT.Namespace,
				podT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetPodByName", func() {
			It("gets non-existent Pod", func() {
				pod, err := podManager.GetPodByName(ctx, namespace, podName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(pod).To(BeNil())
			})

			It("gets an existing Pod through cache", func() {
				err := fakeClient.Create(ctx, podT)
				Expect(err).NotTo(HaveOccurred())

				pod, err := podManager.GetPodByName(ctx, namespace, podName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod).NotTo(BeNil())
				Expect(pod).To(Equal(podT))
			})

			It("gets an existing Pod through API Server", func() {
				err := tracker.Add(podT)
				Expect(err).NotTo(HaveOccurred())

				pod, err := podManager.GetPodByName(ctx, namespace, podName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod).NotTo(BeNil())
				Expect(pod).To(Equal(podT))
			})
		})

		Describe("ListPods", func() {
			It("failed to list Pods due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(podT)
				Expect(err).NotTo(HaveOccurred())

				podList, err := podManager.ListPods(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(podList).To(BeNil())
			})

			It("lists all Pods through cache", func() {
				err := fakeClient.Create(ctx, podT)
				Expect(err).NotTo(HaveOccurred())

				podList, err := podManager.ListPods(ctx, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(podList.Items).NotTo(BeEmpty())

				hasPod := false
				for _, pod := range podList.Items {
					if pod.Name == podName {
						hasPod = true
						break
					}
				}
				Expect(hasPod).To(BeTrue())
			})

			It("lists all Pods through API Server", func() {
				err := tracker.Add(podT)
				Expect(err).NotTo(HaveOccurred())

				podList, err := podManager.ListPods(ctx, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(podList.Items).NotTo(BeEmpty())

				hasPod := false
				for _, pod := range podList.Items {
					if pod.Name == podName {
						hasPod = true
						break
					}
				}
				Expect(hasPod).To(BeTrue())
			})

			It("filters results by Namespace", func() {
				err := tracker.Add(podT)
				Expect(err).NotTo(HaveOccurred())

				podList, err := podManager.ListPods(ctx, constant.IgnoreCache, client.InNamespace(namespace))
				Expect(err).NotTo(HaveOccurred())
				Expect(podList.Items).NotTo(BeEmpty())

				hasPod := false
				for _, pod := range podList.Items {
					if pod.Name == podName {
						hasPod = true
						break
					}
				}
				Expect(hasPod).To(BeTrue())
			})

			It("filters results by label selector", func() {
				err := tracker.Add(podT)
				Expect(err).NotTo(HaveOccurred())

				podList, err := podManager.ListPods(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(podList.Items).NotTo(BeEmpty())

				hasPod := false
				for _, pod := range podList.Items {
					if pod.Name == podName {
						hasPod = true
						break
					}
				}
				Expect(hasPod).To(BeTrue())
			})

			It("filters results by field selector", func() {
				err := tracker.Add(podT)
				Expect(err).NotTo(HaveOccurred())

				podList, err := podManager.ListPods(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: podName})
				Expect(err).NotTo(HaveOccurred())
				Expect(podList.Items).NotTo(BeEmpty())

				hasPod := false
				for _, pod := range podList.Items {
					if pod.Name == podName {
						hasPod = true
						break
					}
				}
				Expect(hasPod).To(BeTrue())
			})
		})

		Describe("GetPodTopController", func() {
			It("Orphan Pod without any controllers", func() {
				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindPod))
			})

			It("Pod with third-party controller", func() {
				err := kruiseapi.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				cloneSet := &kruisev1.CloneSet{}
				err = controllerutil.SetControllerReference(cloneSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(slices.Contains(constant.K8sKinds, podTopController.Kind)).To(BeFalse())
			})

			It("Pod with ReplicaSet controller", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				replicaSet := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = fakeClient.Create(ctx, replicaSet)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(replicaSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindReplicaSet))
			})

			It("Failed to fetch ReplicaSet controller of Pod", func() {
				methodReturn := gomonkey.ApplyMethodReturn(fakeClient, "Get", constant.ErrUnknown)
				defer methodReturn.Reset()

				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				replicaSet := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = fakeClient.Create(ctx, replicaSet)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(replicaSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				_, err = podManager.GetPodTopController(ctx, podT)
				Expect(err).To(HaveOccurred())
			})

			It("Pod with Deployment controller", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				replicaSet := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}

				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}

				err = controllerutil.SetControllerReference(deployment, replicaSet, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(replicaSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, replicaSet)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, deployment)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindDeployment))
			})

			It("Failed to fetch Deployment controller of Pod", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				replicaSet := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}

				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}

				err = controllerutil.SetControllerReference(deployment, replicaSet, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(replicaSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, replicaSet)
				Expect(err).NotTo(HaveOccurred())

				_, err = podManager.GetPodTopController(ctx, podT)
				Expect(err).To(HaveOccurred())
			})

			It("Third-party controller controls ReplicaSet", func() {
				err := kruiseapi.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())
				err = appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				replicaSet := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				cloneSet := &kruisev1.CloneSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(replicaSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(cloneSet, replicaSet, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, replicaSet)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, cloneSet)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindReplicaSet))
			})

			It("Pod with Job controller", func() {
				err := batchv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = fakeClient.Create(ctx, job)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(job, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindJob))
			})

			It("Failed to fetch Job controller of Pod", func() {
				err := batchv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(job, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				_, err = podManager.GetPodTopController(ctx, podT)
				Expect(err).To(HaveOccurred())
			})

			It("Third-party controller controls Job", func() {
				err := kruiseapi.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())
				err = batchv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				cloneSet := &kruisev1.CloneSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(job, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(cloneSet, job, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, job)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, cloneSet)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindJob))
			})

			It("Pod with CronJob controller", func() {
				err := batchv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}

				cronJob := &batchv1.CronJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(cronJob, job, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(job, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, cronJob)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, job)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindCronJob))
			})

			It("Failed to fetch CronJob controller of Pod", func() {
				err := batchv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				job := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}

				cronJob := &batchv1.CronJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(cronJob, job, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(job, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, job)
				Expect(err).NotTo(HaveOccurred())

				_, err = podManager.GetPodTopController(ctx, podT)
				Expect(err).To(HaveOccurred())
			})

			It("Pod with DaemonSet controller", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				daemonSet := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = fakeClient.Create(ctx, daemonSet)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(daemonSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindDaemonSet))
			})

			It("Failed to fetch DaemonSet controller of Pod", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				daemonSet := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(daemonSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				_, err = podManager.GetPodTopController(ctx, podT)
				Expect(err).To(HaveOccurred())
			})

			It("Pod with StatefulSet controller", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				statefulSet := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = fakeClient.Create(ctx, statefulSet)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(statefulSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				podTopController, err := podManager.GetPodTopController(ctx, podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(podTopController.Kind).Should(Equal(constant.KindStatefulSet))
			})

			It("Failed to fetch StatefulSet controller of Pod", func() {
				err := appsv1.AddToScheme(scheme)
				Expect(err).NotTo(HaveOccurred())

				statefulSet := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
					},
				}
				err = controllerutil.SetControllerReference(statefulSet, podT, scheme)
				Expect(err).NotTo(HaveOccurred())

				_, err = podManager.GetPodTopController(ctx, podT)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
