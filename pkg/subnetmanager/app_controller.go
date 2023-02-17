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
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	metrics "github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var informerLogger *zap.Logger

type SubnetAppController struct {
	client        client.Client
	subnetMgr     SubnetManager
	workQueue     workqueue.RateLimitingInterface
	appController *controllers.Controller

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
	WorkQueueRequeueDelayDuration time.Duration
	LeaderRetryElectGap           time.Duration
}

func (sac *SubnetAppController) SetupInformer(ctx context.Context, client kubernetes.Interface, controllerLeader election.SpiderLeaseElector) error {
	if controllerLeader == nil {
		return fmt.Errorf("failed to start SpiderSubnet App informer, controller leader must be specified")
	}

	informerLogger.Info("try to register SpiderSubnet App informer")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !controllerLeader.IsElected() {
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

					if !controllerLeader.IsElected() {
						informerLogger.Warn("Leader lost, stop Subnet App informer")
						innerCancel()
						return
					}
					time.Sleep(sac.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("create SpiderSubnet App informer")
			factory := kubeinformers.NewSharedInformerFactory(client, 0)
			sac.addEventHandlers(factory)
			factory.Start(innerCtx.Done())
			err := sac.Run(innerCtx.Done())
			if nil != err {
				informerLogger.Sugar().Errorf("failed to run SpiderSubnet App controller, error: %v", err)
			}
			informerLogger.Error("SpiderSubnet App informer broken")
		}
	}()

	return nil
}

// ControllerAddOrUpdateHandler serves for kubernetes original controller applications(such as: Deployment,ReplicaSet,Job...),
// to create a new IPPool or scale the IPPool
func (sac *SubnetAppController) ControllerAddOrUpdateHandler() controllers.AppInformersAddOrUpdateFunc {
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

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if controllers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldDeployment := oldObj.(*appsv1.Deployment)
				oldAppReplicas = controllers.GetAppReplicas(oldDeployment.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldDeployment.Spec.Template.Annotations, log)
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
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if controllers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldReplicaSet := oldObj.(*appsv1.ReplicaSet)
				oldAppReplicas = controllers.GetAppReplicas(oldReplicaSet.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldReplicaSet.Spec.Template.Annotations, log)
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

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if controllers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldStatefulSet := oldObj.(*appsv1.StatefulSet)
				oldAppReplicas = controllers.GetAppReplicas(oldStatefulSet.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldStatefulSet.Spec.Template.Annotations, log)
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
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.CalculateJobPodNum(newObject.Spec.Parallelism, newObject.Spec.Completions)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if controllers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldJob := oldObj.(*batchv1.Job)
				oldAppReplicas = controllers.CalculateJobPodNum(oldJob.Spec.Parallelism, oldJob.Spec.Completions)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldJob.Spec.Template.Annotations, log)
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

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.CalculateJobPodNum(newObject.Spec.JobTemplate.Spec.Parallelism, newObject.Spec.JobTemplate.Spec.Completions)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.JobTemplate.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if controllers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldCronJob := oldObj.(*batchv1.CronJob)
				oldAppReplicas = controllers.CalculateJobPodNum(oldCronJob.Spec.JobTemplate.Spec.Parallelism, oldCronJob.Spec.JobTemplate.Spec.Completions)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldCronJob.Spec.JobTemplate.Spec.Template.Annotations, log)
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

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("app has a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = int(newObject.Status.DesiredNumberScheduled)
			newSubnetConfig, err = controllers.GetSubnetAnnoConfig(newObject.Spec.Template.Annotations, log)
			if nil != err {
				return fmt.Errorf("failed to get app subnet configuration, error: %v", err)
			}

			// default IPAM mode
			if controllers.IsDefaultIPPoolMode(newSubnetConfig) {
				log.Debug("app will use default IPAM mode, because there's no subnet annotation or no ClusterDefaultSubnets")
				return nil
			}

			app = newObject.DeepCopy()

			if oldObj != nil {
				oldDaemonSet := oldObj.(*appsv1.DaemonSet)
				oldAppReplicas = int(oldDaemonSet.Status.DesiredNumberScheduled)
				oldSubnetConfig, err = controllers.GetSubnetAnnoConfig(oldDaemonSet.Spec.Template.Annotations, log)
				if nil != err {
					return fmt.Errorf("failed to get old app subnet configuration, error: %v", err)
				}
			}

		default:
			return fmt.Errorf("unrecognized application: %+v", newObj)
		}

		ctx = logutils.IntoContext(ctx, log)
		// check the difference between the two object and choose to reconcile or not
		if sac.hasSubnetConfigChanged(ctx, oldSubnetConfig, newSubnetConfig, oldAppReplicas, newAppReplicas) {
			log.Debug("try to add app to application controller workequeue")
			sac.enqueueApp(ctx, app, appKind)
		}

		return nil
	}
}

