// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// registerPodReconcile registers watch pod
func (s *SpiderGC) registerPodReconcile() error {
	c, err := controller.New("watch-pod-controller", s.controllerMgr, controller.Options{
		Reconciler: s,
	})
	if nil != err {
		return err
	}

	// TODO (Icarus9913): Do we still need predicate to filter the Generic events?
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{})
	if nil != err {
		return err
	}

	logger.Info("register pod reconcile successfully!")

	return nil
}

// Reconcile notice: if reconcile received an error, then the correspond request will requeue.
func (s *SpiderGC) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// backup controller could be elected as master
	if !s.isGCMasterElected() {
		return reconcile.Result{}, nil
	}

	// Fetch the Pod form the cache
	var tmpPodEntry *PodEntry
	pod, err := s.podMgr.GetPodByName(ctx, request.Namespace, request.Name)
	if err != nil || pod == nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Debugf("reconcile found deleted Pod '%+v'", request.NamespacedName)

			tmpPodEntry = s.buildDeletedPodEntry(request.Namespace, request.Name)
		} else {
			logger.Sugar().Errorf("could not fetch Pod: '%+v' with error: %v", request.NamespacedName, err)
			return reconcile.Result{}, nil
		}
	} else if pod.DeletionTimestamp != nil {
		if !s.gcConfig.EnableGCForTerminatingPod {
			logger.Sugar().Debugf("IP gc already turn off 'EnableGCForTerminatingPod' configuration, disacrd pod '%+v'", request.NamespacedName)
			return reconcile.Result{}, nil
		}

		logger.Sugar().Debugf("reconcile found terminating Pod '%+v'", request.NamespacedName)

		tmpPodEntry, err = s.buildPodEntryWithPodYaml(ctx, pod)
		if nil != err {
			logger.Sugar().Errorf("failed to build '%+v' Pod Entry, error: %v", request.NamespacedName, err)
			return reconcile.Result{}, nil
		}
	} else if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		logger.Sugar().Debugf("reconcile found '%v' status Pod '%+v'", pod.Status.Phase, request.NamespacedName)

		tmpPodEntry, err = s.buildPodEntryWithPodYaml(ctx, pod)
		if nil != err {
			logger.Sugar().Errorf("failed to build '%+v' Pod Entry, error: %v", request.NamespacedName, err)
			return reconcile.Result{}, nil
		}
	} else if pod.Status.Phase == corev1.PodRunning {
		return reconcile.Result{}, nil
	} else {
		return reconcile.Result{}, nil
	}

	// flush the pod database
	if tmpPodEntry != nil {
		err = s.GetPodDatabase().ApplyPodEntry(tmpPodEntry)
		if nil != err {
			logger.Sugar().Errorf("failed to apply Pod Entry '%+v', error: %v", request.NamespacedName, err)
		}
	} else {
		logger.Sugar().Debugf("discard to build status '%v' PodEntry '%+v'", pod.Status.Phase, request.NamespacedName)
	}

	return reconcile.Result{}, nil
}

// buildDeletedPodEntry only serves for 'Deleted' phase pod
func (s *SpiderGC) buildDeletedPodEntry(podNS, podName string) *PodEntry {
	podEntry := &PodEntry{
		PodName:          podName,
		Namespace:        podNS,
		EntryUpdateTime:  metav1.Now().UTC(),
		TracingStartTime: metav1.Now().UTC(),
		PodTracingReason: constant.PodDeleted,
	}

	// graceful time
	podEntry.TracingGracefulTime = time.Duration(s.gcConfig.AdditionalGraceDelay) * time.Second

	// stop time
	podEntry.TracingStopTime = podEntry.TracingStartTime.Add(podEntry.TracingGracefulTime)

	return podEntry
}

