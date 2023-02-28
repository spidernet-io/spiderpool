// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
)

var _ = Describe("AppController", Label("unitest"), func() {
	var controller *subnetController
	var err error

	// apps
	var count uint64

	BeforeEach(func() {
		// new appController
		controller, err = newController()
		Expect(err).NotTo(HaveOccurred())
		Expect(controller).NotTo(BeNil())

		// mock
		mockLeaderElector.EXPECT().IsElected().Return(true).AnyTimes()

		// patches
		patches := gomonkey.ApplyFuncReturn(cache.WaitForNamedCacheSync, true)
		DeferCleanup(patches.Reset)

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)

		// fake client
		clientSet := fake.NewSimpleClientset()

		err := controller.SetupInformer(ctx, clientSet, mockLeaderElector)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			g.Expect(controller.SubnetAppController.DeploymentInformer.GetIndexer()).NotTo(BeNil())
		})

	})

	It("nil controllerleader", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		clientSet := fake.NewSimpleClientset()

		err := controller.SetupInformer(ctx, clientSet, nil)
		Expect(err).To(HaveOccurred())
	})

	It("unrecognized application", func() {
		unrecognizedObj := struct{}{}
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
		err = addOrUpdateHandler(ctx, nil, unrecognizedObj)
		Expect(err).To(HaveOccurred())
	})

	Context("deployment", func() {
		atomic.AddUint64(&count, 1)
		deployName := fmt.Sprintf("deploy-%v", count)
		deployT := &appv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deployName,
			},
			Spec: appv1.DeploymentSpec{},
		}
		deployNew := &appv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deployName,
			},
			Spec: appv1.DeploymentSpec{
				Replicas: pointer.Int32(3),
			},
		}

		// ctx
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		It("add deployment", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, deployT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("add hostnetwork deployment", func() {
			deployT.Spec.Template.Spec.HostNetwork = true
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, deployT)
			Expect(err).NotTo(HaveOccurred())
		})

		It("not top controller", func() {
			deployT.SetOwnerReferences([]metav1.OwnerReference{{Kind: "test-kind", Name: "test-name", Controller: pointer.Bool(true), APIVersion: "test-apiversion", UID: types.UID("xxx"), BlockOwnerDeletion: pointer.Bool(true)}})
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, deployT)
			Expect(err).NotTo(HaveOccurred())
		})
		//It("failed to GetSubnetAnnoConfig", func() {
		//	e := fmt.Errorf("bad")
		//	bach := gomonkey.ApplyFuncReturn(controllers.GetSubnetAnnoConfig, nil, e)
		//	addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
		//	err = addOrUpdateHandler(ctx, nil, deployT)
		//	Expect(err).To(HaveOccurred())
		//	defer bach.Reset()
		//})
		It("update deployment", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, deployT, deployNew)
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete deployment", func() {
			deleteHandler := controller.ControllerDeleteHandler()
			err = deleteHandler(ctx, deployT)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("cronJob", func() {
		atomic.AddUint64(&count, 1)
		cronJobName := fmt.Sprintf("cronJob-%v", count)
		cronJobT := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: cronJobName,
			},
			Spec: batchv1.CronJobSpec{},
		}
		cronJobNew := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name: cronJobName,
			},
			Spec: batchv1.CronJobSpec{},
		}

		// ctx
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		It("add cronJob", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, cronJobT)
			Expect(err).NotTo(HaveOccurred())
		})

		It("add hostnetwork cronJob", func() {
			cronJobT.Spec.JobTemplate.Spec.Template.Spec.HostNetwork = true
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, cronJobT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("update cronJob", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, cronJobT, cronJobNew)
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete cronJob", func() {
			deleteHandler := controller.ControllerDeleteHandler()
			err = deleteHandler(ctx, cronJobT)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("job", func() {
		atomic.AddUint64(&count, 1)
		jobName := fmt.Sprintf("job-%v", count)
		jobT := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName,
			},
			Spec: batchv1.JobSpec{},
		}
		jobNew := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName,
			},
			Spec: batchv1.JobSpec{},
		}

		// ctx
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		It("add job", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, jobT)
			Expect(err).NotTo(HaveOccurred())
		})

		It("add hostnetwork job", func() {
			jobT.Spec.Template.Spec.HostNetwork = true
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, jobT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("update job", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, jobT, jobNew)
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete job", func() {
			deleteHandler := controller.ControllerDeleteHandler()
			err = deleteHandler(ctx, jobT)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("daemonSet", func() {
		atomic.AddUint64(&count, 1)
		daemonSetName := fmt.Sprintf("deploy-%v", count)
		daemonSetT := &appv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: daemonSetName,
			},
			Spec: appv1.DaemonSetSpec{},
		}
		daemonSetNew := &appv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: daemonSetName,
			},
			Spec: appv1.DaemonSetSpec{},
		}

		// ctx
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		It("add daemonSet", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, daemonSetT)
			Expect(err).NotTo(HaveOccurred())
		})

		It("add hostnetwork daemonset", func() {
			daemonSetT.Spec.Template.Spec.HostNetwork = true
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, daemonSetT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("update daemonSet", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, daemonSetT, daemonSetNew)
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete daemonSet", func() {
			deleteHandler := controller.ControllerDeleteHandler()
			err = deleteHandler(ctx, daemonSetT)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("replicaSet", func() {
		atomic.AddUint64(&count, 1)
		replicaSetName := fmt.Sprintf("replicaset-%v", count)
		replicaSetT := &appv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: replicaSetName,
			},
			Spec: appv1.ReplicaSetSpec{},
		}
		replicaSetNew := &appv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: replicaSetName,
			},
			Spec: appv1.ReplicaSetSpec{},
		}

		// ctx
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		It("add replicaSet", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, replicaSetT)
			Expect(err).NotTo(HaveOccurred())
		})

		It("add hostnetwork replicaSet", func() {
			replicaSetT.Spec.Template.Spec.HostNetwork = true
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, replicaSetT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("update replicaSet", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, replicaSetT, replicaSetNew)
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete replicaSet", func() {
			deleteHandler := controller.ControllerDeleteHandler()
			err = deleteHandler(ctx, replicaSetT)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("statefulSet", func() {
		atomic.AddUint64(&count, 1)
		statefulSetName := fmt.Sprintf("statefulSet-%v", count)
		statefulSetT := &appv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: statefulSetName,
			},
			Spec: appv1.StatefulSetSpec{},
		}
		statefulSetNew := &appv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: statefulSetName,
			},
			Spec: appv1.ReplicaSetSpec{},
		}

		// ctx
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		It("add statefulSet", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, statefulSetT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("add hostnetwork statefulSet", func() {
			statefulSetT.Spec.Template.Spec.HostNetwork = true
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, nil, statefulSetT)
			Expect(err).NotTo(HaveOccurred())
		})
		It("update statefulSet", func() {
			addOrUpdateHandler := controller.ControllerAddOrUpdateHandler()
			err = addOrUpdateHandler(ctx, statefulSetT, statefulSetNew)
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete statefulSet", func() {
			deleteHandler := controller.ControllerDeleteHandler()
			err = deleteHandler(ctx, statefulSetT)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

type subnetController struct {
	*subnetmanager.SubnetAppController
}

func newController() (*subnetController, error) {
	subnetAppController, err := subnetmanager.NewSubnetAppController(fakeClient, mockSubnetMgr, subnetmanager.SubnetAppControllerConfig{})
	if nil != err {
		return nil, err
	}

	return &subnetController{
		SubnetAppController: subnetAppController,
	}, nil
}
