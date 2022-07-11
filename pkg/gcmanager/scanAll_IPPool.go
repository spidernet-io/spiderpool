// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// monitorGCSignal will monitor signal from CLI, DefaultGCInterval
func (s *SpiderGC) monitorGCSignal(ctx context.Context) {
	logger.Debug("start to monitor gc signal for CLI or default GC interval")

	timer := time.NewTimer(time.Duration(s.gcConfig.DefaultGCIntervalDuration) * time.Second)
	defer timer.Stop()

	logger.Debug("initial scan all for cluster firstly")
	s.gcSignal <- struct{}{}

	for {
		select {
		case <-timer.C:
			select {
			// In concurrency situation, the backup controller must execute scanAll
			case <-s.gcSignal:
				logger.Info("receive CLI GC request, execute scan all right now!")
				s.executeScanAll(ctx)
			default:
				// The Elected controller will scan All with default GC interval
				// TODO (Icarus9913): metric
				if s.isGCMasterElected() {
					logger.Info("trigger default GC interval, execute scan all right now!")
					s.executeScanAll(ctx)
				}
			}

			// CLI request
		case <-s.gcSignal:
			logger.Info("receive CLI GC request, execute scan all right now!")
			s.executeScanAll(ctx)
			time.Sleep(time.Duration(s.gcConfig.GCSignalGapDuration) * time.Second)

			// discard the concurrent signal
			select {
			case <-timer.C:
			default:
			}

		case <-ctx.Done():
			logger.Info("receive ctx done, stop monitoring gc signal.")
			return
		}

		timer.Reset(time.Duration(s.gcConfig.DefaultGCIntervalDuration) * time.Second)
	}
}

// executeScanAll scans the whole pod and whole IPPoolList
func (s *SpiderGC) executeScanAll(ctx context.Context) {
	poolList, err := s.ippoolMgr.ListAllIPPool(ctx)
	if apierrors.IsNotFound(err) {
		logger.Sugar().Warnf("scan all failed, ippoolList not found!")
		return
	}

	if nil != err {
		logger.Sugar().Errorf("scan all failed: '%v'", err)
		return
	}

	for _, pool := range poolList.Items {
		logger.Sugar().Debugf("checking IPPool '%s/%s'", pool.Namespace, pool.Name)

		for poolIP, poolIPAllocation := range pool.Status.AllocatedIPs {
			scanAllLogger := logger.With(zap.String("podNS", poolIPAllocation.Namespace), zap.String("podName", poolIPAllocation.Pod),
				zap.String("containerID", poolIPAllocation.ContainerID), zap.String("NIC", poolIPAllocation.NIC))

			podYaml, err := s.podMgr.GetPodByName(ctx, poolIPAllocation.Namespace, poolIPAllocation.Pod)
			if nil != err {
				// case: The pod in IPPool's ip-allocationDetail is not exist in k8s
				if apierrors.IsNotFound(err) {
					err := s.releaseSingleIPAndRemoveWEPFinalizer(ctx, pool.Name, poolIP, poolIPAllocation)
					if nil != err {
						scanAllLogger.With(zap.String("gc-reason", "pod not found in k8s but still exists in IPPool allocation")).Error(err.Error())
						continue
					}

					scanAllLogger.With(zap.String("gc-reason", "pod not found in k8s but still exists in IPPool allocation")).
						Sugar().Infof("release ip '%s' and remove wep '%s/%s' finalizer successfully!", poolIP, poolIPAllocation.Namespace, poolIPAllocation.Pod)

					continue
				}

				scanAllLogger.With(zap.String("gc-reason", "pod not found in k8s but still exists in ippool allocation")).
					Sugar().Errorf("check pod from kubernetes failed with error '%v'", err)
			} else if podYaml.DeletionTimestamp != nil {
				// case: The pod in IPPool's ip-allocationDetail is also exist in k8s but with phase terminating
				if podYaml.DeletionGracePeriodSeconds == nil {
					scanAllLogger.With(zap.String("gc-reason", "pod is terminating")).
						Sugar().Errorf("get pod data unexpectedly, pod 'DeletionGracePeriodSeconds' is nil!")
					continue
				}

				scanAllCtx := logutils.IntoContext(ctx, scanAllLogger.With(zap.String("gc-reason", "pod is terminating")))
				stopTime := (*podYaml.DeletionTimestamp).Time.Add((time.Duration(*podYaml.DeletionGracePeriodSeconds) + time.Duration(s.gcConfig.AdditionalGraceDelay)) * time.Second)

				if err := s.handleTerminatingPod(scanAllCtx, podYaml, stopTime, pool.Name, poolIP, poolIPAllocation); nil != err {
					scanAllLogger.With(zap.String("gc-reason", "pod is terminating")).Error(err.Error())
				}

			} else if podYaml.Status.Phase == corev1.PodSucceeded || podYaml.Status.Phase == corev1.PodFailed {
				// case: The pod in IPPool's ip-allocationDetail is also exist in k8s but with phase 'Succeeded' or 'Failed'
				_, stopTime, _, err := s.computeSucceededOrFailedPodTerminatingTime(podYaml)
				if nil != err {
					scanAllLogger.With(zap.Any("gc-reason", podYaml.Status.Phase)).Error(err.Error())
					continue
				}

				scanAllCtx := logutils.IntoContext(ctx, scanAllLogger.With(zap.Any("gc-reason", podYaml.Status.Phase)))

				if err = s.handleTerminatingPod(scanAllCtx, podYaml, stopTime, pool.Name, poolIP, poolIPAllocation); nil != err {
					scanAllLogger.With(zap.String("gc-reason", "pod is terminating")).Error(err.Error())
				}

			} else {
				// case: The pod in IPPool's ip-allocationDetail is also exist in k8s, but the poolIP is not belong to WEP current IPs
				isIPBelongWEPCurrent, err := s.wepMgr.IsIPBelongWEPCurrent(ctx, podYaml.Namespace, podYaml.Name, poolIP)
				if nil != err {
					scanAllLogger.Sugar().Errorf("failed to check IP '%s' belong to wep '%s/%s' current data, error: %v", poolIP, podYaml.Namespace, podYaml.Name, err)
					continue
				}

				if !isIPBelongWEPCurrent {
					// release Ip but no need to remove wep finalizer
					if err = s.ippoolMgr.ReleaseIP(ctx, pool.Name, []ippoolmanager.IPAndCID{{
						IP:          poolIP,
						ContainerID: poolIPAllocation.ContainerID},
					}); nil != err {
						scanAllLogger.With(zap.String("gc-reason", "IPPoolAllocation ip is different with wep current ip")).
							Sugar().Errorf("failed to release ip '%s', error: '%v'", poolIP, err)

						continue
					}

					scanAllLogger.With(zap.String("gc-reason", "IPPoolAllocation ip is different with wep current ip")).
						Sugar().Infof("release ip '%s' successfully!", poolIP)
				}
			}
		}
		logger.Sugar().Debugf("task checking IPPool '%s/%s' is completed", pool.Namespace, pool.Name)
	}
}

