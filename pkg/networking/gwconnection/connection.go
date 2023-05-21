package gwconnection

import (
	"fmt"
	"time"

	"github.com/go-ping/ping"
)

func DetectGatewayConnection(gw string) error {
	pingCtl := ping.New(gw)
	pingCtl.Interval = 100 * time.Millisecond
	pingCtl.Count = 3
	pingCtl.Timeout = 2 * time.Second
	if err := pingCtl.Run(); err != nil {
		return fmt.Errorf("failed to DetectGatewayConnection: %v", err)
	}

	stats := pingCtl.Statistics()
	if stats.PacketLoss > 0 {
		return fmt.Errorf("gateway: %s is unreachable", gw)
	}
	return nil
}
