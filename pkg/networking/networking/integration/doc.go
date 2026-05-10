// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package integration holds real-netns tests for pkg/networking/networking.
// These run as their own ginkgo suite (separate test binary) so the
// machine-code patching used by sibling gomonkey-based unit tests stays
// isolated from the OS-thread lifecycle of testutils.NewNS.
//
// Runs as part of `make unittest-tests` via the same ginkgo -r invocation;
// label "unittest" so it's picked up by the standard test target.
package integration
