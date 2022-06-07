// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/lock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PodDBer是数据库接口，支持CURD操作
type PodDBer interface {
	Get(podName, namespace string) (*PodEntry, error)
	Create(pod *corev1.Pod) (*PodEntry, error)
	Update(pod *corev1.Pod) (*PodEntry, error)
	Delete(podName, namespace string)
}

// PodEntry代表一个pod的具体缓存数据
type PodEntry struct {
	PodName   string
	Namespace string
	NodeName  string

	// 保留字段
	EntryCreateTime time.Time

	TerminatingStartTime *time.Time
	PodGracefulTime      *time.Duration
	TerminatingStopTime  *time.Time

	PodPhase corev1.PodPhase
}

// PodDatabase指的是本地缓存数据库
type PodDatabase struct {
	lock.RWMutex
	pods map[types.NamespacedName]*PodEntry
}

func NewPodDBer() PodDBer {
	return &PodDatabase{
		pods: make(map[types.NamespacedName]*PodEntry),
	}
}

func (p *PodDatabase) Get(podName, namespace string) (*PodEntry, error) {
	p.RLock()
	defer p.RUnlock()

	podEntry, ok := p.pods[types.NamespacedName{Namespace: namespace, Name: podName}]
	if !ok {
		return nil, fmt.Errorf("error: no pod '%s' namespace '%s' cache in PodDatabse", podName, namespace)
	}
	return podEntry, nil
}

// 新建pod cache到数据库中
func (p *PodDatabase) Create(pod *corev1.Pod) (*PodEntry, error) {
	p.Lock()
	defer p.Unlock()

	_, ok := p.pods[types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}]
	if ok {
		return nil, fmt.Errorf("error:  pod '%s' namespace '%s' cache already exists in PodDatabase", pod.Name, pod.Namespace)
	}

	podCache := &PodEntry{
		PodName:         pod.Name,
		Namespace:       pod.Namespace,
		NodeName:        pod.Spec.NodeName,
		EntryCreateTime: time.Now(),
		PodPhase:        pod.Status.Phase,
	}

	p.pods[types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}] = podCache

	return podCache, nil
}

// 如果是标志terminate则更新那几个time，并更新phase属性， 否则只更新phase
func (p *PodDatabase) Update(pod *corev1.Pod) (*PodEntry, error) {
	p.Lock()
	defer p.Unlock()

	podEntry, ok := p.pods[types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}]
	if !ok {
		return nil, fmt.Errorf("error: no pod '%s' namespace '%s' cache in PodDatabse", pod.Name, pod.Namespace)
	}

	// RFC3339 && UTC. 这里不能用time.Now()，因为podDB是内存数据。
	if podEntry.TerminatingStartTime == nil && pod.DeletionTimestamp != nil {
		podEntry.TerminatingStartTime = &pod.DeletionTimestamp.Time

		if podEntry.PodGracefulTime == nil && nil != pod.DeletionGracePeriodSeconds {
			gracePeriod := *pod.DeletionGracePeriodSeconds
			podEntry.PodGracefulTime = (*time.Duration)(&gracePeriod)
		}

		// 计算terminating结束时间，即可以执行GC的时间
		if nil != podEntry.PodGracefulTime && nil != podEntry.TerminatingStartTime {
			stopTime := podEntry.TerminatingStartTime.Add(*podEntry.PodGracefulTime)
			podEntry.TerminatingStopTime = &stopTime
		}
	}

	// 更新pod phase
	if podEntry.PodPhase != pod.Status.Phase {
		podEntry.PodPhase = pod.Status.Phase
	}

	return podEntry, nil
}

func (p *PodDatabase) Delete(podName, namespace string) {
	p.Lock()
	defer p.Unlock()

	_, ok := p.pods[types.NamespacedName{Namespace: namespace, Name: podName}]
	if !ok {
		// already deleted
		logger.Sugar().Debugf("PodDatabase already deleted %s", podName)
		return
	}

	delete(p.pods, types.NamespacedName{Namespace: namespace, Name: podName})
	logger.Sugar().Debugf("delete %s pod cache successfully", podName)
}
