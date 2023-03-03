// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type EndpointManagerConfig struct {
	MaxConflictRetries    int
	ConflictRetryUnitTime time.Duration
	scheme                *runtime.Scheme
}

func setDefaultsForEndpointManagerConfig(config EndpointManagerConfig) EndpointManagerConfig {
	if config.scheme == nil {
		config.scheme = runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(config.scheme))
	}

	return config
}
