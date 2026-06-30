// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"log"

	"github.com/go-logr/stdr"
	. "github.com/onsi/ginkgo/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	ctrl.SetLogger(stdr.New(log.New(GinkgoWriter, "[controller-runtime] ", 0)))
}
