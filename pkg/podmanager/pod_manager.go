// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"fmt"
	"os"

	init_cmd "github.com/spidernet-io/spiderpool/cmd/spiderpool-init/cmd"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	k8s_resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string, cached bool) (*corev1.Pod, error)
	ListPods(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.PodList, error)
	GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error)
	admission.CustomDefaulter
	admission.CustomValidator
}

type podManager struct {
	enableDra    bool
	client       client.Client
	apiReader    client.Reader
	SpiderClient crdclientset.Interface
}

var _ webhook.CustomValidator = &podManager{}

func NewPodManager(enableDra bool, client client.Client, apiReader client.Reader, mgr ctrl.Manager) (PodManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	spiderClient, err := crdclientset.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return nil, err
	}

	pm := &podManager{
		enableDra:    enableDra,
		client:       client,
		apiReader:    apiReader,
		SpiderClient: spiderClient,
	}

	if enableDra && mgr != nil {
		return pm, ctrl.NewWebhookManagedBy(mgr).
			For(&corev1.Pod{}).
			WithDefaulter(pm).
			Complete()
	}

	return pm, nil
}

func (pm *podManager) GetPodByName(ctx context.Context, namespace, podName string, cached bool) (*corev1.Pod, error) {
	reader := pm.apiReader
	if cached == constant.UseCache {
		reader = pm.client
	}

	var pod corev1.Pod
	if err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}

func (pm *podManager) ListPods(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.PodList, error) {
	reader := pm.apiReader
	if cached == constant.UseCache {
		reader = pm.client
	}

	var podList corev1.PodList
	if err := reader.List(ctx, &podList, opts...); err != nil {
		return nil, err
	}

	return &podList, nil
}

// GetPodTopController will find the pod top owner controller with the given pod.
// For example, once we create a deployment then it will create replicaset and the replicaset will create pods.
// So, the pods' top owner is deployment. That's what the method implements.
// Notice: if the application is a third party controller, the types.PodTopController property App would be nil!
func (pm *podManager) GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error) {
	logger := logutils.FromContext(ctx)

	var ownerErr = fmt.Errorf("failed to get pod '%s/%s' owner", pod.Namespace, pod.Name)

	podOwner := metav1.GetControllerOf(pod)
	if podOwner == nil {
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				// pod.APIVersion is empty string
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       constant.KindPod,
				Namespace:  pod.Namespace,
				Name:       pod.Name,
			},
			UID: pod.UID,
			APP: pod,
		}, nil
	}

	// third party controller
	if !slices.Contains(constant.K8sAPIVersions, podOwner.APIVersion) {
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: podOwner.APIVersion,
				Kind:       podOwner.Kind,
				Namespace:  pod.Namespace,
				Name:       podOwner.Name,
			},
			UID: podOwner.UID,
		}, nil
	}

	namespacedName := apitypes.NamespacedName{
		Namespace: pod.Namespace,
		Name:      podOwner.Name,
	}

	switch podOwner.Kind {
	case constant.KindReplicaSet:
		var replicaset appsv1.ReplicaSet
		err := pm.client.Get(ctx, namespacedName, &replicaset)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}

		replicasetOwner := metav1.GetControllerOf(&replicaset)
		if replicasetOwner != nil && replicasetOwner.APIVersion == appsv1.SchemeGroupVersion.String() && replicasetOwner.Kind == constant.KindDeployment {
			var deployment appsv1.Deployment
			err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: replicaset.Namespace, Name: replicasetOwner.Name}, &deployment)
			if nil != err {
				return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
			}
			return types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					// deployment.APIVersion is empty string
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       constant.KindDeployment,
					Namespace:  deployment.Namespace,
					Name:       deployment.Name,
				},
				UID: deployment.UID,
				APP: &deployment,
			}, nil
		}

		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindReplicaSet,
				Namespace:  replicaset.Namespace,
				Name:       replicaset.Name,
			},
			UID: replicaset.UID,
			APP: &replicaset,
		}, nil

	case constant.KindJob:
		var job batchv1.Job
		err := pm.client.Get(ctx, namespacedName, &job)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		jobOwner := metav1.GetControllerOf(&job)
		if jobOwner != nil && jobOwner.APIVersion == batchv1.SchemeGroupVersion.String() && jobOwner.Kind == constant.KindCronJob {
			var cronJob batchv1.CronJob
			err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: job.Namespace, Name: jobOwner.Name}, &cronJob)
			if nil != err {
				return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
			}
			return types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					APIVersion: batchv1.SchemeGroupVersion.String(),
					Kind:       constant.KindCronJob,
					Namespace:  cronJob.Namespace,
					Name:       cronJob.Name,
				},
				UID: cronJob.UID,
				APP: &cronJob,
			}, nil
		}

		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: batchv1.SchemeGroupVersion.String(),
				Kind:       constant.KindJob,
				Namespace:  job.Namespace,
				Name:       job.Name,
			},
			UID: job.UID,
			APP: &job,
		}, nil

	case constant.KindDaemonSet:
		var daemonSet appsv1.DaemonSet
		err := pm.client.Get(ctx, namespacedName, &daemonSet)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				// daemonSet.APIVersion is empty string
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindDaemonSet,
				Namespace:  daemonSet.Namespace,
				Name:       daemonSet.Name,
			},
			UID: daemonSet.UID,
			APP: &daemonSet,
		}, nil

	case constant.KindStatefulSet:
		var statefulSet appsv1.StatefulSet
		err := pm.client.Get(ctx, namespacedName, &statefulSet)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				// statefulSet.APIVersion is empty string
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindStatefulSet,
				Namespace:  statefulSet.Namespace,
				Name:       statefulSet.Name,
			},
			UID: statefulSet.UID,
			APP: &statefulSet,
		}, nil
	}

	logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", podOwner.Kind, pod.Namespace, pod.Name)
	return types.PodTopController{
		AppNamespacedName: types.AppNamespacedName{
			APIVersion: podOwner.APIVersion,
			Kind:       podOwner.Kind,
			Namespace:  pod.Namespace,
			Name:       podOwner.Name,
		},
		UID: podOwner.UID,
	}, nil
}

