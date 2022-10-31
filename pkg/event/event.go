// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package event

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// EventRecorder is Singleton
var EventRecorder record.EventRecorder

const FakeRecorderBufferSize = 1024

// init will give the EventRecorder with default fake Recorder to avoid panic if someone forget to initialize it
func init() {
	EventRecorder = record.NewFakeRecorder(FakeRecorderBufferSize)
}

// InitEventRecorder will initialize the Singleton EventRecorder
func InitEventRecorder(client *kubernetes.Clientset, scheme *runtime.Scheme, sourceComponent string) {
	eventBroadcaster := record.NewBroadcaster()

	eventBroadcaster.StartLogging(logutils.Logger.Named(sourceComponent).Sugar().Infof)
	eventBroadcaster.StartRecordingToSink(&typedv1.EventSinkImpl{
		Interface: typedv1.New(client.CoreV1().RESTClient()).Events(""),
	})

	EventRecorder = eventBroadcaster.NewRecorder(scheme, corev1.EventSource{
		Component: sourceComponent,
	})
}
