// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/types"

	ktypes "k8s.io/apimachinery/pkg/types"
)

type PodDBer interface {
	GetPodEntry(podName, namespace string) PodEntry
	DeletePodEntry(podName, namespace string)
	ApplyPodEntry(podEntry *PodEntry) error
	ListAllPodEntries() []PodEntry
}

// PodEntry represents a pod cache
type PodEntry struct {
	PodName   string
	Namespace string
	NodeName  string

	EntryUpdateTime time.Time

	TracingStartTime    time.Time
	TracingGracefulTime time.Duration
	TracingStopTime     time.Time

	PodTracingReason types.PodStatus
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

func (p *PodDatabase) GetPodEntry(podName, namespace string) PodEntry {
	p.RLock()
	defer p.RUnlock()

	podEntry, ok := p.pods[ktypes.NamespacedName{Namespace: namespace, Name: podName}]
	if !ok {
		return PodEntry{}
	}
	return podEntry
}

func (p *PodDatabase) DeletePodEntry(podName, namespace string) {
	p.Lock()
	defer p.Unlock()

	_, ok := p.pods[ktypes.NamespacedName{Namespace: namespace, Name: podName}]
	if !ok {
		// already deleted
		logger.Sugar().Debugf("PodDatabase already deleted %s", podName)
		return
	}

	delete(p.pods, ktypes.NamespacedName{Namespace: namespace, Name: podName})
	logger.Sugar().Debugf("delete %s pod cache successfully", podName)
}

func (p *PodDatabase) ListAllPodEntries() []PodEntry {
	p.RLock()
	defer p.RUnlock()

	var podEntryList []PodEntry
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
	defer p.Unlock()

	podCache, ok := p.pods[ktypes.NamespacedName{Namespace: podEntry.Namespace, Name: podEntry.PodName}]

	if !ok {
		if len(p.pods) == p.maxCap {
			// TODO (Icarus9913): add otel metric
			logger.Sugar().Warnf("podEntry database is out of capacity, discard podEntry '%+v'", *podEntry)
			return fmt.Errorf("podEntry database is out of capacity")
		}

		p.pods[ktypes.NamespacedName{Namespace: podEntry.Namespace, Name: podEntry.PodName}] = *podEntry
		logger.Sugar().Debugf("create pod entry '%+v'", podEntry)
		return nil
	}

	if diffPodEntries(&podCache, podEntry) {
		p.pods[ktypes.NamespacedName{Namespace: podCache.Namespace, Name: podCache.PodName}] = *podEntry
	}

	return nil
}

func diffPodEntries(oldOne, newOne *PodEntry) bool {
	var isDifferent bool

	if oldOne.TracingStartTime != newOne.TracingStartTime ||
		oldOne.TracingGracefulTime != newOne.TracingGracefulTime ||
		oldOne.TracingStopTime != newOne.TracingStopTime ||
		oldOne.PodTracingReason != newOne.PodTracingReason {
		isDifferent = true

		logger.Sugar().Debugf("podEntry '%s/%s' has changed, the old '%+v' and the new is '%+v'",
			oldOne.Namespace, oldOne.PodName, *oldOne, *newOne)
	}

	return isDifferent
}
