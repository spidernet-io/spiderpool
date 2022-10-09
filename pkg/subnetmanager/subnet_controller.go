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
	subnetController, err := controllers.NewSubnetController(sm.newReconcile(), sm.newCleanUpApp(), logger.Named("Controllers"))
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

// newReconcile serves for kubernetes original controller applications(such as: Deployment,ReplicaSet,Job...),
// to create a new IPPool or scale the IPPool
func (sm *subnetManager) newReconcile() controllers.ReconcileAppInformersFunc {
	return func(ctx context.Context, oldObj, newObj interface{}) error {
		log := logutils.FromContext(ctx)

		var err error
		var oldSubnetConfig, newSubnetConfig *controllers.PodSubnetAnno
		var appKind string
		var app metav1.Object
		var podSelector map[string]string
		var oldAppReplicas, newAppReplicas int

		switch newObject := newObj.(type) {
		case *appsv1.Deployment:
			appKind = constant.OwnerDeployment
			logPrefix := fmt.Sprintf("application '%s/%s/%s'", appKind, newObject.Namespace, newObject.Name)

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("%s is HostNetwork mode, we would not create or scale IPPool for it", logPrefix)
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not create or scale IPPool for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(newObject.Spec.Template.Annotations, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to get %s subnet configuration, error: %v", logPrefix, err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Sugar().Debugf("%s will use default IPAM mode with no subnet annotation", logPrefix)

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
				if nil != err {
					log.Sugar().Errorf("failed try to clean up %s legacy IPPools, error: %v", logPrefix, err)
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

			if oldObj != nil {
				oldDeployment := oldObj.(*appsv1.Deployment)
				oldAppReplicas = controllers.GetAppReplicas(oldDeployment.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(oldDeployment.Spec.Template.Annotations, oldAppReplicas)
				if nil != err {
					return fmt.Errorf("failed to get old %s subnet configuration, error: %v", logPrefix, err)
				}
			}

		case *appsv1.ReplicaSet:
			appKind = constant.OwnerReplicaSet
			logPrefix := fmt.Sprintf("application '%s/%s/%s'", appKind, newObject.Namespace, newObject.Name)

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("%s is HostNetwork mode, we would not create or scale IPPool for it", logPrefix)
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not create or scale IPPool for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(newObject.Spec.Template.Annotations, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to get %s subnet configuration, error: %v", logPrefix, err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Sugar().Debugf("%s will use default IPAM mode with no subnet annotation", logPrefix)

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
				if nil != err {
					log.Sugar().Errorf("failed try to clean up %s legacy IPPools, error: %v", logPrefix, err)
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

			if oldObj != nil {
				oldReplicaSet := oldObj.(*appsv1.ReplicaSet)
				oldAppReplicas = controllers.GetAppReplicas(oldReplicaSet.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(oldReplicaSet.Spec.Template.Annotations, oldAppReplicas)
				if nil != err {
					return fmt.Errorf("failed to get old %s subnet configuration, error: %v", logPrefix, err)
				}
			}

		case *appsv1.StatefulSet:
			appKind = constant.OwnerStatefulSet
			logPrefix := fmt.Sprintf("application '%s/%s/%s'", appKind, newObject.Namespace, newObject.Name)

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("%s is HostNetwork mode, we would not create or scale IPPool for it", logPrefix)
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not create or scale IPPool for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = controllers.GetAppReplicas(newObject.Spec.Replicas)
			newSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(newObject.Spec.Template.Annotations, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to get %s subnet configuration, error: %v", logPrefix, err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Sugar().Debugf("%s will use default IPAM mode with no subnet annotation", logPrefix)

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
				if nil != err {
					log.Sugar().Errorf("failed try to clean up %s legacy IPPools, error: %v", logPrefix, err)
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

			if oldObj != nil {
				oldStatefulSet := oldObj.(*appsv1.StatefulSet)
				oldAppReplicas = controllers.GetAppReplicas(oldStatefulSet.Spec.Replicas)
				oldSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(oldStatefulSet.Spec.Template.Annotations, oldAppReplicas)
				if nil != err {
					return fmt.Errorf("failed to get old %s subnet configuration, error: %v", logPrefix, err)
				}
			}

		case *batchv1.Job:
			appKind = constant.OwnerJob
			logPrefix := fmt.Sprintf("application '%s/%s/%s'", appKind, newObject.Namespace, newObject.Name)

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("%s is HostNetwork mode, we would not create or scale IPPool for it", logPrefix)
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not create or scale IPPool for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = calculateJobPodNum(newObject.Spec.Parallelism, newObject.Spec.Completions)
			newSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(newObject.Spec.Template.Annotations, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to get %s subnet configuration, error: %v", logPrefix, err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Sugar().Debugf("%s will use default IPAM mode with no subnet annotation", logPrefix)

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
				if nil != err {
					log.Sugar().Errorf("failed try to clean up %s legacy IPPools, error: %v", logPrefix, err)
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

			if oldObj != nil {
				oldJob := oldObj.(*batchv1.Job)
				oldAppReplicas = calculateJobPodNum(oldJob.Spec.Parallelism, oldJob.Spec.Completions)
				oldSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(oldJob.Spec.Template.Annotations, oldAppReplicas)
				if nil != err {
					return fmt.Errorf("failed to get old %s subnet configuration, error: %v", logPrefix, err)
				}
			}

		case *batchv1.CronJob:
			appKind = constant.OwnerCronJob
			logPrefix := fmt.Sprintf("application '%s/%s/%s'", appKind, newObject.Namespace, newObject.Name)

			// no need reconcile for HostNetwork application
			if newObject.Spec.JobTemplate.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("%s is HostNetwork mode, we would not create or scale IPPool for it", logPrefix)
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not create or scale IPPool for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = calculateJobPodNum(newObject.Spec.JobTemplate.Spec.Parallelism, newObject.Spec.JobTemplate.Spec.Completions)
			newSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(newObject.Spec.JobTemplate.Spec.Template.Annotations, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to get %s subnet configuration, error: %v", logPrefix, err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Sugar().Debugf("%s will use default IPAM mode with no subnet annotation", logPrefix)

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
				if nil != err {
					log.Sugar().Errorf("failed try to clean up %s legacy IPPools, error: %v", logPrefix, err)
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.JobTemplate.Spec.Selector.MatchLabels

			if oldObj != nil {
				oldCronJob := oldObj.(*batchv1.CronJob)
				oldAppReplicas = calculateJobPodNum(oldCronJob.Spec.JobTemplate.Spec.Parallelism, oldCronJob.Spec.JobTemplate.Spec.Completions)
				oldSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(oldCronJob.Spec.JobTemplate.Spec.Template.Annotations, oldAppReplicas)
				if nil != err {
					return fmt.Errorf("failed to get old %s subnet configuration, error: %v", logPrefix, err)
				}
			}

		case *appsv1.DaemonSet:
			appKind = constant.OwnerDaemonSet
			logPrefix := fmt.Sprintf("application '%s/%s/%s'", appKind, newObject.Namespace, newObject.Name)

			// no need reconcile for HostNetwork application
			if newObject.Spec.Template.Spec.HostNetwork {
				log.Sugar().Debugf("%s is HostNetwork mode, we would not create or scale IPPool for it", logPrefix)
				return nil
			}

			// check the app whether is the top controller or not
			owner := metav1.GetControllerOf(newObject)
			if owner != nil {
				log.Sugar().Debugf("%s has a owner '%s/%s', we would not create or scale IPPool for it", logPrefix, owner.Kind, owner.Name)
				return nil
			}

			newAppReplicas = int(newObject.Status.DesiredNumberScheduled)
			newSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(newObject.Spec.Template.Annotations, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to get %s subnet configuration, error: %v", logPrefix, err)
			}

			// default IPAM mode
			if newSubnetConfig == nil {
				log.Sugar().Debugf("%s will use default IPAM mode with no subnet annotation", logPrefix)

				// if one application used to have subnet feature but discard it later, we should also clean up the legacy IPPools
				err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, newObject)
				if nil != err {
					log.Sugar().Errorf("failed try to clean up %s legacy IPPools, error: %v", logPrefix, err)
				}

				return nil
			}

			app = newObject.DeepCopy()
			podSelector = newObject.Spec.Selector.MatchLabels

			if oldObj != nil {
				oldDaemonSet := oldObj.(*appsv1.DaemonSet)
				oldAppReplicas = int(oldDaemonSet.Status.DesiredNumberScheduled)
				oldSubnetConfig, err = controllers.GetSubnetConfigFromPodAnno(oldDaemonSet.Spec.Template.Annotations, oldAppReplicas)
				if nil != err {
					return fmt.Errorf("failed to get old %s subnet configuration, error: %v", logPrefix, err)
				}
			}

		default:
			return fmt.Errorf("unrecognized application: %+v", newObj)
		}

		log = logger.With(zap.String(appKind, fmt.Sprintf("%s/%s", app.GetNamespace(), app.GetName())))

		// check the difference between the two object and choose to reconcile or not
		if sm.hasSubnetConfigChanged(ctx, oldSubnetConfig, newSubnetConfig, oldAppReplicas, newAppReplicas, appKind, app, log) {
			log.Debug("Going to create IPPool or check whether to scale IPPool or not")
			err = sm.createOrScaleIPPool(ctx, *newSubnetConfig, appKind, app, podSelector, newAppReplicas)
			if nil != err {
				return fmt.Errorf("failed to create or scale IPPool, error: %v", err)
			}
		}

		return nil
	}
}

// createOrScaleIPPool try to create an IPPool or check whether to scale the existed IPPool or not with the give SpiderSubnet configuration
func (sm *subnetManager) createOrScaleIPPool(ctx context.Context, podSubnetConfig controllers.PodSubnetAnno, appKind string, app metav1.Object, podSelector map[string]string, appReplicas int) error {
	// retrieve application pools
	f := func(ctx context.Context, pools []*spiderpoolv1.SpiderIPPool, subnetMgrName string, ipVersion types.IPVersion) error {
		var ipNum int
		if podSubnetConfig.FlexibleIPNum != nil {
			ipNum = appReplicas + *(podSubnetConfig.FlexibleIPNum)
		} else {
			ipNum = podSubnetConfig.AssignIPNum
		}

		// verify whether the pool IPs need to be expanded or not
		if len(pools) == 0 {
			// create IPPool when the subnet manager was specified
			err := sm.AllocateIPPool(ctx, subnetMgrName, appKind, app, podSelector, ipNum, ipVersion, podSubnetConfig.ReclaimIPPool)
			if nil != err {
				return err
			}
		} else if len(pools) == 1 {
			err := sm.CheckScaleIPPool(ctx, pools[0], subnetMgrName, ipNum)
			if nil != err {
				return err
			}
		} else {
			return fmt.Errorf("it's invalid for '%s/%s/%s' corresponding SpiderSubnet '%s' owns multiple IPPools '%v' for one specify application",
				appKind, app.GetNamespace(), app.GetName(), subnetMgrName, pools)
		}

		return nil
	}

	if len(podSubnetConfig.SubnetManagerV4) != 0 {
		v4Pools, err := sm.RetrieveIPPoolsByAppUID(ctx, app.GetUID(), client.MatchingLabels{
			constant.LabelIPPoolOwnerSpiderSubnet: podSubnetConfig.SubnetManagerV4,
			constant.LabelIPPoolOwnerApplication:  controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
			constant.LabelIPPoolVersion:           constant.LabelIPPoolVersionV4,
		})
		if nil != err {
			return err
		}

		err = f(ctx, v4Pools, podSubnetConfig.SubnetManagerV4, constant.IPv4)
		if nil != err {
			return err
		}
	}

	if len(podSubnetConfig.SubnetManagerV6) != 0 {
		v6Pools, err := sm.RetrieveIPPoolsByAppUID(ctx, app.GetUID(), client.MatchingLabels{
			constant.LabelIPPoolOwnerSpiderSubnet: podSubnetConfig.SubnetManagerV6,
			constant.LabelIPPoolOwnerApplication:  controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
			constant.LabelIPPoolVersion:           constant.LabelIPPoolVersionV6,
		})
		if nil != err {
			return err
		}

		err = f(ctx, v6Pools, podSubnetConfig.SubnetManagerV6, constant.IPv6)
		if nil != err {
			return err
		}
	}

	return nil
}

// hasSubnetConfigChanged checks whether application subnet configuration changed and the application replicas changed or not.
// The second parameter newSubnetConfig must not be nil.
func (sm *subnetManager) hasSubnetConfigChanged(ctx context.Context, oldSubnetConfig, newSubnetConfig *controllers.PodSubnetAnno, oldAppReplicas, newAppReplicas int,
	appKind string, app metav1.Object, log *zap.Logger) bool {
	// go to reconcile directly with new application
	if oldSubnetConfig == nil {
		return true
	}

	var isChanged bool
	if reflect.DeepEqual(oldSubnetConfig, newSubnetConfig) {
		if oldAppReplicas != newAppReplicas {
			isChanged = true
			log.Sugar().Debugf("new application changed its replicas from '%d' to '%d'", oldAppReplicas, newAppReplicas)
		}
	} else {
		isChanged = true
		log.Sugar().Debugf("new application changed SpiderSubnet configuration, the old one is '%v' and the new one '%v'", oldSubnetConfig, newSubnetConfig)

		if oldSubnetConfig.SubnetManagerV4 != "" && newSubnetConfig.SubnetManagerV4 != "" && oldSubnetConfig.SubnetManagerV4 != newSubnetConfig.SubnetManagerV4 {
			log.Sugar().Warnf("change SpiderSubnet V4 from '%s' to '%s'", oldSubnetConfig.SubnetManagerV4, newSubnetConfig.SubnetManagerV4)

			// we should clean up the legacy IPPools once changed the SpiderSubnet
			if err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, app, client.MatchingLabels{
				constant.LabelIPPoolOwnerSpiderSubnet: oldSubnetConfig.SubnetManagerV4,
			}); err != nil {
				log.Sugar().Errorf("failed to clean up SpiderSubnet '%s' legacy V4 IPPools, error: %v", oldSubnetConfig.SubnetManagerV4, err)
			}
		}

		if oldSubnetConfig.SubnetManagerV6 != "" && newSubnetConfig.SubnetManagerV6 != "" && oldSubnetConfig.SubnetManagerV6 != newSubnetConfig.SubnetManagerV6 {
			log.Sugar().Warnf("change SpiderSubnet V6 from '%s' to '%s'", oldSubnetConfig.SubnetManagerV6, newSubnetConfig.SubnetManagerV6)

			// we should clean up the legacy IPPools once changed the SpiderSubnet
			if err := sm.tryToCleanUpLegacyIPPools(ctx, appKind, app, client.MatchingLabels{
				constant.LabelIPPoolOwnerSpiderSubnet: oldSubnetConfig.SubnetManagerV6,
			}); err != nil {
				log.Sugar().Errorf("failed to clean up SpiderSubnet '%s' legacy V6 IPPools, error: %v", oldSubnetConfig.SubnetManagerV6, err)
			}
		}
	}

	return isChanged
}

// calculateJobPodNum will calculate the job replicas
// once Parallelism and Completions are unset, the API-server will set them to 1
// reference: https://kubernetes.io/docs/concepts/workloads/controllers/job/
func calculateJobPodNum(jobSpecParallelism, jobSpecCompletions *int32) int {
	switch {
	case jobSpecParallelism != nil && jobSpecCompletions == nil:
		// parallel Jobs with a work queue
		if *jobSpecParallelism == 0 {
			return 1
		} else {
			// ignore negative integer, cause API-server will refuse the job creation
			return int(*jobSpecParallelism)
		}

	case jobSpecParallelism == nil && jobSpecCompletions != nil:
		// non-parallel Jobs
		if *jobSpecCompletions == 0 {
			return 1
		} else {
			// ignore negative integer, cause API-server will refuse the job creation
			return int(*jobSpecCompletions)
		}

	case jobSpecParallelism != nil && jobSpecCompletions != nil:
		// parallel Jobs with a fixed completion count
		if *jobSpecCompletions == 0 {
			return 1
		} else {
			// ignore negative integer, cause API-server will refuse the job creation
			return int(*jobSpecCompletions)
		}
	}

	return 1
}

// newCleanUpApp will return a function that clean up the application SpiderSubnet legacies (such as: the before created IPPools)
func (sm *subnetManager) newCleanUpApp() controllers.CleanUpAPPInformersFunc {
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
		return sm.tryToCleanUpLegacyIPPools(ctx, appKind, app)
	}
}

func (sm *subnetManager) tryToCleanUpLegacyIPPools(ctx context.Context, appKind string, app metav1.Object, labels ...client.MatchingLabels) error {
	log := logutils.FromContext(ctx).With(zap.String(appKind, fmt.Sprintf("%s/%s", app.GetNamespace(), app.GetName())))

	matchLabel := client.MatchingLabels{
		constant.LabelReclaimIPPool: constant.LabelAllowReclaimIPPool,
	}

	for _, label := range labels {
		for k, v := range label {
			matchLabel[k] = v
		}
	}

	pools, err := sm.RetrieveIPPoolsByAppUID(ctx, app.GetUID(), matchLabel)
	if nil != err {
		return fmt.Errorf("failed to retrieve '%s/%s/%s' IPPools, error: %v", appKind, app.GetNamespace(), app.GetName(), err)
	}

	if len(pools) != 0 {
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

		for i := range pools {
			wg.Add(1)
			go deletePool(pools[i])
		}

		wg.Wait()
	}

	return nil
}
