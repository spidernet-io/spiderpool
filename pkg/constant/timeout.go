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

	// IaaSProviderRateLimitWait is the maximum time the IaaS provider will wait
	// for a rate-limit token bucket slot before rejecting the request.
	IaaSProviderRateLimitWait = 30 * time.Second

	// IaaSProviderCloudAPITimeout is the maximum time the IaaS provider allows
	// for the underlying cloud API call to complete.
	IaaSProviderCloudAPITimeout = 16 * time.Second

	// IaaSProviderWorstCase is the maximum end-to-end time for a single IaaS
	// provider request: rate-limit wait + cloud API call + a small network margin.
	// Any parent context with less remaining budget than this cannot guarantee
	// the provider request will complete.
	IaaSProviderWorstCase = IaaSProviderRateLimitWait + IaaSProviderCloudAPITimeout + 2*time.Second // 48s

	// DefaultIaaSProviderTimeout is used when IaaS integration is enabled but
	// no explicit httpRequestTimeout is configured. Set to IaaSProviderWorstCase
	// plus a small margin so a single provider call has a safe default budget.
	DefaultIaaSProviderTimeout = 50 * time.Second
)
