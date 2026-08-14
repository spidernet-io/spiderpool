// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import "time"

const (
	// DefaultCNIClientTimeout is the timeout the CNI plugin uses when calling the
	// Spiderpool agent. Shared by CNI ADD and CNI DEL.
	DefaultCNIClientTimeout = 100 * time.Second

	// IaaSTimeoutStaticLimit is the absolute upper bound for an IaaS provider
	// HTTP request timeout. It exists as a safety guard independent of dynamic
	// parent budgets.
	IaaSTimeoutStaticLimit = 2 * time.Minute

	// DefaultIaaSProviderTimeout is used when IaaS integration is enabled but
	// no explicit httpRequestTimeout is configured. It should cover the
	// provider's worst-case completion time (rate-limit queue wait + cloud API
	// call, ~46s with provider defaults) plus a small margin. The client
	// forwards its remaining budget via the X-Request-Timeout-Ms header and the
	// provider performs the authoritative budget check against its own
	// configured limits.
	DefaultIaaSProviderTimeout = 50 * time.Second
)