func NewSubnetAppController(client client.Client, subnetMgr SubnetManager, subnetAppControllerConfig SubnetAppControllerConfig) (*SubnetAppController, error) {
	informerLogger = logutils.Logger.Named("SpiderSubnet-Application-Controllers")

	c := &SubnetAppController{
		client:                    client,
		subnetMgr:                 subnetMgr,
		SubnetAppControllerConfig: subnetAppControllerConfig,
	}

	appController, err := controllers.NewApplicationController(c.ControllerAddOrUpdateHandler(), c.ControllerDeleteHandler(), informerLogger)
	if nil != err {
		return nil, err
	}
	c.appController = appController

	return c, nil
}

func (sac *SubnetAppController) addEventHandlers(factory kubeinformers.SharedInformerFactory) {
	sac.deploymentsLister = factory.Apps().V1().Deployments().Lister()
	sac.deploymentInformer = factory.Apps().V1().Deployments().Informer()
	sac.appController.AddDeploymentHandler(sac.deploymentInformer)

	sac.replicaSetLister = factory.Apps().V1().ReplicaSets().Lister()
	sac.replicaSetInformer = factory.Apps().V1().ReplicaSets().Informer()
	sac.appController.AddReplicaSetHandler(sac.replicaSetInformer)

	sac.statefulSetLister = factory.Apps().V1().StatefulSets().Lister()
	sac.statefulSetInformer = factory.Apps().V1().StatefulSets().Informer()
	sac.appController.AddStatefulSetHandler(sac.statefulSetInformer)

	sac.daemonSetLister = factory.Apps().V1().DaemonSets().Lister()
	sac.daemonSetInformer = factory.Apps().V1().DaemonSets().Informer()
	sac.appController.AddDaemonSetHandler(sac.daemonSetInformer)

	sac.jobLister = factory.Batch().V1().Jobs().Lister()
	sac.jobInformer = factory.Batch().V1().Jobs().Informer()
	sac.appController.AddJobController(sac.jobInformer)

	sac.cronJobLister = factory.Batch().V1().CronJobs().Lister()
	sac.cronJobInformer = factory.Batch().V1().CronJobs().Informer()
	sac.appController.AddCronJobHandler(sac.cronJobInformer)

	// Once we lost the leader but get leader later, we have to use a new workqueue.
	// Because the former workqueue was already shut down and wouldn't be re-start forever.
	sac.workQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Application-Controllers")
}

// appWorkQueueKey involves application object meta namespaceKey and application kind
type appWorkQueueKey struct {
	MetaNamespaceKey string
	AppKind          string
}

// enqueueApp will insert application custom appWorkQueueKey to the workQueue
func (sac *SubnetAppController) enqueueApp(ctx context.Context, obj interface{}, appKind string) {
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

	informerLogger.Debug("Waiting for application informers caches to sync")
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

	for i := 0; i < sac.AppControllerWorkers; i++ {
		informerLogger.Sugar().Debugf("Starting application controller processing worker '%d'", i)
		go wait.Until(sac.runWorker, 1*time.Second, stopCh)
	}

	informerLogger.Info("application controller workers started")

	<-stopCh
	informerLogger.Error("Shutting down application controller workers")
	return nil
}

func (sac *SubnetAppController) runWorker() {
	for sac.processNextWorkItem() {
	}
}

