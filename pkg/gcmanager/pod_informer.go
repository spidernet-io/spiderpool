// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// startPodInformer will set up k8s pod informer in circle
func (s *SpiderGC) startPodInformer(ctx context.Context) {
	logger.Sugar().Infof("try to register pod informer")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		logger.Info("create Pod informer")
		innerCtx, innerCancel := context.WithCancel(ctx)
		defer innerCancel()

		informerFactory := informers.NewSharedInformerFactory(s.k8ClientSet, 0)
		podInformer := informerFactory.Core().V1().Pods().Informer()
		_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onPodAdd,
			UpdateFunc: s.onPodUpdate,
			DeleteFunc: s.onPodDel,
		})
		if nil != err {
			logger.Error(err.Error())
			innerCancel()
			continue
		}
		s.informerFactory = informerFactory
		informerFactory.Start(innerCtx.Done())

		// Let the leader trigger IP GC scan all.
		// When the spiderpool-controller restarted, it will trigger IP GC scan all first.
		// If the pod informer not starts and the user delete some pods, this will lead to IP leakage.
		logger.Debug("try to trigger scan all with leader elected")
		cacheSync := cache.WaitForCacheSync(innerCtx.Done())
		if !cacheSync {
			innerCancel()
			continue
		}
		s.gcSignal <- struct{}{}

		<-innerCtx.Done()
		logger.Error("k8s pod informer broken")
	}
}

// onPodAdd represents Pod informer Add Event
func (s *SpiderGC) onPodAdd(obj interface{}) {
	// backup controller could be elected as master
	if !s.leader.IsElected() {
		return
	}

	pod := obj.(*corev1.Pod)
	podEntry, err := s.buildPodEntry(nil, pod, false)
	if nil != err {
		logger.Sugar().Errorf("onPodAdd: failed to build Pod Entry '%s/%s', error: %v", pod.Namespace, pod.Name, err)
		return
	}

	// flush the pod database
	if podEntry != nil {
		err = s.GetPodDatabase().ApplyPodEntry(podEntry)
		if nil != err {
			logger.Sugar().Errorf("onPodAdd: failed to apply Pod Entry '%s/%s', error: %v", pod.Namespace, pod.Name, err)
		}
	}
}

// onPodUpdate represents Pod informer Update Event
func (s *SpiderGC) onPodUpdate(oldObj interface{}, newObj interface{}) {
	// backup controller could be elected as master
	if !s.leader.IsElected() {
		return
	}

	oldPod := oldObj.(*corev1.Pod)
	pod := newObj.(*corev1.Pod)
	podEntry, err := s.buildPodEntry(oldPod, pod, false)
	if nil != err {
		logger.Sugar().Errorf("onPodUpdate: failed to build Pod Entry '%s/%s', error: %v", pod.Namespace, pod.Name, err)
		return
	}

	// flush the pod database
	if podEntry != nil {
		err = s.GetPodDatabase().ApplyPodEntry(podEntry)
		if nil != err {
			logger.Sugar().Errorf("onPodUpdate: failed to apply Pod Entry '%s/%s', error: %v", pod.Namespace, pod.Name, err)
		}
	}
}

// onPodDel represents Pod informer Delete Event
func (s *SpiderGC) onPodDel(obj interface{}) {
	// backup controller could be elected as master
	if !s.leader.IsElected() {
		return
	}

	pod := obj.(*corev1.Pod)
	logger.Sugar().Debugf("onPodDel: receive pod '%s/%s' deleted event", pod.Namespace, pod.Name)
	podEntry, err := s.buildPodEntry(nil, pod, true)
	if nil != err {
		logger.Sugar().Errorf("onPodDel: failed to build Pod Entry '%s/%s', error: %v", pod.Namespace, pod.Name, err)
		return
	}

	if podEntry != nil {
		err = s.GetPodDatabase().ApplyPodEntry(podEntry)
		if nil != err {
			logger.Sugar().Errorf("onPodDel: failed to apply Pod Entry '%s/%s', error: %v", pod.Namespace, pod.Name, err)
		}
	} else {
		logger.Sugar().Debugf("onPodDel: discard to apply status '%v' PodEntry '%s/%s'", pod.Status.Phase, pod.Namespace, pod.Name)
	}
}
