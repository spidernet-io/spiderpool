// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationcontroller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8types "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var logger *zap.Logger

type SubnetAppController struct {
	client    client.Client
	apiReader client.Reader

	subnetMgr     subnetmanager.SubnetManager
	workQueue     workqueue.RateLimitingInterface
	appController *applicationinformers.Controller

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

	SubnetAppControllerConfig
}

type SubnetAppControllerConfig struct {
	EnableIPv4                    bool
	EnableIPv6                    bool
	AppControllerWorkers          int
	MaxWorkqueueLength            int
	WorkQueueMaxRetries           int
	WorkQueueRequeueDelayDuration time.Duration
	LeaderRetryElectGap           time.Duration
}

func NewSubnetAppController(client client.Client, apiReader client.Reader, subnetMgr subnetmanager.SubnetManager, subnetAppControllerConfig SubnetAppControllerConfig) (*SubnetAppController, error) {
	logger = logutils.Logger.Named("SpiderSubnet-Application-Controllers")

	c := &SubnetAppController{
		client:                    client,
		apiReader:                 apiReader,
		subnetMgr:                 subnetMgr,
		SubnetAppControllerConfig: subnetAppControllerConfig,
	}

	appController, err := applicationinformers.NewApplicationController(c.controllerAddOrUpdateHandler(), c.controllerDeleteHandler(), logger)
	if nil != err {
		return nil, err
	}
	c.appController = appController

	return c, nil
}

func (sac *SubnetAppController) SetupInformer(ctx context.Context, client kubernetes.Interface, leader election.SpiderLeaseElector) error {
	if leader == nil {
		return fmt.Errorf("failed to start SpiderSubnet App informer, controller leader must be specified")
	}

	logger.Info("try to register SpiderSubnet App informer")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !leader.IsElected() {
				time.Sleep(sac.LeaderRetryElectGap)
				continue
			}

			innerCtx, innerCancel := context.WithCancel(ctx)
			go func() {
				for {
					select {
					case <-innerCtx.Done():
						return
					default:
					}

					if !leader.IsElected() {
						logger.Warn("Leader lost, stop Subnet App informer")
						innerCancel()
						return
					}
					time.Sleep(sac.LeaderRetryElectGap)
				}
			}()

			logger.Info("create SpiderSubnet App informer")
			factory := kubeinformers.NewSharedInformerFactory(client, 0)
			err := sac.addEventHandlers(factory)
			if nil != err {
				logger.Error(err.Error())
				continue
			}

			factory.Start(innerCtx.Done())
			err = sac.Run(innerCtx.Done())
			if nil != err {
				logger.Sugar().Errorf("failed to run SpiderSubnet App controller, error: %v", err)
			}
			logger.Error("SpiderSubnet App informer broken")
		}
	}()

	return nil
}

func (sac *SubnetAppController) addEventHandlers(factory kubeinformers.SharedInformerFactory) error {
	sac.deploymentsLister = factory.Apps().V1().Deployments().Lister()
	sac.deploymentInformer = factory.Apps().V1().Deployments().Informer()
	err := sac.appController.AddDeploymentHandler(sac.deploymentInformer)
	if nil != err {
		return err
	}

	sac.replicaSetLister = factory.Apps().V1().ReplicaSets().Lister()
	sac.replicaSetInformer = factory.Apps().V1().ReplicaSets().Informer()
	err = sac.appController.AddReplicaSetHandler(sac.replicaSetInformer)
	if nil != err {
		return err
	}

	sac.statefulSetLister = factory.Apps().V1().StatefulSets().Lister()
	sac.statefulSetInformer = factory.Apps().V1().StatefulSets().Informer()
	err = sac.appController.AddStatefulSetHandler(sac.statefulSetInformer)
	if nil != err {
		return err
	}

	sac.daemonSetLister = factory.Apps().V1().DaemonSets().Lister()
	sac.daemonSetInformer = factory.Apps().V1().DaemonSets().Informer()
	err = sac.appController.AddDaemonSetHandler(sac.daemonSetInformer)
	if nil != err {
		return err
	}

	sac.jobLister = factory.Batch().V1().Jobs().Lister()
	sac.jobInformer = factory.Batch().V1().Jobs().Informer()
	err = sac.appController.AddJobController(sac.jobInformer)
	if nil != err {
		return err
	}

	sac.cronJobLister = factory.Batch().V1().CronJobs().Lister()
	sac.cronJobInformer = factory.Batch().V1().CronJobs().Informer()
	err = sac.appController.AddCronJobHandler(sac.cronJobInformer)
	if nil != err {
		return err
	}

	// Once we lost the leader but get leader later, we have to use a new workqueue.
	// Because the former workqueue was already shut down and wouldn't be re-start forever.
	sac.workQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Application-Controllers")

	return nil
}

