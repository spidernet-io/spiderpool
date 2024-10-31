// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ethtool

import "github.com/safchain/ethtool"

func Stats(netIfName string) (map[string]uint64, error) {
	tool, err := ethtool.NewEthtool()
	if err != nil {
		return nil, err
	}
	defer tool.Close()
	stats, err := tool.Stats(netIfName)
	if err != nil {
		return nil, err
	}
	speed, err := tool.CmdGetMapped(netIfName)
	if err != nil {
		return nil, err
	}
	// speed unknown = 4294967295
	if val, ok := speed["speed"]; ok && val != 4294967295 {
		stats["vport_speed_mbps"] = val
	}
	return stats, nil
}
