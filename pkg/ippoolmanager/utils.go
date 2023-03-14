// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

// TODO(Icarus9913): Deprecated.
func IsAutoCreatedIPPool(pool *spiderpoolv1.SpiderIPPool) bool {
	// only the auto-created IPPool owns the label "ipam.spidernet.io/owner-application"
	poolLabels := pool.GetLabels()
	_, ok := poolLabels[constant.LabelIPPoolOwnerApplication]
	return ok
}