func (sac *SubnetAppController) processNextWorkItem() bool {
	obj, shutdown := sac.workQueue.Get()
	if shutdown {
		informerLogger.Error("application controller workqueue is already shutdown!")
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

		log = informerLogger.With(
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
			if apierrors.IsConflict(err) {
				metrics.AutoPoolCreateOrMarkConflictCounts.Add(context.TODO(), 1)
				sac.workQueue.AddRateLimited(obj)
				log.Sugar().Warnf("encountered app controller syncHandler conflict '%v', retrying...", err)
				return nil
			}

			// if we set nonnegative number for the requeue delay duration, we will requeue it. otherwise we will discard it.
			if sac.WorkQueueRequeueDelayDuration >= 0 {
				if sac.workQueue.NumRequeues(obj) < sac.MaxWorkqueueLength {
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
	var podSelector *metav1.LabelSelector
	var appReplicas int

	switch appKey.AppKind {
	case constant.KindDeployment:
		deployment, err := sac.deploymentsLister.Deployments(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = deployment.Spec.Template.Annotations
		podSelector = deployment.Spec.Selector
		appReplicas = controllers.GetAppReplicas(deployment.Spec.Replicas)
		app = deployment.DeepCopy()

	case constant.KindReplicaSet:
		replicaSet, err := sac.replicaSetLister.ReplicaSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = replicaSet.Spec.Template.Annotations
		podSelector = replicaSet.Spec.Selector
		appReplicas = controllers.GetAppReplicas(replicaSet.Spec.Replicas)
		app = replicaSet.DeepCopy()

	case constant.KindDaemonSet:
		daemonSet, err := sac.daemonSetLister.DaemonSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = daemonSet.Spec.Template.Annotations
		podSelector = daemonSet.Spec.Selector
		appReplicas = int(daemonSet.Status.DesiredNumberScheduled)
		app = daemonSet.DeepCopy()

	case constant.KindStatefulSet:
		statefulSet, err := sac.statefulSetLister.StatefulSets(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = statefulSet.Spec.Template.Annotations
		podSelector = statefulSet.Spec.Selector
		appReplicas = controllers.GetAppReplicas(statefulSet.Spec.Replicas)
		app = statefulSet.DeepCopy()

	case constant.KindJob:
		job, err := sac.jobLister.Jobs(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = job.Spec.Template.Annotations
		podSelector = job.Spec.Selector
		appReplicas = controllers.CalculateJobPodNum(job.Spec.Parallelism, job.Spec.Completions)
		app = job.DeepCopy()

	case constant.KindCronJob:
		cronJob, err := sac.cronJobLister.CronJobs(namespace).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Debugf("application in work queue no longer exists")
				return nil
			}
			return err
		}

		podAnno = cronJob.Spec.JobTemplate.Spec.Template.Annotations
		podSelector = cronJob.Spec.JobTemplate.Spec.Selector
		appReplicas = controllers.CalculateJobPodNum(cronJob.Spec.JobTemplate.Spec.Parallelism, cronJob.Spec.JobTemplate.Spec.Completions)
		app = cronJob.DeepCopy()

	default:
		return fmt.Errorf("%w: unexpected appWorkQueueKey in workQueue '%+v'", constant.ErrWrongInput, appKey)
	}

	subnetConfig, err = controllers.GetSubnetAnnoConfig(podAnno, log)
	if nil != err {
		return fmt.Errorf("%w: failed to get pod annotation subnet config, error: %v", constant.ErrWrongInput, err)
	}

	log.Debug("Going to create IPPool or mark IPPool desired IP number")
	err = sac.createOrMarkIPPool(logutils.IntoContext(context.TODO(), log),
		*subnetConfig,
		types.PodTopController{
			Kind:      appKey.AppKind,
			Namespace: app.GetNamespace(),
			Name:      app.GetName(),
			UID:       app.GetUID(),
			APP:       app,
		},
		podSelector,
		appReplicas)
	if nil != err {
		return fmt.Errorf("failed to create or scale IPPool: %w", err)
	}

	return nil
}

// createOrMarkIPPool try to create an IPPool or mark IPPool desired IP number with the give SpiderSubnet configuration
func (sac *SubnetAppController) createOrMarkIPPool(ctx context.Context, podSubnetConfig types.PodSubnetAnnoConfig,
	podController types.PodTopController, podSelector *metav1.LabelSelector, appReplicas int) error {
	log := logutils.FromContext(ctx)

	// retrieve application pools
	fn := func(poolList spiderpoolv1.SpiderIPPoolList, subnetName string, ipVersion types.IPVersion, ifName string, matchLabel client.MatchingLabels) (err error) {
		var ipNum int
		if podSubnetConfig.FlexibleIPNum != nil {
			ipNum = appReplicas + *(podSubnetConfig.FlexibleIPNum)
		} else {
			ipNum = podSubnetConfig.AssignIPNum
		}

		// verify whether the pool IPs need to be expanded or not
		if len(poolList.Items) == 0 {
			log.Sugar().Debugf("there's no 'IPv%d' IPPoolList retrieved from SpiderSubent '%s' with matchLabel '%v'", ipVersion, subnetName, matchLabel)
			// create an empty IPPool and mark the desired IP number when the subnet name was specified,
			// and the IPPool informer will implement the scale action
			_, err = sac.subnetMgr.AllocateEmptyIPPool(ctx, subnetName, podController, podSelector, ipNum, ipVersion, podSubnetConfig.ReclaimIPPool, ifName)
		} else if len(poolList.Items) == 1 {
			pool := poolList.Items[0]
			log.Sugar().Debugf("found SpiderSubnet '%s' IPPool '%s' with matchLabel '%v', check it whether need to be scaled", subnetName, pool.Name, matchLabel)
			_, err = sac.subnetMgr.CheckScaleIPPool(ctx, &pool, subnetName, ipNum)
		} else {
			err = fmt.Errorf("%w: it's invalid that SpiderSubnet '%s' owns multiple matchLabel '%v' corresponding IPPools '%v' for one specify application",
				constant.ErrWrongInput, subnetName, matchLabel, poolList.Items)
		}

		return
	}

	processNext := func(item types.AnnoSubnetItem) error {
		if sac.EnableIPv4 && len(item.IPv4) == 0 {
			return fmt.Errorf("IPv4 SpiderSubnet not specified when configuration enableIPv4 is on")
		}
		if sac.EnableIPv6 && len(item.IPv6) == 0 {
			return fmt.Errorf("IPv6 SpiderSubnet not specified when configuration enableIPv6 is on")
		}

		var errV4, errV6 error
		var wg sync.WaitGroup
		if sac.EnableIPv4 && len(item.IPv4) != 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				var v4PoolList spiderpoolv1.SpiderIPPoolList
				matchLabel := client.MatchingLabels{
					constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
					constant.LabelIPPoolOwnerSpiderSubnet:   item.IPv4[0],
					constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
					constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV4,
					constant.LabelIPPoolInterface:           item.Interface,
				}
				errV4 = sac.client.List(ctx, &v4PoolList, matchLabel)
				if nil != errV4 {
					return
				}

				errV4 = fn(v4PoolList, item.IPv4[0], constant.IPv4, item.Interface, matchLabel)
			}()
		}

		if sac.EnableIPv6 && len(item.IPv6) != 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				var v6PoolList spiderpoolv1.SpiderIPPoolList
				matchLabel := client.MatchingLabels{
					constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
					constant.LabelIPPoolOwnerSpiderSubnet:   item.IPv6[0],
					constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
					constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV6,
					constant.LabelIPPoolInterface:           item.Interface,
				}
				errV6 = sac.client.List(ctx, &v6PoolList, matchLabel)
				if nil != errV6 {
					return
				}

				errV6 = fn(v6PoolList, item.IPv6[0], constant.IPv6, item.Interface, matchLabel)
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
func (sac *SubnetAppController) hasSubnetConfigChanged(ctx context.Context, oldSubnetConfig, newSubnetConfig *types.PodSubnetAnnoConfig,
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

// ControllerDeleteHandler will return a function that clean up the application SpiderSubnet legacies (such as: the before created IPPools)
func (sac *SubnetAppController) ControllerDeleteHandler() controllers.APPInformersDelFunc {
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
			return fmt.Errorf("unrecognized application: %+v", obj)
		}

		// clean up all legacy IPPools that matched with the application UID
		err := sac.tryToCleanUpLegacyIPPools(logutils.IntoContext(ctx, log), app)
		if nil != err {
			log.Sugar().Errorf("failed to clean up legacy IPPool, error: %v", err)
		}

		return nil
	}
}

func (sac *SubnetAppController) tryToCleanUpLegacyIPPools(ctx context.Context, app metav1.Object, labels ...client.MatchingLabels) error {
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

	var poolList spiderpoolv1.SpiderIPPoolList
	err := sac.client.List(ctx, &poolList, matchLabel)
	if nil != err {
		return err
	}

	if len(poolList.Items) != 0 {
		wg := new(sync.WaitGroup)

		deletePool := func(pool *spiderpoolv1.SpiderIPPool) {
			defer wg.Done()
			err := sac.client.Delete(ctx, pool)
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