// controllerAddOrUpdateHandler serves for kubernetes original controller applications(such as: Deployment,ReplicaSet,Job...),
// to create a new IPPool or scale the IPPool
func (sac *SubnetAppController) controllerAddOrUpdateHandler() applicationinformers.AppInformersAddOrUpdateFunc {
	return func(ctx context.Context, oldObj, newObj interface{}) error {
		log := logutils.FromContext(ctx)

		var err error
		var oldSubnetConfig, newSubnetConfig *types.PodSubnetAnnoConfig
		var appKind string
		var app metav1.Object
		var oldAppReplicas, newAppReplicas int

		switch newObject := newObj.(type) {
		case *appsv1.Deployment:
			appKind = constant.KindDeployment
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			newAppReplicas = applicationinformers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if applicationinformers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldDeployment := oldObj.(*appsv1.Deployment)
				oldAppReplicas = applicationinformers.GetAppReplicas(oldDeployment.Spec.Replicas)
				oldSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(oldDeployment.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *appsv1.ReplicaSet:
			appKind = constant.KindReplicaSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil && owner.APIVersion == appsv1.SchemeGroupVersion.String() && owner.Kind == constant.KindDeployment {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = applicationinformers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if applicationinformers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldReplicaSet := oldObj.(*appsv1.ReplicaSet)
				oldAppReplicas = applicationinformers.GetAppReplicas(oldReplicaSet.Spec.Replicas)
				oldSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(oldReplicaSet.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *appsv1.StatefulSet:
			appKind = constant.KindStatefulSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			newAppReplicas = applicationinformers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if applicationinformers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldStatefulSet := oldObj.(*appsv1.StatefulSet)
				oldAppReplicas = applicationinformers.GetAppReplicas(oldStatefulSet.Spec.Replicas)
				oldSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(oldStatefulSet.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *batchv1.Job:
			appKind = constant.KindJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil && owner.APIVersion == batchv1.SchemeGroupVersion.String() && owner.Kind == constant.KindCronJob {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = applicationinformers.CalculateJobPodNum(newObject.Spec.Parallelism, newObject.Spec.Completions)
			newSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if applicationinformers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldJob := oldObj.(*batchv1.Job)
				oldAppReplicas = applicationinformers.CalculateJobPodNum(oldJob.Spec.Parallelism, oldJob.Spec.Completions)
				oldSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(oldJob.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *batchv1.CronJob:
			appKind = constant.KindCronJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.JobTemplate.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			newAppReplicas = applicationinformers.CalculateJobPodNum(newObject.Spec.JobTemplate.Spec.Parallelism, newObject.Spec.JobTemplate.Spec.Completions)
			newSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(newObject.Spec.JobTemplate.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if applicationinformers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldCronJob := oldObj.(*batchv1.CronJob)
				oldAppReplicas = applicationinformers.CalculateJobPodNum(oldCronJob.Spec.JobTemplate.Spec.Parallelism, oldCronJob.Spec.JobTemplate.Spec.Completions)
				oldSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(oldCronJob.Spec.JobTemplate.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		case *appsv1.DaemonSet:
			appKind = constant.KindDaemonSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			newAppReplicas = int(newObject.Status.DesiredNumberScheduled)
			newSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if applicationinformers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldDaemonSet := oldObj.(*appsv1.DaemonSet)
				oldAppReplicas = int(oldDaemonSet.Status.DesiredNumberScheduled)
				oldSubnetConfig, err = applicationinformers.GetSubnetAnnoConfig(oldDaemonSet.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		default:
			return fmt.Errorf("unrecognized application: %+v", newObj)
		}

		ctx = logutils.IntoContext(ctx, log)
		// check the difference between the two object and choose to reconcile or not
		if hasSubnetConfigChanged(ctx, oldSubnetConfig, newSubnetConfig, oldAppReplicas, newAppReplicas) {
			log.Debug("try to add app to application controller workequeue")
			sac.enqueueApp(ctx, app, appKind, app.GetUID())
		}

		return nil
	}
}

// appWorkQueueKey involves application object meta namespaceKey and application kind
type appWorkQueueKey struct {
	MetaNamespaceKey string
	AppKind          string
	AppUID           k8types.UID
}

// enqueueApp will insert application custom appWorkQueueKey to the workQueue
func (sac *SubnetAppController) enqueueApp(ctx context.Context, obj interface{}, appKind string, appUID k8types.UID) {
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
		AppUID:           appUID,
	}

	// validate workqueue capacity
	if sac.workQueue.Len() >= sac.MaxWorkqueueLength {
		log.Sugar().Errorf("The application controller workqueue is out of capacity, discard enqueue '%v'", appKey)
		return
	}

	sac.workQueue.Add(appKey)
	log.Sugar().Debugf("added '%v' to application controller workequeue", appKey)
}

func (sac *SubnetAppController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer sac.workQueue.ShutDown()

	logger.Debug("Waiting for application informers caches to sync")
	ok := cache.WaitForCacheSync(stopCh,
		sac.deploymentInformer.HasSynced,
		sac.replicaSetInformer.HasSynced,
		sac.daemonSetInformer.HasSynced,
		sac.statefulSetInformer.HasSynced,
		sac.jobInformer.HasSynced,
		sac.cronJobInformer.HasSynced,
	)
	if !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Sugar().Debugf("Starting application controller processing worker")
	go wait.Until(sac.runWorker, 1*time.Second, stopCh)

	<-stopCh
	logger.Error("Shutting down application controller workers")
	return nil
}

func (sac *SubnetAppController) runWorker() {
	for sac.processNextWorkItem() {
	}
}

func (sac *SubnetAppController) processNextWorkItem() bool {
	obj, shutdown := sac.workQueue.Get()
	if shutdown {
		logger.Error("application controller workqueue is already shutdown!")
		return false
	}

	var log *zap.Logger

	process := func(obj interface{}) error {
		defer sac.workQueue.Done(obj)
		key, ok := obj.(appWorkQueueKey)
		if !ok {
			sac.workQueue.Forget(obj)
			return fmt.Errorf("expected appWorkQueueKey in workQueue but got '%+v'", obj)
		}

		log = logger.With(
			zap.String("OPERATION", "process_app_items"),
			zap.String("Application", fmt.Sprintf("%s/%s", key.AppKind, key.MetaNamespaceKey)),
		)

		err := sac.syncHandler(key, log)
		if nil != err {
			// discard wrong input items
			if errors.Is(err, constant.ErrWrongInput) {
				sac.workQueue.Forget(obj)
				return err
			}

			// requeue the conflict items
			if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) || errors.Is(err, constant.ErrFreeIPsNotEnough) {
				sac.workQueue.AddRateLimited(obj)
				log.Sugar().Warnf("encountered app controller syncHandler conflict '%v', retrying...", err)
				return nil
			}

			// if we set nonnegative number for the requeue delay duration, we will requeue it. otherwise we will discard it.
			if sac.WorkQueueRequeueDelayDuration >= 0 {
				if sac.workQueue.NumRequeues(obj) < sac.WorkQueueMaxRetries {
					log.Sugar().Errorf("encountered app controller syncHandler error '%v', requeue it after '%v'", err, sac.WorkQueueRequeueDelayDuration)
					sac.workQueue.AddAfter(obj, sac.WorkQueueRequeueDelayDuration)
					return nil
				}

				log.Warn("out of work queue max retries, drop it")
			}

			sac.workQueue.Forget(obj)
			return err
		}

		sac.workQueue.Forget(obj)
		return nil
	}

	if err := process(obj); nil != err {
		log.Error(err.Error())
	}

	return true
}

// syncHandler retrieves appWorkQueueKey from workQueue and try to create the auto-created IPPool or mark the IPPool status.AutoDesiredIPCount
func (sac *SubnetAppController) syncHandler(appKey appWorkQueueKey, log *zap.Logger) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(appKey.MetaNamespaceKey)
	if nil != err {
		return fmt.Errorf("%w: failed to split meta namespace key '%+v', error: %v", constant.ErrWrongInput, appKey.MetaNamespaceKey, err)
	}

	var app metav1.Object
	var subnetConfig *types.PodSubnetAnnoConfig
	var podAnno map[string]string
	var appReplicas int
	var apiVersion string

	switch appKey.AppKind {
	case constant.KindDeployment:
		deployment, err := sac.deploymentsLister.Deployments(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return sac.deleteAutoPools(logutils.IntoContext(context.TODO(), log), appKey.AppUID)
			}
			return err
		}

		podAnno = deployment.Spec.Template.Annotations
		appReplicas = applicationinformers.GetAppReplicas(deployment.Spec.Replicas)
		app = deployment.DeepCopy()
		// deployment.APIVersion is empty string
		apiVersion = appsv1.SchemeGroupVersion.String()

	case constant.KindReplicaSet:
		replicaSet, err := sac.replicaSetLister.ReplicaSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return sac.deleteAutoPools(logutils.IntoContext(context.TODO(), log), appKey.AppUID)
			}
			return err
		}

		podAnno = replicaSet.Spec.Template.Annotations
		appReplicas = applicationinformers.GetAppReplicas(replicaSet.Spec.Replicas)
		app = replicaSet.DeepCopy()
		// replicaSet.APIVersion is empty string
		apiVersion = appsv1.SchemeGroupVersion.String()

	case constant.KindDaemonSet:
		daemonSet, err := sac.daemonSetLister.DaemonSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return sac.deleteAutoPools(logutils.IntoContext(context.TODO(), log), appKey.AppUID)
			}
			return err
		}

		podAnno = daemonSet.Spec.Template.Annotations
		appReplicas = int(daemonSet.Status.DesiredNumberScheduled)
		app = daemonSet.DeepCopy()
		// daemonSet.APIVersion is empty string
		apiVersion = appsv1.SchemeGroupVersion.String()

	case constant.KindStatefulSet:
		statefulSet, err := sac.statefulSetLister.StatefulSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return sac.deleteAutoPools(logutils.IntoContext(context.TODO(), log), appKey.AppUID)
			}
			return err
		}

		podAnno = statefulSet.Spec.Template.Annotations
		appReplicas = applicationinformers.GetAppReplicas(statefulSet.Spec.Replicas)
		app = statefulSet.DeepCopy()
		// statefulSet.APIVersion is empty string
		apiVersion = appsv1.SchemeGroupVersion.String()

	case constant.KindJob:
		job, err := sac.jobLister.Jobs(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return sac.deleteAutoPools(logutils.IntoContext(context.TODO(), log), appKey.AppUID)
			}
			return err
		}

		podAnno = job.Spec.Template.Annotations
		appReplicas = applicationinformers.CalculateJobPodNum(job.Spec.Parallelism, job.Spec.Completions)
		app = job.DeepCopy()
		// job.APIVersion is empty string
		apiVersion = batchv1.SchemeGroupVersion.String()

	case constant.KindCronJob:
		cronJob, err := sac.cronJobLister.CronJobs(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return sac.deleteAutoPools(logutils.IntoContext(context.TODO(), log), appKey.AppUID)
			}
			return err
		}

		podAnno = cronJob.Spec.JobTemplate.Spec.Template.Annotations
		appReplicas = applicationinformers.CalculateJobPodNum(cronJob.Spec.JobTemplate.Spec.Parallelism, cronJob.Spec.JobTemplate.Spec.Completions)
		app = cronJob.DeepCopy()
		// cronJob.APIVersion is empty string
		apiVersion = batchv1.SchemeGroupVersion.String()

	default:
		return fmt.Errorf("%w: unexpected appWorkQueueKey in workQueue '%+v'", constant.ErrWrongInput, appKey)
	}

	subnetConfig, err = applicationinformers.GetSubnetAnnoConfig(podAnno, log)
	if nil != err {
		return fmt.Errorf("%w: failed to get pod annotation subnet config, error: %v", constant.ErrWrongInput, err)
	}

	log.Debug("try to apply auto-created IPPool")
	err = sac.applyAutoIPPool(logutils.IntoContext(context.TODO(), log),
		*subnetConfig,
		types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: apiVersion,
				Kind:       appKey.AppKind,
				Namespace:  app.GetNamespace(),
				Name:       app.GetName(),
			},
			UID: app.GetUID(),
			APP: app,
		},
		appReplicas)
	if nil != err {
		return fmt.Errorf("failed to create or scale IPPool: %w", err)
	}

	return nil
}

