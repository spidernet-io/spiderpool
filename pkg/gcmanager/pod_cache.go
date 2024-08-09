// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodDBer interface {
	DeletePodEntry(namespace, podName string)
	ApplyPodEntry(podEntry *PodEntry) error
	ListAllPodEntries() []PodEntry
}

// PodEntry represents a pod cache
type PodEntry struct {
	PodName   string
	Namespace string
	NodeName  string
	UID       string

	EntryUpdateTime     time.Time
	TracingStartTime    time.Time
	TracingGracefulTime time.Duration
	TracingStopTime     time.Time

	PodTracingReason types.PodStatus
	NumRequeues      int
}

// PodDatabase represents controller PodEntry database
type PodDatabase struct {
	lock.RWMutex
	pods   map[ktypes.NamespacedName]PodEntry
	maxCap int
}

func NewPodDBer(maxDatabaseCap int) PodDBer {
	return &PodDatabase{
		pods:   make(map[ktypes.NamespacedName]PodEntry, maxDatabaseCap),
		maxCap: maxDatabaseCap,
	}
}

func (p *PodDatabase) DeletePodEntry(namespace, podName string) {
	p.Lock()

	_, ok := p.pods[ktypes.NamespacedName{Namespace: namespace, Name: podName}]
	if !ok {
		// already deleted
		p.Unlock()
		logger.Sugar().Debugf("PodDatabase already deleted %s/%s", namespace, podName)
		return
	}

	delete(p.pods, ktypes.NamespacedName{Namespace: namespace, Name: podName})
	p.Unlock()
	logger.Sugar().Debugf("delete %s/%s pod cache successfully", namespace, podName)
}

func (p *PodDatabase) ListAllPodEntries() []PodEntry {
	p.RLock()
	defer p.RUnlock()

	podEntryList := make([]PodEntry, 0, len(p.pods))
	for podID := range p.pods {
		podEntryList = append(podEntryList, p.pods[podID])
	}

	return podEntryList
}

func (p *PodDatabase) ApplyPodEntry(podEntry *PodEntry) error {
	if podEntry == nil {
		return fmt.Errorf("received a nil podEntry")
	}

	if podEntry.PodTracingReason == constant.PodRunning {
		return fmt.Errorf("discard '%s' status pod", constant.PodRunning)
	}

	p.Lock()
	podCache, ok := p.pods[ktypes.NamespacedName{Namespace: podEntry.Namespace, Name: podEntry.PodName}]
	if !ok {
		if len(p.pods) == p.maxCap {
			p.Unlock()
			return fmt.Errorf("podEntry database is out of capacity, discard podEntry '%+v'", *podEntry)
		}

		p.pods[ktypes.NamespacedName{Namespace: podEntry.Namespace, Name: podEntry.PodName}] = *podEntry
		p.Unlock()
		logger.Sugar().Debugf("create pod entry '%+v'", *podEntry)
		return nil
	}

	// diff and fresh the DB
	if podCache.TracingStartTime != podEntry.TracingStartTime ||
		podCache.TracingGracefulTime != podEntry.TracingGracefulTime ||
		podCache.TracingStopTime != podEntry.TracingStopTime ||
		podCache.PodTracingReason != podEntry.PodTracingReason ||
		podCache.NumRequeues != podEntry.NumRequeues {
		p.pods[ktypes.NamespacedName{Namespace: podCache.Namespace, Name: podCache.PodName}] = *podEntry
		p.Unlock()
		logger.Sugar().Debugf("podEntry '%s/%s' has changed, the old '%+v' and the new is '%+v'",
			podCache.Namespace, podCache.PodName, podCache, *podEntry)
		return nil
	}

	p.Unlock()
	return nil
}

