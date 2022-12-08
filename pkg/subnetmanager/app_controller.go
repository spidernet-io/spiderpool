// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (sm *subnetManager) SetupControllers(client kubernetes.Interface) error {
	if sm.leader == nil {
		return fmt.Errorf("failed to start applications controller, the subnet manager's leader is empty")
	}

	logger.Info("try to register applications controller")
	go func() {
		for {
			if !sm.leader.IsElected() {
				time.Sleep(sm.config.LeaderRetryElectGap)
				continue
			}

			// stopper lifecycle is same with applications Informer
			stopper := make(chan struct{})

			go func() {
				for {
					if !sm.leader.IsElected() {
						logger.Error("leader lost! stop application controllers!")
						close(stopper)
						return
					}

					time.Sleep(sm.config.LeaderRetryElectGap)
				}
			}()

			logger.Info("create applications informer")
			kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
			c, err := newAppController(sm, kubeInformerFactory)
			if nil != err {
				logger.Sugar().Errorf("failed to new application controller, error: %v", err)
				return
			}
			kubeInformerFactory.Start(stopper)

			err = c.Run(sm.config.Workers, stopper)
			if nil != err {
				logger.Sugar().Errorf("failed to run application controller, error: %v", err)
			}

			logger.Error("applications controller broken")
		}
	}()

	return nil
}

