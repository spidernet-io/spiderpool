// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"github.com/safchain/ethtool"
)

func EthtoolGetBusInfoByInterfaceName(ifName string) (string, error) {
	ethHandle, err := ethtool.NewEthtool()
	if err != nil {
		return "", err
	}
	defer ethHandle.Close()

	return ethHandle.BusInfo(ifName)
}