// applyAutoIPPool try to create an IPPool or mark IPPool desired IP number with the give SpiderSubnet configuration
func (sac *SubnetAppController) applyAutoIPPool(ctx context.Context, podSubnetConfig types.PodSubnetAnnoConfig,
	podController types.PodTopController, appReplicas int) error {
	log := logutils.FromContext(ctx)

	// retrieve application pools
	fn := func(subnetName string, ipVersion types.IPVersion, ifName string) error {
		var tmpPool *spiderpoolv2beta1.SpiderIPPool
		tmpPoolList := &spiderpoolv2beta1.SpiderIPPoolList{}
		matchLabels := client.MatchingLabels{
			constant.LabelIPPoolOwnerSpiderSubnet:         subnetName,
			constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(podController.APIVersion),
			constant.LabelIPPoolOwnerApplicationKind:      podController.Kind,
			constant.LabelIPPoolOwnerApplicationNamespace: podController.Namespace,
			constant.LabelIPPoolOwnerApplicationName:      podController.Name,
			constant.LabelIPPoolIPVersion:                 applicationinformers.AutoPoolIPVersionLabelValue(ipVersion),
			constant.LabelIPPoolInterface:                 ifName,
		}

		err := sac.apiReader.List(ctx, tmpPoolList, matchLabels)
		if nil != err {
			return fmt.Errorf("failed to get auto-created IPPoolList with matchLabels %v, error: %w", matchLabels, err)
		}

		if len(tmpPoolList.Items) == 0 {
			tmpPool = nil
		} else {
			for i := range tmpPoolList.Items {
				// We need to ignore the previous same NamespacedName application corresponding auto-created IPPool.
				// Because this auto-created IPPool will be deleted by the system with 'ippool-reclaim'
				labels := tmpPoolList.Items[i].GetLabels()
				if labels[constant.LabelIPPoolReclaimIPPool] == constant.True && labels[constant.LabelIPPoolOwnerApplicationUID] != string(podController.UID) {
					log.Sugar().Debugf("found the previous same app auto-created IPPool %s", tmpPoolList.Items[i].Name)
					continue
				}

				// If 'ippool-reclaim' is true, we'll go to scale the auto-created IPPool because of the same application UID.
				// If 'ippool-reclaim' is false, we'll reuse this auto-crated IPPool and refresh its application UID label.
				tmpPool = tmpPoolList.Items[i].DeepCopy()
				log.Sugar().Debugf("found reuse app auto-created IPPool %s", tmpPool.Name)
				break
			}
		}

		var desiredIPNumber int
		var annoPoolIPNumberVal string
		if podSubnetConfig.FlexibleIPNum != nil {
			desiredIPNumber = appReplicas + *(podSubnetConfig.FlexibleIPNum)
			annoPoolIPNumberVal = fmt.Sprintf("+%d", *podSubnetConfig.FlexibleIPNum)
		} else {
			desiredIPNumber = podSubnetConfig.AssignIPNum
			annoPoolIPNumberVal = strconv.Itoa(podSubnetConfig.AssignIPNum)
		}

		log.Sugar().Infof("try to reconcile auto-created IPv%d IPPool for Interface %s by SpiderSubnet %s with application controller %v",
			ipVersion, ifName, subnetName, podController.AppNamespacedName)
		_, err = sac.subnetMgr.ReconcileAutoIPPool(ctx, tmpPool, subnetName, podController, types.AutoPoolProperty{
			DesiredIPNumber:     desiredIPNumber,
			IPVersion:           ipVersion,
			IsReclaimIPPool:     podSubnetConfig.ReclaimIPPool,
			IfName:              ifName,
			AnnoPoolIPNumberVal: annoPoolIPNumberVal,
		})
		if nil != err {
			return err
		}

		return nil
	}

	processNext := func(item types.AnnoSubnetItem) error {
		var errV4, errV6 error
		var wg sync.WaitGroup
		if sac.EnableIPv4 && len(item.IPv4) != 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errV4 = fn(item.IPv4[0], constant.IPv4, item.Interface)
			}()
		}
		if sac.EnableIPv6 && len(item.IPv6) != 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errV6 = fn(item.IPv6[0], constant.IPv6, item.Interface)
			}()
		}

		wg.Wait()

		if errV4 != nil || errV6 != nil {
			// NewAggregate will check each the given error slice elements whether is nil or not
			return multierr.Append(errV4, errV6)
		}

		return nil
	}

	if len(podSubnetConfig.MultipleSubnets) != 0 {
		for index := range podSubnetConfig.MultipleSubnets {
			err := processNext(podSubnetConfig.MultipleSubnets[index])
			if nil != err {
				return err
			}
		}
	} else if podSubnetConfig.SingleSubnet != nil {
		err := processNext(*podSubnetConfig.SingleSubnet)
		if nil != err {
			return err
		}
	} else {
		return fmt.Errorf("%w: no subnets specified to create or mark auto-created IPPool for application '%s/%s/%s', the pod subnet configuration is %v",
			constant.ErrWrongInput, podController.Kind, podController.Namespace, podController.Name, podSubnetConfig)
	}

	return nil
}