// ControllerAddOrUpdateHandler serves for kubernetes original controller applications(such as: Deployment,ReplicaSet,Job...),
// to create a new IPPool or scale the IPPool
func (c *appController) ControllerAddOrUpdateHandler() controllers.AppInformersAddOrUpdateFunc {
	return func(ctx context.Context, oldObj, newObj interface{}) error {
		log := logutils.FromContext(ctx)

		var err error
		var oldSubnetConfig, newSubnetConfig *controllers.PodSubnetAnnoConfig
		var appKind string
		var app metav1.Object
		var oldAppReplicas, newAppReplicas int

		switch newObject := newObj.(type) {
		case *appsv1.Deployment:
			appKind = constant.OwnerDeployment
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Debug("app will use default IPAM mode with no subnet annotation")

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
					err = c.tryToCleanUpLegacyIPPools(ctx, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldDeployment := oldObj.(*appsv1.Deployment)
				oldAppReplicas = controllers.GetAppReplicas(oldDeployment.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldDeployment.Spec.Template.Annotations)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *appsv1.ReplicaSet:
			appKind = constant.OwnerReplicaSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Debug("app will use default IPAM mode with no subnet annotation")

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
					err = c.tryToCleanUpLegacyIPPools(ctx, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldReplicaSet := oldObj.(*appsv1.ReplicaSet)
				oldAppReplicas = controllers.GetAppReplicas(oldReplicaSet.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldReplicaSet.Spec.Template.Annotations)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *appsv1.StatefulSet:
			appKind = constant.OwnerStatefulSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Debug("app will use default IPAM mode with no subnet annotation")

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
					err = c.tryToCleanUpLegacyIPPools(ctx, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldStatefulSet := oldObj.(*appsv1.StatefulSet)
				oldAppReplicas = controllers.GetAppReplicas(oldStatefulSet.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldStatefulSet.Spec.Template.Annotations)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *batchv1.Job:
			appKind = constant.OwnerJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.CalculateJobPodNum(newObject.Spec.Parallelism, newObject.Spec.Completions)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Debug("app will use default IPAM mode with no subnet annotation")

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
					err = c.tryToCleanUpLegacyIPPools(ctx, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldJob := oldObj.(*batchv1.Job)
				oldAppReplicas = controllers.CalculateJobPodNum(oldJob.Spec.Parallelism, oldJob.Spec.Completions)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldJob.Spec.Template.Annotations)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *batchv1.CronJob:
			appKind = constant.OwnerCronJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.JobTemplate.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.CalculateJobPodNum(newObject.Spec.JobTemplate.Spec.Parallelism, newObject.Spec.JobTemplate.Spec.Completions)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.JobTemplate.Spec.Template.Annotations)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Debug("app will use default IPAM mode with no subnet annotation")

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
					err = c.tryToCleanUpLegacyIPPools(ctx, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldCronJob := oldObj.(*batchv1.CronJob)
				oldAppReplicas = controllers.CalculateJobPodNum(oldCronJob.Spec.JobTemplate.Spec.Parallelism, oldCronJob.Spec.JobTemplate.Spec.Completions)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldCronJob.Spec.JobTemplate.Spec.Template.Annotations)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *appsv1.DaemonSet:
			appKind = constant.OwnerDaemonSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = int(newObject.Status.DesiredNumberScheduled)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Debug("app will use default IPAM mode with no subnet annotation")

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
					err = c.tryToCleanUpLegacyIPPools(ctx, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldDaemonSet := oldObj.(*appsv1.DaemonSet)
				oldAppReplicas = int(oldDaemonSet.Status.DesiredNumberScheduled)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldDaemonSet.Spec.Template.Annotations)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		default:
			return fmt.Errorf("unrecognized application: %+v", newObj)
		}

		ctx = logutils.IntoContext(ctx, log)
		// check the difference between the two object and choose to reconcile or not
		if c.hasSubnetConfigChanged(ctx, oldSubnetConfig, newSubnetConfig, oldAppReplicas, newAppReplicas, app) {
			c.enqueueApp(ctx, app, appKind)
		}

		return nil
	}
}

type appController struct {
	subnetMgr *subnetManager

	deploymentsLister  appslisters.DeploymentLister
	deploymentInformer cache.SharedIndexInformer

	replicaSetLister   appslisters.ReplicaSetLister
	replicaSetInformer cache.SharedIndexInformer

	statefulSetLister   appslisters.StatefulSetLister
	statefulSetInformer cache.SharedIndexInformer

	daemonSetLister   appslisters.DaemonSetLister
	daemonSetInformer cache.SharedIndexInformer

	jobLister   batchlisters.JobLister
	jobInformer cache.SharedIndexInformer

	cronJobLister   batchlisters.CronJobLister
	cronJobInformer cache.SharedIndexInformer
}

func newAppController(subnetMgr *subnetManager, factory kubeinformers.SharedInformerFactory) (*appController, error) {
	// Once we lost the leader but get leader later, we have to use a new workqueue.
	// Because the former workqueue was already shut down and wouldn't be re-start forever.
	subnetMgr.workQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Application-Controllers")

	c := &appController{
		subnetMgr: subnetMgr,

		deploymentsLister:  factory.Apps().V1().Deployments().Lister(),
		deploymentInformer: factory.Apps().V1().Deployments().Informer(),

		replicaSetLister:   factory.Apps().V1().ReplicaSets().Lister(),
		replicaSetInformer: factory.Apps().V1().ReplicaSets().Informer(),

		statefulSetLister:   factory.Apps().V1().StatefulSets().Lister(),
		statefulSetInformer: factory.Apps().V1().StatefulSets().Informer(),

		daemonSetLister:   factory.Apps().V1().DaemonSets().Lister(),
		daemonSetInformer: factory.Apps().V1().DaemonSets().Informer(),

		jobLister:   factory.Batch().V1().Jobs().Lister(),
		jobInformer: factory.Batch().V1().Jobs().Informer(),

		cronJobLister:   factory.Batch().V1().CronJobs().Lister(),
		cronJobInformer: factory.Batch().V1().CronJobs().Informer(),
	}

	subnetController, err := controllers.NewSubnetController(c.ControllerAddOrUpdateHandler(), c.ControllerDeleteHandler(), logger.Named("Controllers"))
	if nil != err {
		return nil, err
	}

	subnetController.AddDeploymentHandler(c.deploymentInformer)
	subnetController.AddReplicaSetHandler(c.replicaSetInformer)
	subnetController.AddDaemonSetHandler(c.daemonSetInformer)
	subnetController.AddStatefulSetHandler(c.statefulSetInformer)
	subnetController.AddJobController(c.jobInformer)
	subnetController.AddCronJobHandler(c.cronJobInformer)

	return c, nil
}

// appWorkQueueKey involves application object meta namespaceKey and application kind
type appWorkQueueKey struct {
	MetaNamespaceKey string
	AppKind          string
}

// enqueueApp will insert application custom appWorkQueueKey to the workQueue
func (c *appController) enqueueApp(ctx context.Context, obj interface{}, appKind string) {
	log := logutils.FromContext(ctx)

	// object meta key: 'namespace/name'
	metaKey, err := cache.MetaNamespaceKeyFunc(obj)
	if nil != err {
		log.Sugar().Errorf("failed to API object '%+v' meta key", obj)
		return
	}

	appKey := appWorkQueueKey{
		MetaNamespaceKey: metaKey,
		AppKind:          appKind,
	}

	// validate workqueue capacity
	maxQueueLength := c.subnetMgr.config.MaxWorkqueueLength
	if c.subnetMgr.workQueue.Len() >= maxQueueLength {
		log.Sugar().Errorf("The application controller workqueue is out of capacity, discard enqueue '%v'", appKey)
		return
	}

	c.subnetMgr.workQueue.Add(appKey)
	log.Sugar().Debugf("added '%v' to application controller workequeue", appKey)
}

func (c *appController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.subnetMgr.workQueue.ShutDown()

	logger.Debug("Waiting for application informers caches to sync")
	ok := cache.WaitForCacheSync(stopCh,
		c.deploymentInformer.HasSynced,
		c.replicaSetInformer.HasSynced,
		c.daemonSetInformer.HasSynced,
		c.statefulSetInformer.HasSynced,
		c.jobInformer.HasSynced,
		c.cronJobInformer.HasSynced,
	)
	if !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		logger.Sugar().Debugf("Starting application controller processing worker '%d'", i)
		go wait.Until(c.runWorker, 500*time.Millisecond, stopCh)
	}

	logger.Info("application controller workers started")

	<-stopCh
	logger.Error("Shutting down application controller workers")
	return nil
}

func (c *appController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *appController) processNextWorkItem() bool {
	obj, shutdown := c.subnetMgr.workQueue.Get()
	if shutdown {
		logger.Error("application controller workqueue is already shutdown!")
		return false
	}

	var log *zap.Logger

	process := func(obj interface{}) error {
		defer c.subnetMgr.workQueue.Done(obj)
		key, ok := obj.(appWorkQueueKey)
		if !ok {
			c.subnetMgr.workQueue.Forget(obj)
			return fmt.Errorf("expected appWorkQueueKey in workQueue but got '%+v'", obj)
		}

		log = logger.With(
			zap.String("OPERATION", "process_app_items"),
			zap.String("Application", fmt.Sprintf("%s/%s", key.AppKind, key.MetaNamespaceKey)),
		)

		err := c.syncHandler(key, log)
		if nil != err {
			// discard wrong input items
			if errors.Is(err, constant.ErrWrongInput) {
				c.subnetMgr.workQueue.Forget(obj)
				return err
			}

			// requeue the conflict items
			if apierrors.IsConflict(err) {
				c.subnetMgr.workQueue.AddRateLimited(obj)
				log.Sugar().Warnf("encountered app controller syncHandler conflict '%v', retrying...", err)
				return nil
			}

			// if we set nonnegative number for the requeue delay duration, we will requeue it. otherwise we will discard it.
			if c.subnetMgr.config.RequeueDelayDuration >= 0 {
				if c.subnetMgr.workQueue.NumRequeues(obj) < c.subnetMgr.config.MaxWorkqueueLength {
					log.Sugar().Errorf("encountered app controller syncHandler error '%v', requeue it after '%v'", err, c.subnetMgr.config.RequeueDelayDuration)
					c.subnetMgr.workQueue.AddAfter(obj, c.subnetMgr.config.RequeueDelayDuration)
					return nil
				}

				log.Warn("out of work queue max retries, drop it")
			}

			c.subnetMgr.workQueue.Forget(obj)
			return err
		}

		c.subnetMgr.workQueue.Forget(obj)
		return nil
	}

	if err := process(obj); nil != err {
		log.Error(err.Error())
	}

	return true
}

// syncHandler retrieves appWorkQueueKey from workQueue and try to create the auto-created IPPool or mark the IPPool status.AutoDesiredIPCount
func (c *appController) syncHandler(appKey appWorkQueueKey, log *zap.Logger) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(appKey.MetaNamespaceKey)
	if nil != err {
		return fmt.Errorf("%w: failed to split meta namespace key '%+v', error: %v", constant.ErrWrongInput, appKey.MetaNamespaceKey, err)
	}

	var app metav1.Object
	var subnetConfig *controllers.PodSubnetAnnoConfig
	var podAnno, podSelector map[string]string
	var appReplicas int

	switch appKey.AppKind {
	case constant.OwnerDeployment:
		deployment, err := c.deploymentsLister.Deployments(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = deployment.Spec.Template.Annotations
		podSelector = deployment.Spec.Selector.MatchLabels
		appReplicas = controllers.GetAppReplicas(deployment.Spec.Replicas)
		app = deployment.DeepCopy()

	case constant.OwnerReplicaSet:
		replicaSet, err := c.replicaSetLister.ReplicaSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = replicaSet.Spec.Template.Annotations
		podSelector = replicaSet.Spec.Selector.MatchLabels
		appReplicas = controllers.GetAppReplicas(replicaSet.Spec.Replicas)
		app = replicaSet.DeepCopy()

	case constant.OwnerDaemonSet:
		daemonSet, err := c.daemonSetLister.DaemonSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = daemonSet.Spec.Template.Annotations
		podSelector = daemonSet.Spec.Selector.MatchLabels
		appReplicas = int(daemonSet.Status.DesiredNumberScheduled)
		app = daemonSet.DeepCopy()

	case constant.OwnerStatefulSet:
		statefulSet, err := c.statefulSetLister.StatefulSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = statefulSet.Spec.Template.Annotations
		podSelector = statefulSet.Spec.Selector.MatchLabels
		appReplicas = controllers.GetAppReplicas(statefulSet.Spec.Replicas)
		app = statefulSet.DeepCopy()

	case constant.OwnerJob:
		job, err := c.jobLister.Jobs(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = job.Spec.Template.Annotations
		podSelector = job.Spec.Selector.MatchLabels
		appReplicas = controllers.CalculateJobPodNum(job.Spec.Parallelism, job.Spec.Completions)
		app = job.DeepCopy()

	case constant.OwnerCronJob:
		cronJob, err := c.cronJobLister.CronJobs(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = cronJob.Spec.JobTemplate.Spec.Template.Annotations
		podSelector = cronJob.Spec.JobTemplate.Spec.Selector.MatchLabels
		appReplicas = controllers.CalculateJobPodNum(cronJob.Spec.JobTemplate.Spec.Parallelism, cronJob.Spec.JobTemplate.Spec.Completions)
		app = cronJob.DeepCopy()

	default:
		return fmt.Errorf("%w: unexpected appWorkQueueKey in workQueue '%+v'", constant.ErrWrongInput, appKey)
	}

	subnetConfig, err = controllers.GetSubnetAnnoConfig(podAnno)
	if nil != err {
		return fmt.Errorf("%w: failed to get pod annotation subnet config, error: %v", constant.ErrWrongInput, err)
	}

	log.Debug("Going to create IPPool or mark IPPool desired IP number")
	err = c.createOrMarkIPPool(logutils.IntoContext(context.TODO(), log), *subnetConfig, appKey.AppKind, app, podSelector, appReplicas)
	if nil != err {
		return fmt.Errorf("failed to create or scale IPPool, error: %w", err)
	}

	return nil
}

// createOrMarkIPPool try to create an IPPool or mark IPPool desired IP number with the give SpiderSubnet configuration
func (c *appController) createOrMarkIPPool(ctx context.Context, podSubnetConfig controllers.PodSubnetAnnoConfig, appKind string, app metav1.Object,
	podSelector map[string]string, appReplicas int) error {
	if c.subnetMgr.config.EnableIPv4 && len(podSubnetConfig.SubnetName.IPv4) == 0 {
		return fmt.Errorf("IPv4 SpiderSubnet not specified when configuration enableIPv4 is on")
	}
	if c.subnetMgr.config.EnableIPv6 && len(podSubnetConfig.SubnetName.IPv6) == 0 {
		return fmt.Errorf("IPv6 SpiderSubnet not specified when configuration enableIPv6 is on")
	}

	log := logutils.FromContext(ctx)

	// retrieve application pools
	f := func(ctx context.Context, poolList *spiderpoolv1.SpiderIPPoolList, subnetName string, ipVersion types.IPVersion) (err error) {
		var ipNum int
		if podSubnetConfig.FlexibleIPNum != nil {
			ipNum = appReplicas + *(podSubnetConfig.FlexibleIPNum)
		} else {
			ipNum = podSubnetConfig.AssignIPNum
		}

		// verify whether the pool IPs need to be expanded or not
		if poolList == nil || len(poolList.Items) == 0 {
			log.Sugar().Debugf("there's no 'IPv%d' IPPoolList retrieved from SpiderSubent '%s'", ipVersion, subnetName)
			// create an empty IPPool and mark the desired IP number when the subnet name was specified,
			// and the IPPool informer will implement the scale action
			err = c.subnetMgr.AllocateEmptyIPPool(ctx, subnetName, appKind, app, podSelector, ipNum, ipVersion, podSubnetConfig.ReclaimIPPool)
		} else if len(poolList.Items) == 1 {
			pool := poolList.Items[0]
			log.Sugar().Debugf("found SpiderSubnet '%s' IPPool '%s' and check it whether need to be scaled", subnetName, pool.Name)
			err = c.subnetMgr.CheckScaleIPPool(ctx, &pool, subnetName, ipNum)
		} else {
			err = fmt.Errorf("%w: it's invalid that SpiderSubnet '%s' owns multiple IPPools '%v' for one specify application", constant.ErrWrongInput, subnetName, poolList.Items)
		}

		return
	}

	var errV4, errV6 error
	var wg sync.WaitGroup
	if c.subnetMgr.config.EnableIPv4 && len(podSubnetConfig.SubnetName.IPv4) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var v4PoolList *spiderpoolv1.SpiderIPPoolList
			v4PoolList, errV4 = c.subnetMgr.ipPoolManager.ListIPPools(ctx, client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
				constant.LabelIPPoolOwnerSpiderSubnet:   podSubnetConfig.SubnetName.IPv4[0],
				constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV4,
			})
			if nil != errV4 {
				return
			}

			errV4 = f(ctx, v4PoolList, podSubnetConfig.SubnetName.IPv4[0], constant.IPv4)
		}()
	}

	if c.subnetMgr.config.EnableIPv6 && len(podSubnetConfig.SubnetName.IPv6) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var v6PoolList *spiderpoolv1.SpiderIPPoolList
			v6PoolList, errV6 = c.subnetMgr.ipPoolManager.ListIPPools(ctx, client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
				constant.LabelIPPoolOwnerSpiderSubnet:   podSubnetConfig.SubnetName.IPv6[0],
				constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV6,
			})
			if nil != errV6 {
				return
			}

			errV6 = f(ctx, v6PoolList, podSubnetConfig.SubnetName.IPv6[0], constant.IPv6)
		}()
	}

	wg.Wait()

	if errV4 != nil || errV6 != nil {
		// NewAggregate will check each the given error slice elements whether is nil or not
		return multierr.Append(errV4, errV6)
	}

	return nil
}

