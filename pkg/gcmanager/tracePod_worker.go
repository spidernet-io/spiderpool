// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

var errRequeue = fmt.Errorf("requeue")

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
			// If the statefulset application quickly experiences scaling down and up,
			// check whether `Status.PodIPs` is empty to determine whether the Pod in the current K8S has completed the normal IP release to avoid releasing the wrong IP.
			ctx := context.TODO()
			currentPodYaml, err := s.podMgr.GetPodByName(ctx, podEntry.Namespace, podEntry.PodName, constant.UseCache)
			if err != nil {
				tracingReason := fmt.Sprintf("the graceful deletion period of pod '%s/%s' is over, get the current pod status in Kubernetes", podEntry.Namespace, podEntry.PodName)
				if apierrors.IsNotFound(err) {
					logger.With(zap.Any("podEntry tracing-reason", tracingReason)).
						Sugar().Debugf("pod '%s/%s' not found", podEntry.Namespace, podEntry.PodName)
				} else {
					logger.With(zap.Any("podEntry tracing-reason", tracingReason)).
						Sugar().Errorf("failed to get pod '%s/%s', error: %v", podEntry.Namespace, podEntry.PodName, err)
					// the pod will be handled next time.
					return
				}
			} else {
				if len(currentPodYaml.Status.PodIPs) == 0 {
					logger.Sugar().Infof("The IP address of the Pod %v that has exceeded the grace period has been released through cmdDel, ignore it.", podEntry.PodName)
					s.PodDB.DeletePodEntry(podEntry.Namespace, podEntry.PodName)
					return
				}
			}

			logger.With(zap.Any("podEntry tracing-reason", podEntry.PodTracingReason)).
				Sugar().Infof("the graceful deletion period of pod '%s/%s' is over, try to release the IP address.", podEntry.Namespace, podEntry.PodName)
		} else {
			// not time out
			return
		}
	}

	select {
	case s.gcIPPoolIPSignal <- podEntry:
		logger.Sugar().Debugf("sending signal to gc pod '%s/%s' IP", podEntry.Namespace, podEntry.PodName)
	case <-time.After(time.Duration(s.gcConfig.GCSignalTimeoutDuration) * time.Second):
		logger.Sugar().Errorf("failed to gc IP, gcSignal:len=%d, event:'%s/%s' will be dropped", len(s.gcSignal), podEntry.Namespace, podEntry.PodName)
	}
}