// hasSubnetConfigChanged checks whether application subnet configuration changed and the application replicas changed or not.
// The second parameter newSubnetConfig must not be nil.
func hasSubnetConfigChanged(ctx context.Context, oldSubnetConfig, newSubnetConfig *types.PodSubnetAnnoConfig,
	oldAppReplicas, newAppReplicas int) bool {
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
	}

	return isChanged
}

// controllerDeleteHandler will return a function that clean up the application SpiderSubnet legacies (such as: the before created IPPools)
func (sac *SubnetAppController) controllerDeleteHandler() applicationinformers.APPInformersDelFunc {
	return func(ctx context.Context, obj interface{}) error {
		log := logutils.FromContext(ctx)

		var appKind string
		var app metav1.Object

		switch object := obj.(type) {
		case *appsv1.Deployment:
			appKind = constant.KindDeployment
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))
			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}
			app = object

		case *appsv1.ReplicaSet:
			appKind = constant.KindReplicaSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))
			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}
			app = object

		case *appsv1.StatefulSet:
			appKind = constant.KindStatefulSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))
			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}
			app = object

		case *batchv1.Job:
			appKind = constant.KindJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))
			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}
			app = object

		case *batchv1.CronJob:
			appKind = constant.KindCronJob
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))
			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}
			app = object

		case *appsv1.DaemonSet:
			appKind = constant.KindDaemonSet
			log = log.With(zap.String(appKind, fmt.Sprintf("%s/%s", object.Namespace, object.Name)))
			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("the application has a owner '%s/%s', we would not clean up legacy for it", owner.Kind, owner.Name)
				return nil
			}
			app = object

		default:
			return fmt.Errorf("%w: unrecognized application: %+v", constant.ErrWrongInput, obj)
		}

		err := sac.deleteAutoPools(logutils.IntoContext(ctx, log), app.GetUID())
		if nil != err {
			log.Sugar().Errorf("failed to clean up legacy IPPool, error: %v", err)
			sac.enqueueApp(ctx, obj, appKind, app.GetUID())
			return nil
		}

		log.Info("delete application corresponding auto-created IPPools successfully")
		return nil
	}
}

func (sac *SubnetAppController) deleteAutoPools(ctx context.Context, appUID k8types.UID) error {
	log := logutils.FromContext(ctx)
	err := sac.client.DeleteAllOf(ctx, &spiderpoolv2beta1.SpiderIPPool{}, client.MatchingLabels{
		constant.LabelIPPoolOwnerApplicationUID: string(appUID),
		constant.LabelIPPoolReclaimIPPool:       constant.True,
	})
	if nil != err {
		if apierrors.IsNotFound(err) {
			log.Info("delete application corresponding auto-created IPPools successfully")
			return nil
		}
		return err
	}
	return nil
}