// hasSubnetConfigChanged checks whether application subnet configuration changed and the application replicas changed or not.
// The second parameter newSubnetConfig must not be nil.
func (c *appController) hasSubnetConfigChanged(ctx context.Context, oldSubnetConfig, newSubnetConfig *controllers.PodSubnetAnnoConfig,
	oldAppReplicas, newAppReplicas int, app metav1.Object) bool {
	// go to reconcile directly with new application
	if oldSubnetConfig == nil {
		return true
	}

	log := logutils.FromContext(ctx)

	var isChanged bool
	if reflect.DeepEqual(oldSubnetConfig, newSubnetConfig) {
		if oldAppReplicas != newAppReplicas {
			isChanged = true
			log.Sugar().Debugf("new application changed its replicas from '%d' to '%d'", oldAppReplicas, newAppReplicas)
		}
	} else {
		isChanged = true
		log.Sugar().Debugf("new application changed SpiderSubnet configuration, the old one is '%v' and the new one '%v'", oldSubnetConfig, newSubnetConfig)

		if len(oldSubnetConfig.SubnetName.IPv4) != 0 && oldSubnetConfig.SubnetName.IPv4[0] != newSubnetConfig.SubnetName.IPv4[0] {
			log.Sugar().Warnf("change SpiderSubnet IPv4 from '%s' to '%s'", oldSubnetConfig.SubnetName.IPv4[0], newSubnetConfig.SubnetName.IPv4[0])

			// we should clean up the legacy IPPools once changed the SpiderSubnet
			if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
				if err := c.tryToCleanUpLegacyIPPools(ctx, app, client.MatchingLabels{
					constant.LabelIPPoolOwnerSpiderSubnet: oldSubnetConfig.SubnetName.IPv4[0],
				}); err != nil {
					log.Sugar().Errorf("failed to clean up SpiderSubnet '%s' legacy V4 IPPools, error: %v", oldSubnetConfig.SubnetName.IPv4[0], err)
				}
			}
		}

		if len(oldSubnetConfig.SubnetName.IPv6) != 0 && oldSubnetConfig.SubnetName.IPv6[0] != newSubnetConfig.SubnetName.IPv6[0] {
			log.Sugar().Warnf("change SpiderSubnet IPv6 from '%s' to '%s'", oldSubnetConfig.SubnetName.IPv6[0], newSubnetConfig.SubnetName.IPv6[0])

			// we should clean up the legacy IPPools once changed the SpiderSubnet
			if c.subnetMgr.config.EnableSubnetDeleteStaleIPPool {
				if err := c.tryToCleanUpLegacyIPPools(ctx, app, client.MatchingLabels{
					constant.LabelIPPoolOwnerSpiderSubnet: oldSubnetConfig.SubnetName.IPv6[0],
				}); err != nil {
					log.Sugar().Errorf("failed to clean up SpiderSubnet '%s' legacy V6 IPPools, error: %v", oldSubnetConfig.SubnetName.IPv6[0], err)
				}
			}
		}
	}

	return isChanged
}

