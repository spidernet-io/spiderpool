// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (sm *subnetManager) SetupControllers(ctx context.Context, client kubernetes.Interface) error {
	subnetController, err := controllers.NewSubnetController(sm.ControllerAddOrUpdateHandler(), sm.ControllerDeleteHandler(), logger.Named("Controllers"))
	if nil != err {
		return fmt.Errorf("failed to set up controllers informers, error: %v", err)
	}

	go func() {
		for {
			if !sm.leader.IsElected() {
				time.Sleep(sm.config.LeaderRetryElectGap)
				continue
			}

			logger.Info("Starting subnet manager controllers informers")
			kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
			stopper := make(chan struct{})

			go subnetController.StartDeploymentController(kubeInformerFactory.Apps().V1().Deployments().Informer(), stopper)
			go subnetController.StartReplicaSetController(kubeInformerFactory.Apps().V1().ReplicaSets().Informer(), stopper)
			go subnetController.StartDaemonSetController(kubeInformerFactory.Apps().V1().DaemonSets().Informer(), stopper)
			go subnetController.StartStatefulSetController(kubeInformerFactory.Apps().V1().StatefulSets().Informer(), stopper)
			go subnetController.StartJobController(kubeInformerFactory.Batch().V1().Jobs().Informer(), stopper)
			go subnetController.StartCronJobController(kubeInformerFactory.Batch().V1().CronJobs().Informer(), stopper)

			go func() {
				for {
					if !sm.leader.IsElected() {
						logger.Warn("leader lost! stop subnet controllers!")
						close(stopper)
						return
					}

					time.Sleep(sm.config.LeaderRetryElectGap)
				}
			}()

			<-stopper
			logger.Error("subnet manager controllers broken")
		}
	}()

	return nil
}

