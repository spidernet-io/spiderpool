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

type gcIPPoolIPIdentify struct {
	PodName      string
	PodNamespace string
}

// tracePodWorker will circle traverse PodEntry database
func (s *SpiderGC) tracePodWorker() {
	logger.Info("starting trace pod worker")

	for {
		podEntryList := s.PodDB.ListAllPodEntries()
		for _, podEntry := range podEntryList {
			s.handlePodEntryForTracingTimeOut(&podEntry)
		}

		time.Sleep(s.gcConfig.TracePodGapDuration)
	}
}

// handlePodEntryForTracingTimeOut check the given podEntry whether out of time. If so, just send a signal to execute gc
func (s *SpiderGC) handlePodEntryForTracingTimeOut(podEntry *PodEntry) {
	if podEntry.TracingStopTime.IsZero() {
		logger.Sugar().Warnf("unknown podEntry: %+v", *podEntry)
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
	case s.gcIPPoolIPSignal <- gcIPPoolIPIdentify{PodName: podEntry.PodName, PodNamespace: podEntry.Namespace}:
		logger.Sugar().Debugf("sending signal to gc pod '%s/%s' IP", podEntry.Namespace, podEntry.PodName)
		s.PodDB.DeletePodEntry(podEntry.PodName, podEntry.Namespace)

	case <-time.After(s.gcConfig.GCSignalTimeoutDuration):
		logger.Sugar().Errorf("failed to gc IP, gcSignal:len=%d, event:'%+v' will be dropped", len(s.gcSignal),
			gcIPPoolIPIdentify{PodName: podEntry.PodName, PodNamespace: podEntry.Namespace})
	}
}

// releaseIPPoolIPExecutor receive signals to execute gc IP
func (s *SpiderGC) releaseIPPoolIPExecutor(ctx context.Context, workerIndex int) {
	loggerReleaseIP := logger.With(zap.Any("IPPoolIP_Worker", workerIndex))
	loggerReleaseIP.Info("Starting running 'releaseIPPoolIPExecutor'")

	for {
		select {
		case gcIPPoolIPDetail := <-s.gcIPPoolIPSignal:
			// when we received one gc signal which contains a pod name and its namespace,
			// we need to search the pod corresponding workloadenpoint to get the used history IPs.
			podUsedIPs, err := s.wepMgr.ListAllHistoricalIPs(ctx, gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName)
			if apierrors.IsNotFound(err) {
				loggerReleaseIP.Sugar().Warnf("wep '%s/%s' not found, maybe already cleaned by execScanAll",
					gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName)
				continue
			}

			if nil != err {
				loggerReleaseIP.Sugar().Errorf("failed to list wep '%s/%s' all historical IPs: %v",
					gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName, err)
				continue
			}

			// release pod used history IPs
			for poolName, ips := range podUsedIPs {
				loggerReleaseIP.Sugar().Debugf("pod '%s/%s used IPs '%+v' from pool '%s', begin to release",
					gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName, ips, poolName)
				err = s.ippoolMgr.ReleaseIP(ctx, poolName, ips)
				if apierrors.IsNotFound(err) {
					// If we can not find the IP object, which means cmdDel already released the IP
					continue
				}

				if nil != err {
					logger.Sugar().Errorf("failed to release pool '%s' IPs '%+v' in wep '%s/%s', error: %w",
						poolName, ips, gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName, err)
					continue
				}
				// TODO (Icarus9913): metric
			}

			loggerReleaseIP.Sugar().Infof("release IPPoolIP task '%+v' successfully", gcIPPoolIPDetail)

			err = s.wepMgr.RemoveFinalizer(ctx, gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName)
			if nil != err {
				loggerReleaseIP.Sugar().Errorf("failed to remove wep '%s/%s' finalizer, error: '%v'",
					gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName, err)
			}

			loggerReleaseIP.Sugar().Debugf("remove '%v/%v' finalizer '%s' successfully",
				gcIPPoolIPDetail.PodNamespace, gcIPPoolIPDetail.PodName, constant.SpiderWorkloadEndpointFinalizer)

		case <-ctx.Done():
			loggerReleaseIP.Info("receive ctx done, stop running releaseIPPoolIPExecutor")
			return
		}
	}
}