// ControllerDeleteHandler will return a function that clean up the application SpiderSubnet legacies (such as: the before created IPPools)
func (c *appController) ControllerDeleteHandler() controllers.APPInformersDelFunc {
	return func(ctx context.Context, obj interface{}) error {
		log := logutils.FromContext(ctx)

		var appKind string
		var app metav1.Object

		switch object := obj.(type) {
		case *appsv1.Deployment:
			appKind = constant.OwnerDeployment
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *appsv1.ReplicaSet:
			appKind = constant.OwnerReplicaSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *appsv1.StatefulSet:
			appKind = constant.OwnerStatefulSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *batchv1.Job:
			appKind = constant.OwnerJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *batchv1.CronJob:
			appKind = constant.OwnerCronJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *appsv1.DaemonSet:
			appKind = constant.OwnerDaemonSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}

			app = object

		default:
			return fmt.Errorf("unrecognized application: %+v", obj)
		}

		// clean up all legacy IPPools that matched with the application UID
		ctx = logutils.IntoContext(ctx, log)
		go func() {
			err := c.tryToCleanUpLegacyIPPools(ctx, app)
			if nil != err {
				log.Sugar().Errorf("failed to clean up legacy IPPool, error: %v", err)
			}
		}()

		return nil
	}
}

func (c *appController) tryToCleanUpLegacyIPPools(ctx context.Context, app metav1.Object, labels ...client.MatchingLabels) error {
	log := logutils.FromContext(ctx)

	matchLabel := client.MatchingLabels{
		constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
		constant.LabelIPPoolReclaimIPPool:       constant.True,
	}

	for _, label := range labels {
		for k, v := range label {
			matchLabel[k] = v
		}
	}

	poolList, err := c.subnetMgr.ipPoolManager.ListIPPools(ctx, matchLabel)
	if nil != err {
		return err
	}

	if poolList != nil && len(poolList.Items) != 0 {
		wg := new(sync.WaitGroup)

		deletePool := func(pool *spiderpoolv1.SpiderIPPool) {
			defer wg.Done()
			err := c.subnetMgr.ipPoolManager.DeleteIPPool(ctx, pool)
			if nil != err {
				log.Sugar().Errorf("failed to delete IPPool '%s', error: %v", pool.Name, err)
				return
			}

			log.Sugar().Infof("delete IPPool '%s' successfully", pool.Name)
		}

		for i := range poolList.Items {
			wg.Add(1)
			pool := poolList.Items[i]
			go deletePool(&pool)
		}

		wg.Wait()
	}

	return nil
}
