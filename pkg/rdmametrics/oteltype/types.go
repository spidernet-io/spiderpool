// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package oteltype

import "go.opentelemetry.io/otel/attribute"

type Metrics struct {
	Name   string
	Value  int64
	Labels []attribute.KeyValue
}