// ControllerAddOrUpdateHandler serves for kubernetes original controller applications(such as: Deployment,ReplicaSet,Job...),
// to create a new IPPool or scale the IPPool
func (sm *subnetManager) ControllerAddOrUpdateHandler() controllers.AppInformersAddOrUpdateFunc {
	return func(ctx context.Context, oldObj, newObj interface{}) error {
		var log *zap.Logger

		var err error
		var oldSubnetConfig, newSubnetConfig *controllers.PodSubnetAnnoConfig
		var appKind string
		var app metav1.Object
		var podSelector map[string]string
		var oldAppReplicas, newAppReplicas int

		switch newObject := newObj.(type) {
		case *appsv1.Deployment:
			appKind = constant.OwnerDeployment
			log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

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
				if sm.config.EnableSubnetDeleteStaleIPPool {
					err = sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

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
			log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

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
				if sm.config.EnableSubnetDeleteStaleIPPool {
					err = sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

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
			log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Debug("HostNetwork mode, we would not create or scale IPPool for it")
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("apphas a owner '%s/%s', we would not create or scale IPPool for it", owner.Kind, owner.Name)
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
				if sm.config.EnableSubnetDeleteStaleIPPool {
					err = sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

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
			log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

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
				if sm.config.EnableSubnetDeleteStaleIPPool {
					err = sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

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
			log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

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
				if sm.config.EnableSubnetDeleteStaleIPPool {
					err = sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			var lastJob batchv1.Job
			lastJobName := newObject.Status.Active[len(newObject.Status.Active)].Name
			err = sm.client.Get(ctx, apitypes.NamespacedName{Namespace: newObject.Namespace, Name: lastJobName}, &lastJob)
			if nil != err {
				return fmt.Errorf("failed to get Job '%s/%s', error: %v", newObject.Namespace, lastJobName, err)
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.JobTemplate.Spec.Selector.MatchLabels

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
			log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", newObject.GetNamespace(), newObject.GetName())))

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
				if sm.config.EnableSubnetDeleteStaleIPPool {
					err = sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
					if nil != err {
						log.Sugar().Errorf("failed try to clean up legacy IPPools, error: %v", err)
					}
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

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

		// check the difference between the two object and choose to reconcile or not
		if sm.hasSubnetConfigChanged(logutils.IntoContext(ctx, log), oldSubnetConfig, newSubnetConfig, oldAppReplicas, newAppReplicas, appKind, app) {
			go func() {
				log.Debug("Going to create IPPool or mark IPPool desired IP number")
				err = sm.createOrMarkIPPool(ctx, *newSubnetConfig, appKind, app, podSelector, newAppReplicas)
				if nil != err {
					log.Sugar().Errorf("failed to create or scale IPPool, error: %v", err)
				}
			}()
		}

		return nil
	}
}

// createOrMarkIPPool try to create an IPPool or mark IPPool desired IP number with the give SpiderSubnet configuration
func (sm *subnetManager) createOrMarkIPPool(ctx context.Context, podSubnetConfig controllers.PodSubnetAnnoConfig, appKind string, app metav1.Object,
	podSelector map[string]string, appReplicas int) error {

	// retrieve application pools
	f := func(ctx context.Context, poolList *spiderpoolv1.SpiderIPPoolList, subnetName string, ipVersion types.IPVersion) error {
		var ipNum int
		if podSubnetConfig.FlexibleIPNum != nil {
			ipNum = appReplicas + *(podSubnetConfig.FlexibleIPNum)
		} else {
			ipNum = podSubnetConfig.AssignIPNum
		}

		// verify whether the pool IPs need to be expanded or not
		if poolList == nil || len(poolList.Items) == 0 {
			// create an empty IPPool and mark the desired IP number when the subnet name was specified,
			// and the IPPool informer will implement the scale action
			err := sm.AllocateEmptyIPPool(ctx, subnetName, appKind, app, podSelector, ipNum, ipVersion, podSubnetConfig.ReclaimIPPool)
			if nil != err {
				return err
			}
		} else if len(poolList.Items) == 1 {
			pool := poolList.Items[0]
			err := sm.CheckScaleIPPool(ctx, &pool, subnetName, ipNum)
			if nil != err {
				return err
			}
		} else {
			return fmt.Errorf("it's invalid for '%s/%s/%s' corresponding SpiderSubnet '%s' owns multiple IPPools '%v' for one specify application",
				appKind, app.GetNamespace(), app.GetName(), subnetName, poolList.Items)
		}

		return nil
	}

	if sm.config.EnableIPv4 && len(podSubnetConfig.SubnetName.IPv4) == 0 {
		return fmt.Errorf("IPv4 SpiderSubnet not specified when configuration enableIPv4 is on")
	}
	if sm.config.EnableIPv6 && len(podSubnetConfig.SubnetName.IPv6) == 0 {
		return fmt.Errorf("IPv6 SpiderSubnet not specified when configuration enableIPv6 is on")
	}

	var errs []error
	if sm.config.EnableIPv4 {
		if len(podSubnetConfig.SubnetName.IPv4) != 0 {
			v4PoolList, err := sm.ipPoolManager.ListIPPools(ctx, client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
				constant.LabelIPPoolOwnerSpiderSubnet:   podSubnetConfig.SubnetName.IPv4[0],
				constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV4,
			})
			if nil != err {
				return err
			}

			err = f(ctx, v4PoolList, podSubnetConfig.SubnetName.IPv4[0], constant.IPv4)
			if nil != err {
				errs = append(errs, err)
			}
		}
	}

	if sm.config.EnableIPv6 {
		if len(podSubnetConfig.SubnetName.IPv6) != 0 {
			v6PoolList, err := sm.ipPoolManager.ListIPPools(ctx, client.MatchingLabels{
				constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
				constant.LabelIPPoolOwnerSpiderSubnet:   podSubnetConfig.SubnetName.IPv6[0],
				constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
				constant.LabelIPPoolVersion:             constant.LabelIPPoolVersionV6,
			})
			if nil != err {
				return err
			}

			err = f(ctx, v6PoolList, podSubnetConfig.SubnetName.IPv6[0], constant.IPv6)
			if nil != err {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}

	return nil
}

// hasSubnetConfigChanged checks whether application subnet configuration changed and the application replicas changed or not.
// The second parameter newSubnetConfig must not be nil.
func (sm *subnetManager) hasSubnetConfigChanged(ctx context.Context, oldSubnetConfig, newSubnetConfig *controllers.PodSubnetAnnoConfig,
	oldAppReplicas, newAppReplicas int, appKind string, app metav1.Object) bool {
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
			if sm.config.EnableSubnetDeleteStaleIPPool {
				if err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, app, client.MatchingLabels{
					constant.LabelIPPoolOwnerSpiderSubnet: oldSubnetConfig.SubnetName.IPv4[0],
				}); err != nil {
					log.Sugar().Errorf("failed to clean up SpiderSubnet '%s' legacy V4 IPPools, error: %v", oldSubnetConfig.SubnetName.IPv4[0], err)
				}
			}
		}

		if len(oldSubnetConfig.SubnetName.IPv6) != 0 && oldSubnetConfig.SubnetName.IPv6[0] != newSubnetConfig.SubnetName.IPv6[0] {
			log.Sugar().Warnf("change SpiderSubnet IPv6 from '%s' to '%s'", oldSubnetConfig.SubnetName.IPv6[0], newSubnetConfig.SubnetName.IPv6[0])

			// we should clean up the legacy IPPools once changed the SpiderSubnet
			if sm.config.EnableSubnetDeleteStaleIPPool {
				if err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, app, client.MatchingLabels{
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
func (sm *subnetManager) ControllerDeleteHandler() controllers.APPInformersDelFunc {
	return func(ctx context.Context, obj interface{}) error {
		log := logutils.FromContext(ctx)

		var appKind string
		var app metav1.Object
		var logPrefix string

		switch object := obj.(type) {
		case *appsv1.Deployment:
			appKind = constant.OwnerDeployment
			logPrefix = fmt.Sprintf("application '%s/%s/%s'", appKind, object.Namespace, object.Name)

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not clean up legacies for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *appsv1.ReplicaSet:
			appKind = constant.OwnerReplicaSet
			logPrefix = fmt.Sprintf("application '%s/%s/%s'", appKind, object.Namespace, object.Name)

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not clean up legacies for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *appsv1.StatefulSet:
			appKind = constant.OwnerStatefulSet
			logPrefix = fmt.Sprintf("application '%s/%s/%s'", appKind, object.Namespace, object.Name)

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not clean up legacies for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *batchv1.Job:
			appKind = constant.OwnerJob
			logPrefix = fmt.Sprintf("application '%s/%s/%s'", appKind, object.Namespace, object.Name)

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not clean up legacies for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			app = object

		case *batchv1.CronJob:
			appKind = constant.OwnerCronJob
			logPrefix = fmt.Sprintf("application '%s/%s/%s'", appKind, object.Namespace, object.Name)

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not clean up legacies for it", logPrefix, owner.Kind, owner.Name)

				return nil
			}

			app = object

		case *appsv1.DaemonSet:
			appKind = constant.OwnerDaemonSet
			logPrefix = fmt.Sprintf("application '%s/%s/%s'", appKind, object.Namespace, object.Name)

			owner := metav1.GetControllerOf(object)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not clean up legacies for it", logPrefix, owner.Kind, owner.Name)

				return nil
			}

			app = object

		default:
			return fmt.Errorf("unrecognized application: %+v", obj)
		}

		// clean up all legacy IPPools that matched with the application UID
		go func() {
			err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, app)
			if nil != err {
				log.Sugar().Errorf("failed to clean up legacy IPPool, error: %v", err)
			}
		}()

		return nil
	}
}

func (sm *subnetManager) tryToCleanUpLegacyIPPools(ctx context.Context, appKind string, app metav1.Object, labels ...client.MatchingLabels) error {
	log := logutils.FromContext(ctx).With(zap.String(appKind, fmt.Sprintf("%s/%s", app.GetNamespace(), app.GetName())))

	matchLabel := client.MatchingLabels{
		constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
		constant.LabelIPPoolReclaimIPPool:       constant.True,
	}

	for _, label := range labels {
		for k, v := range label {
			matchLabel[k] = v
		}
	}

	poolList, err := sm.ipPoolManager.ListIPPools(ctx, matchLabel)
	if nil != err {
		return fmt.Errorf("failed to retrieve '%s/%s/%s' IPPools, error: %v", appKind, app.GetNamespace(), app.GetName(), err)
	}

	if poolList != nil && len(poolList.Items) != 0 {
		wg := new(sync.WaitGroup)

		deletePool := func(pool *spiderpoolv1.SpiderIPPool) {
			defer wg.Done()
			err := sm.ipPoolManager.DeleteIPPool(ctx, pool)
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