// releaseSingleIPAndRemoveWEPFinalizer serves for handleTerminatingPod to gc singleIP and remove wep finalizer
func (s *SpiderGC) releaseSingleIPAndRemoveWEPFinalizer(ctx context.Context, poolName, poolIP string, poolIPAllocation v1.PoolIPAllocation) error {
	singleIP := []ippoolmanager.IPAndCID{{IP: poolIP, ContainerID: poolIPAllocation.ContainerID}}
	err := s.ippoolMgr.ReleaseIP(ctx, poolName, singleIP)
	if nil != err {
		return fmt.Errorf("failed to release IP '%s', error: '%v'", poolIP, err)
	}

	err = s.wepMgr.RemoveFinalizer(ctx, poolIPAllocation.Namespace, poolIPAllocation.Pod)
	if nil != err {
		return fmt.Errorf("remove wep '%s/%s' finalizer failed with err '%v'", poolIPAllocation.Namespace, poolIPAllocation.Pod, err)
	}

	return nil
}

// handleTerminatingPod serves for executeScanAll to gc single IP once the given pod is out of time
func (s *SpiderGC) handleTerminatingPod(ctx context.Context, podYaml *corev1.Pod, stopTime time.Time, poolName, ippoolIP string, poolIPAllocation v1.PoolIPAllocation) error {
	log := logutils.FromContext(ctx)

	// once it's out of time, just go to gc the IP and remove wep finalizer
	if time.Now().UTC().After(stopTime) {
		log.Sugar().Infof("begin to release ip '%s' and wep '%s/%s'.", ippoolIP, poolIPAllocation.Namespace, poolIPAllocation.Pod)

		err := s.releaseSingleIPAndRemoveWEPFinalizer(ctx, poolName, ippoolIP, poolIPAllocation)
		if nil != err {
			return err
		}

		log.Sugar().Infof("release ip '%s' and wep '%s/%s' successfully!", ippoolIP, poolIPAllocation.Namespace, poolIPAllocation.Pod)
	} else {
		// otherwise, flush the pod yaml to PodEntry database and let tracePodWorker to solve it if the current controller is elected master.
		if s.isGCMasterElected() {
			newPodEntry, err := s.buildPodEntryWithPodYaml(ctx, podYaml)
			if nil != err {
				return err
			}

			err = s.PodDB.ApplyPodEntry(newPodEntry)
			if nil != err {
				return err
			}
		}
	}

	return nil
}
