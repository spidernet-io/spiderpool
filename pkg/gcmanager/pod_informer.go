// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// startPodInformer will set up k8s pod informer in circle
func (s *SpiderGC) startPodInformer() {
	logger.Sugar().Infof("register pod informer")

	for {
		logger.Info("create Pod informer")
		informerFactory := informers.NewSharedInformerFactory(s.k8ClientSet, 0)
		stopper := make(chan struct{})

		podInformer := informerFactory.Core().V1().Pods().Informer()
		podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onPodAdd,
			UpdateFunc: s.onPodUpdate,
			DeleteFunc: s.onPodDel,
		})
		go podInformer.Run(stopper)

		<-stopper
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
	logger.Sugar().Infof("onPodDel: receive pod '%s/%s' deleted event", pod.Namespace, pod.Name)
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
