// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gwconnection

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	ping "github.com/prometheus-community/pro-bing"
)

type Pinger struct {
	logger *zap.Logger
	pinger *ping.Pinger
}

func NewPinger(count int, interval, timeout, gw string, logger *zap.Logger) (*Pinger, error) {
	pinger := ping.New(gw)
	pinger.Count = count

	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, err
	}
	pinger.Interval = intervalDuration

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, err
	}
	pinger.Timeout = timeoutDuration
	pinger.SetPrivileged(true)

	return &Pinger{logger, pinger}, nil
}

func (p *Pinger) DetectGateway() error {
	if err := p.pinger.Run(); err != nil {
		return fmt.Errorf("failed to run DetectGateway: %v", err)
	}

	stats := p.pinger.Statistics()
	if stats.PacketLoss > 0 {
		return fmt.Errorf("gateway %s is unreachable", p.pinger.Addr())
	}

	p.logger.Sugar().Debugf("gateway %s is reachable", p.pinger.Addr())
	return nil
}
