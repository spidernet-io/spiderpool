// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/spidernet-io/spiderpool/pkg/constant"
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
			wep, err := s.wepMgr.GetEndpointByName(ctx, podCache.Namespace, podCache.PodName)
			if nil != err {
				if apierrors.IsNotFound(err) {
					loggerReleaseIP.Sugar().Infof("wep '%s/%s' not found, maybe already cleaned", podCache.Namespace, podCache.PodName)
					continue
				}

				loggerReleaseIP.Sugar().Errorf("failed to get wep '%s/%s', error: %v", podCache.Namespace, podCache.PodName, err)
				continue
			}

			// when we received one gc signal which contains a pod name and its namespace,
			// we need to search the pod corresponding workloadenpoint to get the used history IPs.
			podUsedIPs, err := s.wepMgr.ListAllHistoricalIPs(ctx, podCache.Namespace, podCache.PodName)
			if apierrors.IsNotFound(err) {
				loggerReleaseIP.Sugar().Warnf("wep '%s/%s' not found, maybe already cleaned by execScanAll",
					podCache.Namespace, podCache.PodName)
				continue
			}

			if nil != err {
				loggerReleaseIP.Sugar().Errorf("failed to list wep '%s/%s' all historical IPs: %v",
					podCache.Namespace, podCache.PodName, err)
				continue
			}

			// release pod used history IPs
			for poolName, ips := range podUsedIPs {
				loggerReleaseIP.Sugar().Debugf("pod '%s/%s used IPs '%+v' from pool '%s', begin to release",
					podCache.Namespace, podCache.PodName, ips, poolName)

				err = s.ippoolMgr.ReleaseIP(ctx, poolName, ips)
				if apierrors.IsNotFound(err) {
					// If we can not find the IP object, which means cmdDel already released the IP
					continue
				}

				if nil != err {
					logger.Sugar().Errorf("failed to release pool '%s' IPs '%+v' in wep '%s/%s', error: %w",
						poolName, ips, podCache.Namespace, podCache.PodName, err)
					continue
				}
				// TODO (Icarus9913): metric
			}

			loggerReleaseIP.Sugar().Infof("release IPPoolIP task '%+v' successfully", podCache)

			// remove wep finalizer
			err = s.wepMgr.RemoveFinalizer(ctx, podCache.Namespace, podCache.PodName)
			if nil != err {
				loggerReleaseIP.Sugar().Errorf("failed to remove wep '%s/%s' finalizer, error: '%v'",
					podCache.Namespace, podCache.PodName, err)
				continue
			}

			// delete StatefulSet wep (other controller wep has OwnerReference, its lifecycle is same with pod)
			if wep.Status.OwnerControllerType == constant.OwnerStatefulSet {
				err = s.wepMgr.Delete(ctx, wep)
				if nil != err {
					loggerReleaseIP.Sugar().Errorf("failed to delete StatefulSet wep '%s/%s', error: '%v'",
						podCache.Namespace, podCache.PodName, err)
					continue
				}
			}

			loggerReleaseIP.Sugar().Debugf("remove wep '%v/%v' finalizer '%s' successfully",
				podCache.Namespace, podCache.PodName, constant.SpiderFinalizer)

		case <-ctx.Done():
			loggerReleaseIP.Info("receive ctx done, stop running releaseIPPoolIPExecutor")
			return
		}
	}
}
