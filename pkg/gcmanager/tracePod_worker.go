// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	metrics "github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

// tracePodWorker will circle traverse PodEntry database
func (s *SpiderGC) tracePodWorker(ctx context.Context) {
	logger.Info("starting trace pod worker")

	for {
		select {
		case <-ctx.Done():
			logger.Warn("trace pod worker received ctx done, stop tracing")
			return
		default:
			podEntryList := s.PodDB.ListAllPodEntries()
			for _, podEntry := range podEntryList {
				podCache := podEntry
				s.handlePodEntryForTracingTimeOut(&podCache)
			}

			time.Sleep(time.Duration(s.gcConfig.TracePodGapDuration) * time.Second)
		}
	}
}

// handlePodEntryForTracingTimeOut check the given podEntry whether out of time. If so, just send a signal to execute gc
func (s *SpiderGC) handlePodEntryForTracingTimeOut(podEntry *PodEntry) {
	if podEntry.TracingStopTime.IsZero() {
		logger.Sugar().Warnf("unknown podEntry: %+v", podEntry)
		return
	} else {
		if time.Now().UTC().After(podEntry.TracingStopTime) {
			logger.With(zap.Any("podEntry tracing-reason", podEntry.PodTracingReason)).
				Sugar().Infof("pod '%s/%s' is out of time, begin to gc IP", podEntry.Namespace, podEntry.PodName)
		} else {
			// not time out
			return
		}
	}

	select {
	case s.gcIPPoolIPSignal <- podEntry:
		logger.Sugar().Debugf("sending signal to gc pod '%s/%s' IP", podEntry.Namespace, podEntry.PodName)
		s.PodDB.DeletePodEntry(podEntry.Namespace, podEntry.PodName)

	case <-time.After(time.Duration(s.gcConfig.GCSignalTimeoutDuration) * time.Second):
		logger.Sugar().Errorf("failed to gc IP, gcSignal:len=%d, event:'%s/%s' will be dropped", len(s.gcSignal), podEntry.Namespace, podEntry.PodName)
	}
}

// releaseIPPoolIPExecutor receive signals to execute gc IP
func (s *SpiderGC) releaseIPPoolIPExecutor(ctx context.Context, workerIndex int) {
	loggerReleaseIP := logger.With(zap.Any("IPPoolIP_Worker", workerIndex))
	loggerReleaseIP.Info("Starting running 'releaseIPPoolIPExecutor'")

	for {
		select {
		case podCache := <-s.gcIPPoolIPSignal:
			endpoint, err := s.wepMgr.GetEndpointByName(ctx, podCache.Namespace, podCache.PodName)
			if nil != err {
				if apierrors.IsNotFound(err) {
					loggerReleaseIP.Sugar().Infof("SpiderEndpoint '%s/%s' not found, maybe already cleaned by ScanAll", podCache.Namespace, podCache.PodName)
					continue
				}

				loggerReleaseIP.Sugar().Errorf("failed to get SpiderEndpoint '%s/%s', error: %v", podCache.Namespace, podCache.PodName, err)
				continue
			}

			// we need to gather the pod corresponding SpiderEndpoint to get the used history IPs.
			podUsedIPs := workloadendpointmanager.ListAllHistoricalIPs(endpoint)

			// release pod used history IPs
			for poolName, ips := range podUsedIPs {
				loggerReleaseIP.Sugar().Infof("pod '%s/%s used IPs '%+v' from pool '%s', begin to release",
					podCache.Namespace, podCache.PodName, ips, poolName)

				err = s.ippoolMgr.ReleaseIP(ctx, poolName, ips)
				if nil != err {
					metrics.IPGCFailureCounts.Add(ctx, 1)
					loggerReleaseIP.Sugar().Errorf("failed to release pool '%s' IPs '%+v' in wep '%s/%s', error: %v",
						poolName, ips, podCache.Namespace, podCache.PodName, err)
					continue
				}

				// metric
				metrics.IPGCTotalCounts.Add(ctx, 1)
			}

			loggerReleaseIP.Sugar().Infof("release IPPoolIP task '%+v' successfully", *podCache)

			// delete StatefulSet wep (other controller wep has OwnerReference, its lifecycle is same with pod)
			if endpoint.Status.OwnerControllerType == constant.OwnerStatefulSet {
				err = s.wepMgr.Delete(ctx, endpoint)
				if nil != err {
					loggerReleaseIP.Sugar().Errorf("failed to delete StatefulSet wep '%s/%s', error: '%v'",
						podCache.Namespace, podCache.PodName, err)
					continue
				}
			}

			err = s.wepMgr.RemoveFinalizer(ctx, podCache.Namespace, podCache.PodName)
			if nil != err {
				loggerReleaseIP.Sugar().Errorf("failed to remove wep '%s/%s' finalizer, error: '%v'",
					podCache.Namespace, podCache.PodName, err)
				continue
			}

			loggerReleaseIP.Sugar().Infof("remove wep '%s/%s' finalizer '%s' successfully",
				podCache.Namespace, podCache.PodName, constant.SpiderFinalizer)

		case <-ctx.Done():
			loggerReleaseIP.Info("receive ctx done, stop running releaseIPPoolIPExecutor")
			return
		}
	}
}
