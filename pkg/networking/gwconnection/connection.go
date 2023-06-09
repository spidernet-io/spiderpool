// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gwconnection

import (
	"fmt"
	"time"

	"github.com/prometheus-community/pro-bing"
)

func DetectGatewayConnection(gw string) error {
	pingCtl := probing.New(gw)
	pingCtl.Interval = 100 * time.Millisecond
	pingCtl.Count = 3
	pingCtl.Timeout = 2 * time.Second

	if err := pingCtl.Run(); err != nil {
		return fmt.Errorf("failed to DetectGatewayConnection: %v", err)
	}

	if pingCtl.Statistics().PacketLoss > 0 {
		return fmt.Errorf("gateway %s is unreachable", gw)
	}
	return nil
}
