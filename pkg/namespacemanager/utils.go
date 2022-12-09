// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func GetNSDefaultPools(ns *corev1.Namespace) ([]string, []string, error) {
	if ns == nil {
		return nil, nil, fmt.Errorf("namespace %w", constant.ErrMissingRequiredParam)
	}

	var nsDefaultV4Pool types.AnnoNSDefautlV4PoolValue
	var nsDefaultV6Pool types.AnnoNSDefautlV6PoolValue
	if v, ok := ns.Annotations[constant.AnnoNSDefautlV4Pool]; ok {
		if err := json.Unmarshal([]byte(v), &nsDefaultV4Pool); err != nil {
			return nil, nil, err
		}
	}

	if v, ok := ns.Annotations[constant.AnnoNSDefautlV6Pool]; ok {
		if err := json.Unmarshal([]byte(v), &nsDefaultV6Pool); err != nil {
			return nil, nil, err
		}
	}

	return nsDefaultV4Pool, nsDefaultV6Pool, nil
}
