// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to release")

	pod, err := i.podManager.GetPodByName(ctx, *delArgs.PodNamespace, *delArgs.PodName, constant.IgnoreCache)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get Pod %s/%s: %w", *delArgs.PodNamespace, *delArgs.PodName, err)
	}

	isAlive := podmanager.IsPodAlive(pod)
	if isAlive {
		logger.Info("Pod is still alive, ignore release for reuse IP allocation")
		return nil
	}

	// If Pod still exists, change the timeout of ctx to be consistent with
	// the deletion grace period of Pod. After this time, all IP allocation
	// recycling should be completed by GC instead of CNI DEL.
	//
	// But if Pod no longer exists, CNI DEL is still called (CNI DEL may be
	// called multiple times according to the CNI Specification), continue
	// to use the original ctx of OAI UNIX client (default 30s).
	if pod != nil {
		*delArgs.PodUID = string(pod.UID)
		logger = logger.With(zap.String("PodUID", *delArgs.PodUID))

		var timeoutSec int64
		if pod.DeletionGracePeriodSeconds != nil {
			timeoutSec = *pod.DeletionGracePeriodSeconds - 5
		}
		if timeoutSec <= 0 {
			timeoutSec = 5
		}

		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(logutils.IntoContext(ctx, logger), time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	// Give priority to the UID of Pod, and then consider ENV K8S_POD_UID in
	// CNI_ARGS, because some CRIs do not set K8S_POD_UID (such as dockershim).
	// If do not get UID through all the above channels, skip CNI DEL and hand
	// over the task of IP allocation recycling to GC.
	if len(*delArgs.PodUID) == 0 {
		logger.Info("No way to get Pod UID, skip release")
		return nil
	}

	defer i.failure.rmFailureIPs(*delArgs.PodUID)

	endpointName := *delArgs.PodName
	// if the kubevirt vm pod is not exist, the gc will release the legacy IP later
	if pod != nil {
		ownerReference := metav1.GetControllerOf(pod)
		if ownerReference != nil && i.config.EnableKubevirtStaticIP &&
			ownerReference.APIVersion == kubevirtv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindKubevirtVMI {
			endpointName = ownerReference.Name
		}
	}
	endpoint, err := i.endpointManager.GetEndpointByName(ctx, *delArgs.PodNamespace, endpointName, constant.IgnoreCache)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Endpoint does not exist, ignore release")
			return nil
		}
		return fmt.Errorf("failed to get Endpoint %s/%s: %w", *delArgs.PodNamespace, *delArgs.PodName, err)
	}

	if err := i.releaseForAllNICs(ctx, *delArgs.PodUID, *delArgs.IfName, endpoint); err != nil {
		return err
	}
	logger.Info("Succeed to release")

	return nil
}

func (i *ipam) releaseForAllNICs(ctx context.Context, uid, nic string, endpoint *spiderpoolv2beta1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)

	// Check whether an StatefulSet needs to release its currently allocated IP addresses.
	// It is discussed in https://github.com/spidernet-io/spiderpool/issues/1045
	if i.config.EnableStatefulSet && endpoint.Status.OwnerControllerType == constant.KindStatefulSet {
		isValidStatefulSetPod, err := i.stsManager.IsValidStatefulSetPod(ctx, endpoint.Namespace, endpoint.Name, endpoint.Status.OwnerControllerType)
		if nil != err {
			return fmt.Errorf("failed to check pod '%s/%s' whether is a valid StatefulSet pod, error: %w", endpoint.Namespace, endpoint.Name, err)
		}

		if isValidStatefulSetPod {
			logger.Info("There is no need to release the IP allocation of StatefulSet")
			return nil
		}

		if err := i.endpointManager.DeleteEndpoint(ctx, endpoint); err != nil {
			return err
		}
	}

	// Check whether the kubevirt VM pod needs to keep its IP allocation.
	if i.config.EnableKubevirtStaticIP && endpoint.Status.OwnerControllerType == constant.KindKubevirtVMI {
		isValidVMPod, err := i.kubevirtManager.IsValidVMPod(ctx, endpoint.Namespace, endpoint.Status.OwnerControllerType, endpoint.Status.OwnerControllerName)
		if nil != err {
			return fmt.Errorf("failed to check pod '%s/%s' whether is a valid kubevirt VM pod, error: %w", endpoint.Namespace, endpoint.Name, err)
		}

		if isValidVMPod {
			logger.Info("There is no need to release the IP allocation of kubevirt VM")
			return nil
		}

		if err := i.endpointManager.DeleteEndpoint(ctx, endpoint); err != nil {
			return err
		}
	}

	allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic, endpoint, false)
	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}

	logger.Sugar().Infof("Release IP allocation details: %v", allocation.IPs)
	if err := i.release(ctx, allocation.UID, allocation.IPs); err != nil {
		return err
	}

	logger.Info("Clean Endpoint")
	if err := i.endpointManager.RemoveFinalizer(ctx, endpoint); err != nil {
		return fmt.Errorf("failed to clean Endpoint: %w", err)
	}

	return nil
}

