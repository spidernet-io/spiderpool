// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationcontroller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
)

var _ = Describe("AppController", Label("app_controller_test"), func() {
	var deployment1 *appsv1.Deployment
	var replicaSet1 *appsv1.ReplicaSet
	var daemonSet1 *appsv1.DaemonSet
	var statefulSet1 *appsv1.StatefulSet
	var job1 *batchv1.Job
	var cronJob1 *batchv1.CronJob

	BeforeEach(func() {
		deployment1 = &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindDeployment,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "ns1",
				UID:       types.UID("123"),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							constant.AnnoSpiderSubnet:             `{"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}`,
							constant.AnnoSpiderSubnetPoolIPNumber: "+1",
						},
					},
				},
			},
		}
		replicaSet1 = &appsv1.ReplicaSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindReplicaSet,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-replicaset",
				Namespace: "ns1",
				UID:       types.UID("123"),
			},
			Spec: appsv1.ReplicaSetSpec{
				Replicas: ptr.To(int32(1)),
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							constant.AnnoSpiderSubnets:            `[{"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}]`,
							constant.AnnoSpiderSubnetPoolIPNumber: "1",
						},
					},
				},
			},
		}
		daemonSet1 = &appsv1.DaemonSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindDaemonSet,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-daemonset",
				Namespace: "ns1",
				UID:       types.UID("123"),
			},
			Spec: appsv1.DaemonSetSpec{
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							constant.AnnoSpiderSubnet:             `{"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}`,
							constant.AnnoSpiderSubnetPoolIPNumber: "1",
						},
					},
				},
			},
		}
		statefulSet1 = &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindStatefulSet,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-statefulset",
				Namespace: "ns1",
				UID:       types.UID("123"),
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(1)),
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							constant.AnnoSpiderSubnet:             `{"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}`,
							constant.AnnoSpiderSubnetPoolIPNumber: "1",
						},
					},
				},
			},
		}
		job1 = &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				APIVersion: batchv1.SchemeGroupVersion.String(),
				Kind:       constant.KindJob,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-job",
				Namespace: "ns1",
				UID:       types.UID("123"),
			},
			Spec: batchv1.JobSpec{
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							constant.AnnoSpiderSubnet:             `{"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}`,
							constant.AnnoSpiderSubnetPoolIPNumber: "1",
						},
					},
				},
			},
		}
		cronJob1 = &batchv1.CronJob{
			TypeMeta: metav1.TypeMeta{
				APIVersion: batchv1.SchemeGroupVersion.String(),
				Kind:       constant.KindCronJob,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cronJob",
				Namespace: "ns1",
				UID:       types.UID("123"),
			},
			Spec: batchv1.CronJobSpec{
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									constant.AnnoSpiderSubnet:             `{"ipv4": ["subnet-demo-v4"], "ipv6": ["subnet-demo-v6"]}`,
									constant.AnnoSpiderSubnetPoolIPNumber: "1",
								},
							},
						},
					},
				},
			},
		}

	})

	Describe("run subnet app controller", func() {
		var control *subnetApplicationController
		var clientSet *fake.Clientset

		BeforeEach(func() {
			c, err := newController()
			Expect(err).NotTo(HaveOccurred())
			control = c

			clientSet = fake.NewSimpleClientset()

			factory := kubeinformers.NewSharedInformerFactory(clientSet, 0)
			err = control.addEventHandlers(factory)
			Expect(err).NotTo(HaveOccurred())

			control.deploymentStore = factory.Apps().V1().Deployments().Informer().GetStore()
			control.replicasSetStore = factory.Apps().V1().ReplicaSets().Informer().GetStore()
			control.daemonSetStore = factory.Apps().V1().DaemonSets().Informer().GetStore()
			control.statefulSetStore = factory.Apps().V1().StatefulSets().Informer().GetStore()
			control.jobStore = factory.Batch().V1().Jobs().Informer().GetStore()
			control.cronJobStore = factory.Batch().V1().CronJobs().Informer().GetStore()
		})

		Context("enqueue an Deployment", func() {
			It("enqueue an deployment that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.deploymentStore.Add(deployment1)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueApp(ctx, deployment1, constant.KindDeployment, deployment1.UID)
			})

			It("enqueue an deployment that informer didn't sync", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueApp(ctx, deployment1, constant.KindDeployment, deployment1.UID)
			})
		})

		Context("enqueue an ReplicaSet", func() {
			It("enqueue an replicaset that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.replicasSetStore.Add(replicaSet1)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueApp(ctx, replicaSet1, constant.KindReplicaSet, replicaSet1.UID)
			})

			It("enqueue an replicaset that informer didn't sync", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueApp(ctx, replicaSet1, constant.KindReplicaSet, replicaSet1.UID)
			})
		})

		Context("enqueue an DaemonSet", func() {
			It("enqueue an daemonset that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.daemonSetStore.Add(daemonSet1)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueApp(ctx, daemonSet1, constant.KindDaemonSet, daemonSet1.UID)
			})

			It("enqueue an daemonset that informer didn't sync", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueApp(ctx, daemonSet1, constant.KindDaemonSet, daemonSet1.UID)
			})
		})

		Context("enqueue an StatefulSet", func() {
			It("enqueue an statefulset that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.statefulSetStore.Add(statefulSet1)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueApp(ctx, statefulSet1, constant.KindStatefulSet, statefulSet1.UID)
			})

			It("enqueue an statefulset that informer didn't sync", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueApp(ctx, statefulSet1, constant.KindStatefulSet, statefulSet1.UID)
			})
		})

		Context("enqueue an job", func() {
			It("enqueue an job that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.jobStore.Add(job1)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueApp(ctx, job1, constant.KindJob, job1.UID)
			})

			It("enqueue an job that informer didn't sync", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueApp(ctx, job1, constant.KindJob, job1.UID)
			})
		})

		Context("enqueue an cronJob", func() {
			It("enqueue an cronJob that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.cronJobStore.Add(cronJob1)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueApp(ctx, cronJob1, constant.KindCronJob, cronJob1.UID)
			})

			It("enqueue an cronJob that informer didn't sync", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueApp(ctx, cronJob1, constant.KindCronJob, cronJob1.UID)
			})
		})
	})

	Describe("test application add or update event hook handler", func() {
		var reconcileFunc applicationinformers.AppInformersAddOrUpdateFunc
		var ctx context.Context
		cloneSet := &v1alpha1.CloneSet{}

		BeforeEach(func() {
			ctx = context.TODO()

			c, err := newController()
			Expect(err).NotTo(HaveOccurred())
			c.workQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "unit-test-workqueue")
			reconcileFunc = c.controllerAddOrUpdateHandler()
		})

		Context("deployment", func() {
			It("create host network deployment", func() {
				deployment1.Spec.Template.Spec.HostNetwork = true
				err := reconcileFunc(ctx, nil, deployment1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create default IPPool mode deployment", func() {
				deployment1.Spec.Template.Annotations = map[string]string{}
				err := reconcileFunc(ctx, nil, deployment1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create deployment with spider subnet annotation", func() {
				err := reconcileFunc(ctx, nil, deployment1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change deployment replicas with spider subnet annotation", func() {
				deployment2 := deployment1.DeepCopy()
				deployment2.Spec.Replicas = ptr.To(int32(2))
				err := reconcileFunc(ctx, deployment1, deployment2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change deployment with spider subnet annotation configuration", func() {
				deployment2 := deployment1.DeepCopy()
				deployment2.Spec.Template.Annotations[constant.AnnoSpiderSubnetPoolIPNumber] = "2"
				err := reconcileFunc(ctx, deployment1, deployment2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the deployment has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, deployment1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, deployment1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("replicaset", func() {
			It("create host network replicaset", func() {
				replicaSet1.Spec.Template.Spec.HostNetwork = true
				err := reconcileFunc(ctx, nil, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create default IPPool mode replicaset", func() {
				replicaSet1.Spec.Template.Annotations = map[string]string{}
				err := reconcileFunc(ctx, nil, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create replicaset with spider subnet annotation", func() {
				err := reconcileFunc(ctx, nil, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change replicaSet replicas with spider subnet annotation", func() {
				replicaSet2 := replicaSet1.DeepCopy()
				replicaSet2.Spec.Replicas = ptr.To(int32(2))
				err := reconcileFunc(ctx, replicaSet1, replicaSet2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change replicaSet with spider subnet annotation configuration", func() {
				replicaSet2 := replicaSet1.DeepCopy()
				replicaSet2.Spec.Template.Annotations[constant.AnnoSpiderSubnetPoolIPNumber] = "2"
				err := reconcileFunc(ctx, replicaSet1, replicaSet2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the replicaSet has third-party controller owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, replicaSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the replicaSet has deployment controller owner", func() {
				err := controllerutil.SetControllerReference(deployment1, replicaSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("daemonset", func() {
			It("create host network daemonset", func() {
				daemonSet1.Spec.Template.Spec.HostNetwork = true
				err := reconcileFunc(ctx, nil, daemonSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create default IPPool mode daemonset", func() {
				daemonSet1.Spec.Template.Annotations = map[string]string{}
				err := reconcileFunc(ctx, nil, daemonSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create daemonset with spider subnet annotation", func() {
				err := reconcileFunc(ctx, nil, daemonSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change daemonset replicas with spider subnet annotation", func() {
				daemonSet2 := daemonSet1.DeepCopy()
				daemonSet2.Status.DesiredNumberScheduled = int32(3)
				err := reconcileFunc(ctx, daemonSet1, daemonSet2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change daemonset with spider subnet annotation configuration", func() {
				daemonSet2 := daemonSet1.DeepCopy()
				daemonSet2.Spec.Template.Annotations[constant.AnnoSpiderSubnetPoolIPNumber] = "2"
				err := reconcileFunc(ctx, daemonSet1, daemonSet2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the daemonset has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, daemonSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, daemonSet1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("statefulset", func() {
			It("create host network statefulset", func() {
				statefulSet1.Spec.Template.Spec.HostNetwork = true
				err := reconcileFunc(ctx, nil, statefulSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create default IPPool mode statefulset", func() {
				statefulSet1.Spec.Template.Annotations = map[string]string{}
				err := reconcileFunc(ctx, nil, statefulSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create statefulset with spider subnet annotation", func() {
				err := reconcileFunc(ctx, nil, statefulSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change statefulset replicas with spider subnet annotation", func() {
				statefulSet2 := statefulSet1.DeepCopy()
				statefulSet2.Spec.Replicas = ptr.To(int32(2))
				err := reconcileFunc(ctx, statefulSet1, statefulSet2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change statefulset with spider subnet annotation configuration", func() {
				statefulSet2 := statefulSet1.DeepCopy()
				statefulSet2.Spec.Template.Annotations[constant.AnnoSpiderSubnetPoolIPNumber] = "2"
				err := reconcileFunc(ctx, statefulSet1, statefulSet2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the statefulset has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, statefulSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, statefulSet1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("job", func() {
			It("create host network job", func() {
				job1.Spec.Template.Spec.HostNetwork = true
				err := reconcileFunc(ctx, nil, job1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create default IPPool mode job", func() {
				job1.Spec.Template.Annotations = map[string]string{}
				err := reconcileFunc(ctx, nil, job1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create job with spider subnet annotation", func() {
				err := reconcileFunc(ctx, nil, job1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change job replicas with spider subnet annotation", func() {
				job2 := job1.DeepCopy()
				job2.Spec.Parallelism = ptr.To(int32(2))
				err := reconcileFunc(ctx, job1, job2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change job with spider subnet annotation configuration", func() {
				job2 := job1.DeepCopy()
				job2.Spec.Template.Annotations[constant.AnnoSpiderSubnetPoolIPNumber] = "2"
				err := reconcileFunc(ctx, job1, job2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the job has third-party controller owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, job1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, job1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the job has cronJob controller owner", func() {
				err := controllerutil.SetControllerReference(cronJob1, job1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, job1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("cronJob", func() {
			It("create host network cronJob", func() {
				cronJob1.Spec.JobTemplate.Spec.Template.Spec.HostNetwork = true
				err := reconcileFunc(ctx, nil, cronJob1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create default IPPool mode cronJob", func() {
				cronJob1.Spec.JobTemplate.Spec.Template.Annotations = map[string]string{}
				err := reconcileFunc(ctx, nil, cronJob1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("create cronJob with spider subnet annotation", func() {
				err := reconcileFunc(ctx, nil, cronJob1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change cronJob replicas with spider subnet annotation", func() {
				cronJob2 := cronJob1.DeepCopy()
				cronJob2.Spec.JobTemplate.Spec.Parallelism = ptr.To(int32(3))
				err := reconcileFunc(ctx, cronJob1, cronJob2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("change cronJob with spider subnet annotation configuration", func() {
				cronJob2 := cronJob1.DeepCopy()
				cronJob2.Spec.JobTemplate.Spec.Template.Annotations[constant.AnnoSpiderSubnetPoolIPNumber] = "2"
				err := reconcileFunc(ctx, cronJob1, cronJob2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the cronJob has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, cronJob1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = reconcileFunc(ctx, nil, cronJob1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("unrecognized controller", func() {
			It("do not support third-party controller", func() {
				err := reconcileFunc(ctx, nil, cloneSet)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("test application delete event hook handler", func() {
		var cleanupFunc applicationinformers.APPInformersDelFunc
		var ctx context.Context
		cloneSet := &v1alpha1.CloneSet{}

		BeforeEach(func() {
			ctx = context.TODO()

			c, err := newController()
			Expect(err).NotTo(HaveOccurred())
			c.workQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "unit-test-workqueue")
			cleanupFunc = c.controllerDeleteHandler()
		})

		Context("deployment", func() {
			It("delete deployment", func() {
				err := cleanupFunc(ctx, deployment1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the deployment has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, deployment1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = cleanupFunc(ctx, deployment1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("replicaset", func() {
			It("delete replicaset", func() {
				err := cleanupFunc(ctx, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the replicaset has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, replicaSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = cleanupFunc(ctx, replicaSet1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("daemonset", func() {
			It("delete daemonset", func() {
				err := cleanupFunc(ctx, daemonSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the deployment has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, daemonSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = cleanupFunc(ctx, daemonSet1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("statefulset", func() {
			It("delete statefulset", func() {
				err := cleanupFunc(ctx, statefulSet1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the statefulSet has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, statefulSet1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = cleanupFunc(ctx, statefulSet1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("job", func() {
			It("delete job", func() {
				err := cleanupFunc(ctx, job1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the job has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, job1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = cleanupFunc(ctx, job1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("cronJob", func() {
			It("delete cronJob", func() {
				err := cleanupFunc(ctx, cronJob1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("the cronJob has owner", func() {
				err := controllerutil.SetControllerReference(cloneSet, cronJob1, scheme)
				Expect(err).NotTo(HaveOccurred())
				err = cleanupFunc(ctx, cronJob1)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("unrecognized controller", func() {
			It("do not support third-party controller", func() {
				err := cleanupFunc(ctx, cloneSet)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("test deleteAutoPools", func() {
		var ctx context.Context
		var control *subnetApplicationController

		BeforeEach(func() {
			ctx = context.TODO()
			c, err := newController()
			Expect(err).NotTo(HaveOccurred())
			control = c
		})

		It("test deleteAutoPools with not exist SpiderIPPool", func() {
			patch := gomonkey.ApplyMethodReturn(control.client, "DeleteAllOf", constant.ErrUnknown)
			patch.ApplyFuncReturn(errors.IsNotFound, true)
			defer patch.Reset()

			err := control.deleteAutoPools(ctx, deployment1.UID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("test deleteAutoPools with unknown error", func() {
			patch := gomonkey.ApplyMethodReturn(control.client, "DeleteAllOf", constant.ErrUnknown)
			defer patch.Reset()

			err := control.deleteAutoPools(ctx, deployment1.UID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(constant.ErrUnknown))
		})

		It("test deleteAutoPools successfully", func() {
			patch := gomonkey.ApplyMethodReturn(control.client, "DeleteAllOf", nil)
			defer patch.Reset()

			err := control.deleteAutoPools(ctx, deployment1.UID)
			Expect(err).NotTo(HaveOccurred())
		})
	})

})
