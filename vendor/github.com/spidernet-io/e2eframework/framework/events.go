// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) WaitExceptEventOccurred(ctx context.Context, eventKind, objName, objNamespace, message string) error {

	if eventKind == "" || objName == "" || objNamespace == "" || message == "" {
		return ErrWrongInput
	}
	l := &client.ListOptions{
		Raw: &metav1.ListOptions{
			TypeMeta:      metav1.TypeMeta{Kind: eventKind},
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", objName, objNamespace),
		},
	}
	watchInterface, err := f.KClient.Watch(ctx, &corev1.EventList{}, l)
	if err != nil {
		return ErrWatch
	}
	defer watchInterface.Stop()
	for {
		select {
		case <-ctx.Done():
			return ErrTimeOut
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return ErrChanelClosed
			}
			f.Log("watch event object %v", event.Object)
			switch event.Type {
			case watch.Deleted:
				return ErrResDel
			default:
				event, ok := event.Object.(*corev1.Event)
				if !ok {
					return ErrGetObj
				}
				f.Log("Event occurred message is %s/%v \n", event.Type, event.Message)
				if strings.Contains(event.Message, message) {
					return nil
				}
			}
		}
	}
}

func (f *Framework) GetEvents(ctx context.Context, eventKind, objName, objNamespace string) (*corev1.EventList, error) {
	l := &client.ListOptions{
		Raw: &metav1.ListOptions{
			TypeMeta:      metav1.TypeMeta{Kind: eventKind},
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", objName, objNamespace),
		},
	}

	events := &corev1.EventList{}
	if err := f.KClient.List(ctx, events, l); err != nil {
		return nil, err
	}

	return events, nil
}