func (i *ipam) release(ctx context.Context, uid string, details []spiderpoolv2beta1.IPAllocationDetail) error {
	logger := logutils.FromContext(ctx)

	pius := convert.GroupIPAllocationDetails(uid, details)
	tickets := pius.Pools()
	timeRecorder := metric.NewTimeRecorder()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return fmt.Errorf("failed to queue correctly: %w", err)
	}
	defer i.ipamLimiter.ReleaseTicket(ctx, tickets...)

	// Record the metric of queuing time for release.
	metric.IPAMDurationConstruct.RecordIPAMReleaseLimitDuration(ctx, timeRecorder.SinceInSeconds())

	errCh := make(chan error, len(pius))
	wg := sync.WaitGroup{}
	wg.Add(len(pius))

	for p, ius := range pius {
		go func(poolName string, ipAndUIDs []types.IPAndUID) {
			defer wg.Done()

			if err := i.ipPoolManager.ReleaseIP(ctx, poolName, ipAndUIDs); err != nil {
				logger.Warn(err.Error())
				errCh <- err
				return
			}
			logger.Sugar().Infof("Succeed to release IP addresses %+v from IPPool %s", ipAndUIDs, poolName)
		}(p, ius)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to release all allocated IP addresses %+v: %w", pius, utilerrors.NewAggregate(errs))
	}

	return nil
}

// ReleaseIPs will release the given IP corresponding NIC whole IPs,
// and we will release the SpiderEndpoint recorded IPs first and release the SpiderIPPool recorded IPs later.
func (i *ipam) ReleaseIPs(ctx context.Context, delArgs *models.IpamBatchDelArgs) (err error) {
	log := logutils.FromContext(ctx)

	var pod *corev1.Pod
	// we need to have the Pod UID for IP release operation
	if len(*delArgs.PodUID) == 0 {
		pod, err = i.podManager.GetPodByName(ctx, *delArgs.PodNamespace, *delArgs.PodName, constant.IgnoreCache)
		if nil != err {
			if apierrors.IsNotFound(err) {
				log.Sugar().Warnf("Pod '%s/%s' is not found, skip to release IPs due to no Pod UID", *delArgs.PodNamespace, *delArgs.PodName)
				return nil
			}
			return fmt.Errorf("failed to get Pod '%s/%s', error: %w", *delArgs.PodNamespace, *delArgs.PodName, err)
		}
		// set Pod UID to parameter
		*delArgs.PodUID = string(pod.UID)
	}

	// check for release conflict IPs
	if delArgs.IsReleaseConflictIPs {
		if i.config.EnableReleaseConflictIPsForStateless {
			if pod == nil {
				pod, err = i.podManager.GetPodByName(ctx, *delArgs.PodNamespace, *delArgs.PodName, constant.IgnoreCache)
				if nil != err {
					return fmt.Errorf("failed to get Pod '%s/%s', error: %w", *delArgs.PodNamespace, *delArgs.PodName, err)
				}
			}

			podTopController, err := i.podManager.GetPodTopController(ctx, pod)
			if nil != err {
				return fmt.Errorf("failed to get the top controller of the Pod %s/%s, error: %w", pod.Namespace, pod.Name, err)
			}

			// do not release conflict IPs for stateful Pod
			if (i.config.EnableStatefulSet && podTopController.APIVersion == appsv1.SchemeGroupVersion.String() && podTopController.Kind == constant.KindStatefulSet) ||
				(i.config.EnableKubevirtStaticIP && podTopController.APIVersion == kubevirtv1.SchemeGroupVersion.String() && podTopController.Kind == constant.KindKubevirtVMI) {
				log.Warn("no need to release conflict IPs for stateful Pod")
				// return error for 'IsReleaseConflictIPs'
				return constant.ErrForbidReleasingStatefulWorkload
			}
		} else {
			log.Warn("EnableReleaseConflictIPsForStateless is disabled, skip to release IPs")
			// return error for 'IsReleaseConflictIPs'
			return constant.ErrForbidReleasingStatelessWorkload
		}
	}

	// release stateless workload SpiderEndpoint IPs
	endpoint, err := i.endpointManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName, constant.IgnoreCache)
	if nil != err {
		return fmt.Errorf("failed to get SpiderEndpoint '%s/%s', error: %w", *delArgs.PodNamespace, *delArgs.PodName, err)
	}
	recordedIPAllocationDetails, err := i.endpointManager.ReleaseEndpointIPs(ctx, endpoint, *delArgs.PodUID)
	if nil != err {
		return fmt.Errorf("failed to release SpiderEndpoint IPs, error: %w", err)
	}

	// release IPPool IPs
	if len(recordedIPAllocationDetails) != 0 {
		log.Sugar().Infof("try to release IPs: %v", recordedIPAllocationDetails)
		err := i.release(ctx, *delArgs.PodUID, recordedIPAllocationDetails)
		if nil != err {
			return err
		}
	}

	return nil
}
