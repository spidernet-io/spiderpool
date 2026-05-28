// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package integration holds real-netns tests for pkg/networking/networking.
// Separated from the parent package's gomonkey-mocked unit tests because
// testutils.NewNS() leaks an OS thread by design — it locks the new
// thread to the new netns and never unlocks, so Go retires the thread on
// goroutine exit (see containernetworking/plugins/pkg/testutils/netns_linux.go:116-118).
// That thread churn destabilizes gomonkey's machine-code patches across
// ginkgo's randomized spec ordering; with both styles in one suite, ~40%
// of seeds fail. Running them as separate ginkgo suites (separate test
// binaries via `-r`) fully isolates them.
//
// Runs as part of `make unittest-tests` via the same `ginkgo -r ./pkg
// ./cmd` invocation the Makefile drives; the `unittest` label on the
// suite test mirrors `pkg/ip` and friends so the standard target picks
// it up.
package integration