// buildPodEntryWithPodYaml only serves for 'Terminating | Succeeded | Failed' phase pod
func (s *SpiderGC) buildPodEntryWithPodYaml(ctx context.Context, podYaml *corev1.Pod) (*PodEntry, error) {
	if podYaml == nil {
		return nil, fmt.Errorf("received a nil podYaml")
	}

	podStatus, _ := s.podMgr.CheckPodStatus(ctx, podYaml)

	podEntry := &PodEntry{
		PodName:          podYaml.Name,
		Namespace:        podYaml.Namespace,
		NodeName:         podYaml.Spec.NodeName,
		EntryUpdateTime:  metav1.Now().UTC(),
		PodTracingReason: podStatus,
	}

	if podStatus == constant.PodTerminating || podStatus == constant.PodGraceTimeOut {
		// start time
		podEntry.TracingStartTime = podYaml.DeletionTimestamp.Time

		// grace period
		if podYaml.DeletionGracePeriodSeconds == nil {
			return nil, fmt.Errorf("pod '%s/%s' status is '%v' but doesn't have 'DeletionGracePeriodSeconds' property", podYaml.Namespace, podYaml.Name, podStatus)
		}
		podEntry.TracingGracefulTime = (time.Duration(*podYaml.DeletionGracePeriodSeconds) + time.Duration(s.gcConfig.AdditionalGraceDelay)) * time.Second
	} else if podStatus == constant.PodSucceeded || podStatus == constant.PodFailed {
		startTime, _, gracefulTime, err := s.computeSucceededOrFailedPodTerminatingTime(podYaml)
		if nil != err {
			return nil, err
		}

		podEntry.TracingStartTime = startTime
		podEntry.TracingGracefulTime = gracefulTime
	} else {
		logger.Sugar().Debugf("discard building '%s' status PodEntry '%s/%s'", podStatus, podYaml.Namespace, podYaml.Name)
		return nil, nil
	}

	// stop time
	podEntry.TracingStopTime = podEntry.TracingStartTime.Add(podEntry.TracingGracefulTime)

	return podEntry, nil
}

// computeSucceededOrFailedPodTerminatingTime will compute terminating start time, stop time and graceful period for 'Succeeded | Failed' phase pod
func (s *SpiderGC) computeSucceededOrFailedPodTerminatingTime(podYaml *corev1.Pod) (terminatingStartTime, terminatingStopTime time.Time, gracefulTime time.Duration, err error) {
	if podYaml.Status.Phase == corev1.PodSucceeded || podYaml.Status.Phase == corev1.PodFailed {
		// check container numbers
		containerNum := len(podYaml.Status.ContainerStatuses)
		if containerNum == 0 {
			err = fmt.Errorf("pod '%s/%s' doesn't have any containers", podYaml.Namespace, podYaml.Name)
			return
		}

		// check container status
		if len(podYaml.Status.ContainerStatuses) == 0 {
			err = fmt.Errorf("pod '%s/%s' status is '%v' but doesn't have 'ContainerStateTerminated' property", podYaml.Namespace, podYaml.Name, podYaml.Status.Phase)
			return
		}

		// compute Succeeded | Failed pod start time
		var tmpStartTime time.Time
		for _, containerStatus := range podYaml.Status.ContainerStatuses {
			if containerStatus.State.Terminated == nil {
				continue
			}

			if tmpStartTime.Before(containerStatus.State.Terminated.FinishedAt.UTC()) {
				tmpStartTime = containerStatus.State.Terminated.FinishedAt.UTC()
			}
		}

		if tmpStartTime.IsZero() {
			err = fmt.Errorf("pod '%s/%s' status is '%v' but doesn't have terminated finishedTime",
				podYaml.Namespace, podYaml.Name, podYaml.Status.Phase)
			return
		}

		terminatingStartTime = tmpStartTime

		// graceful period
		if podYaml.Spec.TerminationGracePeriodSeconds == nil {
			err = fmt.Errorf("pod '%s/%s' doesn't have 'TerminationGracePeriodSeconds' property", podYaml.Namespace, podYaml.Name)
			return
		}
		gracefulTime = (time.Duration(*podYaml.Spec.TerminationGracePeriodSeconds) + time.Duration(s.gcConfig.AdditionalGraceDelay)) * time.Second

		// stop time
		terminatingStopTime = terminatingStartTime.Add(gracefulTime)
		return
	}

	err = fmt.Errorf("not match pod status! Pod '%s/%s' status is '%v'", podYaml.Namespace, podYaml.Name, podYaml.Status.Phase)
	return
}
