// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
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
		return fmt.Errorf("failed to get Pod %s/%s: %v", *delArgs.PodNamespace, *delArgs.PodName, err)
	}

	podStatus, _ := podmanager.CheckPodStatus(pod)
	if podStatus == constant.PodRunning {
		logger.Info("Pod is still running, ignore release for reuse IP allocation")
		return nil
	}

	// If Pod still exists, change the timeout of ctx to be consistent with
	// the deletion grace period of Pod. After this time, all IP allocation
	// recycling should be completed by GC instead of CNI DEL.
	//
	// But if Pod no longer exists, CNI DEL is still called (CNI DEL may be
	// called multiple times according to the CNI Specification), continue
	// to use the original ctx of OAI UNIX client (default 30s).
	if podStatus != constant.PodUnknown {
		*delArgs.PodUID = string(pod.UID)

		timeoutSec := *pod.DeletionGracePeriodSeconds - 5
		if timeoutSec < 0 {
			timeoutSec = 5
		}

		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
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
	endpoint, err := i.endpointManager.GetEndpointByName(ctx, *delArgs.PodNamespace, *delArgs.PodName, constant.IgnoreCache)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Endpoint does not exist, ignore release")
			return nil
		}
		return fmt.Errorf("failed to get Endpoint %s/%s: %v", *delArgs.PodNamespace, *delArgs.PodName, err)
	}

	if err := i.releaseForAllNICs(ctx, *delArgs.PodUID, *delArgs.IfName, endpoint); err != nil {
		return err
	}
	logger.Info("Succeed to release")

	return nil
}

func (i *ipam) releaseForAllNICs(ctx context.Context, uid, nic string, endpoint *spiderpoolv1.SpiderEndpoint) error {
	logger := logutils.FromContext(ctx)

	// Check whether an StatefulSet needs to release its currently allocated IP addresses.
	// It is discussed in https://github.com/spidernet-io/spiderpool/issues/1045
	if i.config.EnableStatefulSet && endpoint.Status.OwnerControllerType == constant.KindStatefulSet {
		valid, err := i.stsManager.IsValidStatefulSetPod(ctx, endpoint.Namespace, endpoint.Name, endpoint.Status.OwnerControllerType)
		if nil != err {
			return fmt.Errorf("failed to check pod %s/%s whether is a valid StatefulSet pod: %v", endpoint.Namespace, endpoint.Name, err)
		}

		if valid {
			logger.Info("There is no need to release the IP allocation of StatefulSet")
			return nil
		}

		if err := i.endpointManager.DeleteEndpoint(ctx, endpoint); err != nil {
			return err
		}
	}

	allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic, endpoint)
	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}

	logger.Sugar().Infof("Release IP allocation details: %+v", allocation.IPs)
	if err := i.release(ctx, allocation.UID, allocation.IPs); err != nil {
		return err
	}

	logger.Info("Clean Endpoint")
	if err := i.endpointManager.RemoveFinalizer(ctx, endpoint.Namespace, endpoint.Name); err != nil {
		return fmt.Errorf("failed to clean Endpoint: %v", err)
	}

	return nil
}

func (i *ipam) release(ctx context.Context, uid string, details []spiderpoolv1.IPAllocationDetail) error {
	logger := logutils.FromContext(ctx)

	pius := convert.GroupIPAllocationDetails(uid, details)
	tickets := pius.Pools()
	timeRecorder := metric.NewTimeRecorder()
	if err := i.ipamLimiter.AcquireTicket(ctx, tickets...); err != nil {
		return fmt.Errorf("failed to queue correctly: %v", err)
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