// buildPodEntry will build PodEntry with the given args, it serves for Pod Informer event hooks
func (s *SpiderGC) buildPodEntry(oldPod, currentPod *corev1.Pod, deleted bool) (*PodEntry, error) {
	if currentPod == nil {
		return nil, fmt.Errorf("currentPod must be specified")
	}

	if currentPod.Spec.HostNetwork {
		logger.Sugar().Debugf("discard tracing HostNetwork pod %s/%s", currentPod.Namespace, currentPod.Name)
		return nil, nil
	}

	ownerRef := metav1.GetControllerOf(currentPod)
	ctx := context.TODO()

	// check StatefulSet pod, we will trace it if its controller StatefulSet object was deleted or decreased its replicas and the pod index was out of the replicas.
	if s.gcConfig.EnableStatefulSet && ownerRef != nil &&
		ownerRef.APIVersion == appsv1.SchemeGroupVersion.String() && ownerRef.Kind == constant.KindStatefulSet {
		isValidStsPod, err := s.stsMgr.IsValidStatefulSetPod(ctx, currentPod.Namespace, currentPod.Name, ownerRef.Kind)
		if nil != err {
			return nil, err
		}

		// StatefulSet pod restarted, no need to trace it.
		if isValidStsPod {
			logger.Sugar().Debugf("the StatefulSet pod '%s/%s' just restarts, keep its IPs", currentPod.Namespace, currentPod.Name)
			return nil, nil
		}
	}

	// check kubevirt vm pod, we will trace it if its controller is no longer exist
	if s.gcConfig.EnableKubevirtStaticIP && ownerRef != nil &&
		ownerRef.APIVersion == kubevirtv1.SchemeGroupVersion.String() && ownerRef.Kind == constant.KindKubevirtVMI {
		isValidVMPod, err := s.kubevirtMgr.IsValidVMPod(logutils.IntoContext(ctx, logger), currentPod.Namespace, ownerRef.Kind, ownerRef.Name)
		if nil != err {
			return nil, err
		}

		if isValidVMPod {
			logger.Sugar().Debugf("the kubevirt vm pod '%s/%s' just restarts, keep its IPs", currentPod.Namespace, currentPod.Name)
			return nil, nil
		}
	}

	// deleted pod
	if deleted {

		podEntry := &PodEntry{
			PodName:             currentPod.Name,
			Namespace:           currentPod.Namespace,
			NodeName:            currentPod.Spec.NodeName,
			UID:                 string(currentPod.UID),
			EntryUpdateTime:     metav1.Now().UTC(),
			TracingStartTime:    metav1.Now().UTC(),
			TracingGracefulTime: time.Duration(s.gcConfig.AdditionalGraceDelay) * time.Second,
			PodTracingReason:    constant.PodDeleted,
		}

		// stop time
		podEntry.TracingStopTime = podEntry.TracingStartTime.Add(podEntry.TracingGracefulTime)
		return podEntry, nil
	} else {
		var podStatus types.PodStatus
		var isBuildTerminatingPodEntry, isBuildSucceededOrFailedPodEntry bool
		switch {
		case currentPod.DeletionTimestamp != nil && oldPod == nil:
			// case: current pod is 'Terminating', no old pod
			isBuildTerminatingPodEntry = true
			podStatus = constant.PodTerminating

		case currentPod.DeletionTimestamp != nil && oldPod != nil && oldPod.DeletionTimestamp == nil:
			// case: current pod is 'Terminating', old pod wasn't 'Terminating' phase
			isBuildTerminatingPodEntry = true
			podStatus = constant.PodTerminating

		case currentPod.DeletionTimestamp != nil && oldPod.DeletionTimestamp != nil && *currentPod.Spec.TerminationGracePeriodSeconds != *oldPod.Spec.TerminationGracePeriodSeconds:
			// case: both of current pod and old pod are 'Terminating', but 'TerminationGracePeriodSeconds' changed
			isBuildTerminatingPodEntry = true
			podStatus = constant.PodTerminating

		case (currentPod.Status.Phase == corev1.PodSucceeded || currentPod.Status.Phase == corev1.PodFailed) && oldPod == nil:
			// case: current pod is 'Succeeded|Failed', no old pod
			isBuildSucceededOrFailedPodEntry = true
			podStatus = types.PodStatus(currentPod.Status.Phase)

		case (currentPod.Status.Phase == corev1.PodSucceeded || currentPod.Status.Phase == corev1.PodFailed) && (oldPod != nil && currentPod.Status.Phase != oldPod.Status.Phase):
			// case: current pod is 'Succeeded|Failed', old pod wasn't 'Succeeded|Failed' phase
			isBuildSucceededOrFailedPodEntry = true
			podStatus = types.PodStatus(currentPod.Status.Phase)

		case (currentPod.Status.Phase == corev1.PodSucceeded || currentPod.Status.Phase == corev1.PodFailed) &&
			(oldPod != nil && currentPod.Status.Phase == oldPod.Status.Phase && *currentPod.Spec.TerminationGracePeriodSeconds != *oldPod.Spec.TerminationGracePeriodSeconds):
			// case: both of current pod and old pod are 'Terminating', but 'TerminationGracePeriodSeconds' changed
			isBuildSucceededOrFailedPodEntry = true
			podStatus = types.PodStatus(currentPod.Status.Phase)

		default:
			// Running | Unknown
			return nil, nil
		}

		if isBuildTerminatingPodEntry {
			// check terminating Pod corresponding Node status
			node, err := s.nodeMgr.GetNodeByName(ctx, currentPod.Spec.NodeName, constant.UseCache)
			if nil != err {
				return nil, fmt.Errorf("failed to get terminating Pod '%s/%s' corredponing Node '%s', error: %v", currentPod.Namespace, currentPod.Name, currentPod.Spec.NodeName, err)
			}
			// disable for gc terminating pod with Node Ready
			if nodemanager.IsNodeReady(node) && !s.gcConfig.EnableGCStatelessTerminatingPodOnReadyNode {
				logger.Sugar().Debugf("IP GC already turn off 'EnableGCForTerminatingPodWithNodeReady' configuration, disacrd tracing pod '%s/%s'", currentPod.Namespace, currentPod.Name)
				return nil, nil
			}
			// disable for gc terminating pod with Node NotReady
			if !nodemanager.IsNodeReady(node) && !s.gcConfig.EnableGCStatelessTerminatingPodOnNotReadyNode {
				logger.Sugar().Debugf("IP GC already turn off 'EnableGCForTerminatingPodWithNodeNotReady' configuration, disacrd tracing pod '%s/%s'", currentPod.Namespace, currentPod.Name)
				return nil, nil
			}

			podEntry := &PodEntry{
				PodName:          currentPod.Name,
				Namespace:        currentPod.Namespace,
				NodeName:         currentPod.Spec.NodeName,
				EntryUpdateTime:  metav1.Now().UTC(),
				UID:              string(currentPod.UID),
				TracingStartTime: currentPod.DeletionTimestamp.Time,
				PodTracingReason: podStatus,
			}

			// grace period
			if currentPod.DeletionGracePeriodSeconds == nil {
				return nil, fmt.Errorf("pod '%s/%s' status is '%v' but doesn't have 'DeletionGracePeriodSeconds' property", currentPod.Namespace, currentPod.Name, podStatus)
			}
			podEntry.TracingGracefulTime = (time.Duration(*currentPod.DeletionGracePeriodSeconds) + time.Duration(s.gcConfig.AdditionalGraceDelay)) * time.Second

			// stop time
			podEntry.TracingStopTime = podEntry.TracingStartTime.Add(podEntry.TracingGracefulTime)

			return podEntry, nil
		} else if isBuildSucceededOrFailedPodEntry {
			podEntry := &PodEntry{
				PodName:          currentPod.Name,
				Namespace:        currentPod.Namespace,
				NodeName:         currentPod.Spec.NodeName,
				UID:              string(currentPod.UID),
				EntryUpdateTime:  metav1.Now().UTC(),
				PodTracingReason: podStatus,
			}

			startTime, stopTime, gracefulTime, err := s.computeSucceededOrFailedPodTerminatingTime(currentPod)
			if nil != err {
				return nil, err
			}
			// start time
			podEntry.TracingStartTime = startTime
			// grace period
			podEntry.TracingGracefulTime = gracefulTime

			// stop time
			podEntry.TracingStopTime = stopTime

			return podEntry, nil
		} else {
			return nil, nil
		}
	}
}

// computeSucceededOrFailedPodTerminatingTime will compute terminating start time, stop time and graceful period for 'Succeeded | Failed' phase pod
func (s *SpiderGC) computeSucceededOrFailedPodTerminatingTime(podYaml *corev1.Pod) (terminatingStartTime, terminatingStopTime time.Time, gracefulTime time.Duration, err error) {
	// check container numbers
	containerNum := len(podYaml.Status.ContainerStatuses)
	if containerNum == 0 {
		err = fmt.Errorf("pod '%s/%s' doesn't have any containers", podYaml.Namespace, podYaml.Name)
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