// releaseIPPoolIPExecutor receive signals to execute gc IP
func (s *SpiderGC) releaseIPPoolIPExecutor(ctx context.Context, workerIndex int) {
	log := logger.With(zap.Any("garbage collected trace", workerIndex))
	log.Info("Start checking if IPPool.Status.AllocatedIPs and the endpoint need to be garbage collected ")

	for {
		select {
		case podCache := <-s.gcIPPoolIPSignal:
			err := func() error {
				endpoint, err := s.wepMgr.GetEndpointByName(ctx, podCache.Namespace, podCache.PodName, constant.UseCache)
				if nil != err {
					if apierrors.IsNotFound(err) {
						log.Sugar().Infof("SpiderEndpoint '%s/%s' not found, maybe already cleaned by cmdDel or ScanAll",
							podCache.Namespace, podCache.PodName)
						return nil
					}

					log.Sugar().Errorf("failed to get SpiderEndpoint '%s/%s', error: %v", podCache.Namespace, podCache.PodName, err)
					return err
				}

				// Pod has the same name as SpiderEndpoint, but the UID does not match.
				// Such SpiderEndpoint should be reclaim, but because the IPPool name used by SpiderEndpoint cannot be tracked,
				// it will be reclaim later via GC All
				if endpoint.Status.Current.UID != podCache.UID {
					log.Sugar().Infof("Pod name=%s/%s,UID=%s and SpiderEndpoint name=%s/%s,UID=%s have the same name but different UIDs, trace gc cannot be traced, handle it through scan All",
						podCache.Namespace, podCache.PodName, podCache.UID, endpoint.Namespace, endpoint.Name, endpoint.Status.Current.UID)
					log.Sugar().Warnf("Since the IPPool name used by SpiderEndpoint cannot be tracked, it is waiting for GC all to process",
						podCache.PodName, podCache.UID, endpoint.Name, endpoint.Status.Current.UID)

					s.PodDB.DeletePodEntry(podCache.Namespace, podCache.PodName)
					return nil
				}

				// we need to gather the pod corresponding SpiderEndpoint allocation data to get the used history IPs.
				podUsedIPs := convert.GroupIPAllocationDetails(endpoint.Status.Current.UID, endpoint.Status.Current.IPs)
				tickets := podUsedIPs.Pools()
				err = s.gcLimiter.AcquireTicket(ctx, tickets...)
				if nil != err {
					log.Sugar().Errorf("failed to get IP GC limiter tickets, error: %v", err)
				}
				defer s.gcLimiter.ReleaseTicket(ctx, tickets...)

				var isReleaseFailed atomic.Bool
				wg := sync.WaitGroup{}
				wg.Add(len(podUsedIPs))
				// release pod used history IPs
				for tmpPoolName, tmpIPs := range podUsedIPs {
					go func(poolName string, ips []types.IPAndUID) {
						defer wg.Done()

						log.Sugar().Infof("pod '%s/%s used IPs '%+v' from pool '%s', begin to release",
							podCache.Namespace, podCache.PodName, ips, poolName)

						err := s.ippoolMgr.ReleaseIP(ctx, poolName, ips)
						if client.IgnoreNotFound(err) != nil {
							isReleaseFailed.Store(true)
							metric.IPGCFailureCounts.Add(ctx, 1)
							log.Sugar().Errorf("failed to release pool '%s' IPs '%+v' in SpiderEndpoint '%s/%s', error: %v",
								poolName, ips, podCache.Namespace, podCache.PodName, err)
						}
						metric.IPGCTotalCounts.Add(ctx, 1)
					}(tmpPoolName, tmpIPs)
				}
				wg.Wait()

				if isReleaseFailed.Load() {
					log.Debug("there are releasing failure in this round, we want to get a try next time")
					return errRequeue
				}

				// delete StatefulSet/kubevirtVMI wep (other controller wep has OwnerReference, its lifecycle is same with pod)
				if (endpoint.Status.OwnerControllerType == constant.KindStatefulSet || endpoint.Status.OwnerControllerType == constant.KindKubevirtVMI) &&
					endpoint.DeletionTimestamp == nil {
					err = s.wepMgr.DeleteEndpoint(ctx, endpoint)
					if nil != err {
						log.Sugar().Errorf("failed to delete '%s' wep '%s/%s', error: '%v'",
							endpoint.Status.OwnerControllerType, podCache.Namespace, podCache.PodName, err)
						return err
					}
				}

				err = s.wepMgr.RemoveFinalizer(ctx, endpoint)
				if nil != err {
					log.Sugar().Errorf("failed to remove wep '%s/%s' finalizer, error: '%v'",
						podCache.Namespace, podCache.PodName, err)
					return err
				}
				log.Sugar().Infof("remove wep '%s/%s' finalizer '%s' successfully",
					podCache.Namespace, podCache.PodName, constant.SpiderFinalizer)

				return nil
			}()

			if nil != err && podCache.NumRequeues < s.gcConfig.WorkQueueMaxRetries {
				log.Sugar().Debugf("requeue PodEntry '%s/%s' and get a retry next time", podCache.Namespace, podCache.PodName)

				podCache.EntryUpdateTime = metav1.Now().UTC()
				podCache.NumRequeues++
				err := s.PodDB.ApplyPodEntry(podCache)
				if nil != err {
					log.Error(err.Error())
					s.PodDB.DeletePodEntry(podCache.Namespace, podCache.PodName)
				}
			} else {
				s.PodDB.DeletePodEntry(podCache.Namespace, podCache.PodName)
			}
		case <-ctx.Done():
			log.Info("receive ctx done, stop running releaseIPPoolIPExecutor")
			return
		}
	}
}