// Default implements admission.CustomDefaulter.
func (pw *podManager) Default(ctx context.Context, obj runtime.Object) error {
	// Avoids affecting the time of pod creation when dra is not enabled
	if !pw.enableDra {
		return nil
	}

	logger := logutils.FromContext(ctx)
	pod := obj.(*corev1.Pod)
	mutateLogger := logger.Named("Mutating").With(
		zap.String("Pod", pod.Name))
	mutateLogger.Sugar().Debugf("Request Pod: %+v", *pod)

	return pw.injectPodResources(logutils.IntoContext(ctx, mutateLogger), mutateLogger, pod)
}

func (pw *podManager) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (pw *podManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete will implement something just like kubernetes Foreground cascade deletion to delete the MultusConfig corresponding net-attach-def firstly
// Since the MultusConf doesn't have Finalizer, you could delete it as soon as possible and we can't filter it to delete the net-attach-def at first.
func (pw *podManager) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
func (pw *podManager) injectPodResources(ctx context.Context, l *zap.Logger, pod *corev1.Pod) error {
	if pod.Spec.ResourceClaims == nil {
		return nil
	}

	staticNics, err := pw.getStaticNicsFromSpiderClaimParameter(ctx, pod)
	if err != nil {
		l.Error(err.Error())
		return err
	}

	if len(staticNics) == 0 {
		l.Debug("spiderClaimParameter no staticNics configure, exit")
		return nil
	}

	resourceMap, err := pw.getResourceMapFromStaticNics(ctx, staticNics)
	if err != nil {
		l.Error("error get resourceMap for the staticNics", zap.Error(err))
		return err
	}

	if len(resourceMap) == 0 {
		l.Debug("staticNics no rdma resource claimed, exit")
		return nil
	}

	l.Info("find pod has dra claim with staticNics and rdma resources claim, try to inject rdma resource to pod resources")
	InjectRdmaResourceToPod(resourceMap, pod)
	l.Debug("Finish inject resource to pod", zap.Any("Pod", pod))
	return nil
}

func (pw *podManager) getStaticNicsFromSpiderClaimParameter(ctx context.Context, pod *corev1.Pod) ([]spiderpoolv2beta1.StaticNic, error) {
	for _, rc := range pod.Spec.ResourceClaims {
		if rc.Source.ResourceClaimTemplateName != nil {
			var rct resourcev1alpha2.ResourceClaimTemplate
			if err := pw.apiReader.Get(ctx, apitypes.NamespacedName{Namespace: pod.Namespace, Name: *rc.Source.ResourceClaimTemplateName}, &rct); err != nil {
				return nil, err
			}

			if rct.Spec.Spec.ResourceClassName == constant.DRADriverName && rct.Spec.Spec.ParametersRef.APIGroup == constant.SpiderpoolAPIGroup &&
				rct.Spec.Spec.ParametersRef.Kind == constant.KindSpiderClaimParameter {

				spc, err := pw.SpiderClient.SpiderpoolV2beta1().SpiderClaimParameters(pod.Namespace).Get(ctx, rct.Spec.Spec.ParametersRef.Name, metav1.GetOptions{})
				if err != nil {
					return nil, fmt.Errorf("failed to get spiderClaimParameter for pod %s/%s: %v", pod.Namespace, pod.Name, err)
				}
				return spc.Spec.StaticNics, nil
			}
		}
	}
	return []spiderpoolv2beta1.StaticNic{}, nil
}

func (pw *podManager) getResourceMapFromStaticNics(ctx context.Context, staticNics []spiderpoolv2beta1.StaticNic) (map[string]bool, error) {
	resourceMap := make(map[string]bool)
	for _, nic := range staticNics {
		if nic.Namespace == "" {
			nic.Namespace = os.Getenv(init_cmd.ENVNamespace)
		}

		smc, err := pw.SpiderClient.SpiderpoolV2beta1().SpiderMultusConfigs(nic.Namespace).Get(ctx, nic.MultusConfigName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get spiderMultusConfigs %s/%s: %v", nic.Namespace, nic.MultusConfigName, err)
		}

		resourceName := pw.resourceName(smc)
		if resourceName == "" {
			continue
		}

		if _, ok := resourceMap[resourceName]; !ok {
			resourceMap[resourceName] = false
		}
	}
	return resourceMap, nil
}

// resourceName return the resourceName for given spiderMultusConfig
func (pw *podManager) resourceName(smc *spiderpoolv2beta1.SpiderMultusConfig) string {
	switch *smc.Spec.CniType {
	case constant.MacvlanCNI:
		if smc.Spec.MacvlanConfig != nil && smc.Spec.MacvlanConfig.EnableRdma {
			return smc.Spec.MacvlanConfig.RdmaResourceName
		}
	case constant.IPVlanCNI:
		if smc.Spec.IPVlanConfig != nil && smc.Spec.IPVlanConfig.EnableRdma {
			return smc.Spec.IPVlanConfig.RdmaResourceName
		}
	case constant.SriovCNI:
		if smc.Spec.SriovConfig != nil {
			return smc.Spec.SriovConfig.ResourceName
		}
	case constant.IBSriovCNI:
		if smc.Spec.IbSriovConfig != nil {
			return smc.Spec.IbSriovConfig.ResourceName
		}
	}
	return ""
}

func InjectRdmaResourceToPod(resourceMap map[string]bool, pod *corev1.Pod) {
	for _, c := range pod.Spec.Containers {
		for resource := range resourceMap {
			if resourceMap[resource] {
				// the resource has found in pod, skip
				continue
			}

			// try to find the resource in container resources.requests
			if _, ok := c.Resources.Requests[corev1.ResourceName(resource)]; ok {
				resourceMap[resource] = true
			} else {
				if _, ok := c.Resources.Limits[corev1.ResourceName(resource)]; ok {
					resourceMap[resource] = true
				}
			}
		}
	}

	for resource, found := range resourceMap {
		if !found {
			// inject the resource to the pod.containers[0].resources.requests
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(resource)] = k8s_resource.MustParse("1")
		}
	}
}
