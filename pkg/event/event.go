// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func NewEventRecorder(sourceComponent string, clientConfig *rest.Config, scheme *runtime.Scheme) (record.EventRecorder, error) {
	eventClient, err := v1core.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Spiderpool event client: %v", err)
	}

	eventBroadcaster := record.NewBroadcaster()
	logger := logutils.Logger.Named(sourceComponent)
	eventBroadcaster.StartLogging(logger.Sugar().Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: eventClient.Events(""),
	})

	return eventBroadcaster.NewRecorder(scheme, corev1.EventSource{
		Component: sourceComponent,
	}), nil
}
