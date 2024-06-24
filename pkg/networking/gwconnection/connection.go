// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gwconnection

import (
	"fmt"
	"net"
	"net/netip"
	"time"

	"go.uber.org/zap"

	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/mdlayher/arp"
	_ "github.com/mdlayher/ethernet"
	"github.com/mdlayher/ndp"
	"github.com/spidernet-io/spiderpool/pkg/constant"
)

type DetectGateway struct {
	retries                    int
	iface                      string
	interval                   time.Duration
	timeout                    time.Duration
	v4Addr, v6Addr, V4Gw, V6Gw net.IP
	logger                     *zap.Logger
}

func New(retries int, interval, timeout, iface string, logger *zap.Logger) (*DetectGateway, error) {
	var err error
	dg := &DetectGateway{
		retries: retries,
		iface:   iface,
	}

	dg.interval, err = time.ParseDuration(interval)
	if err != nil {
		return nil, err
	}

	dg.timeout, err = time.ParseDuration(timeout)
	if err != nil {
		return nil, err
	}
	dg.logger = logger

	return dg, nil
}

func (dg *DetectGateway) ParseAddrFromPreresult(ipconfigs []*types100.IPConfig) {
	for _, ipconfig := range ipconfigs {
		if ipconfig.Address.IP.To4() != nil {
			dg.v4Addr = ipconfig.Address.IP
		} else {
			dg.v6Addr = ipconfig.Address.IP
		}
	}
}

// PingOverIface sends an arp ping over interface 'iface' to 'dstIP'
func (dg *DetectGateway) ArpingOverIface() error {
	ifi, err := net.InterfaceByName(dg.iface)
	if err != nil {
		return err
	}

	client, err := arp.Dial(ifi)
	if err != nil {
		return err
	}
	defer client.Close()

	gwNetIP := netip.MustParseAddr(dg.V4Gw.String())
	var gwHwAddr net.HardwareAddr
	for i := 0; i < dg.retries; i++ {

		err = client.SetReadDeadline(time.Now().Add(dg.timeout))
		if err != nil {
			dg.logger.Sugar().Errorf("[RetryNum: %v]failed to set ReadDeadline: %v", i+1, err)
			time.Sleep(dg.interval)
			continue
		}

		dg.logger.Sugar().Debugf("[RetryNum: %v]try to arping the gateway", i+1)
		gwHwAddr, err = client.Resolve(gwNetIP)
		if err != nil {
			dg.logger.Sugar().Errorf("[RetryNum: %v]failed to resolve: %v", i+1, err)
			time.Sleep(dg.interval)
			continue
		}

		if gwHwAddr != nil {
			dg.logger.Sugar().Infof("Gateway %s is reachable, gateway is located at %v", gwNetIP, gwHwAddr.String())
			return nil
		}
		time.Sleep(dg.interval)
	}

	if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
		dg.logger.Sugar().Errorf("gateway %s is %v, reason: %v", dg.V4Gw.String(), err)
		return fmt.Errorf("gateway %s is %v", dg.V4Gw.String(), constant.ErrGatewayUnreachable)
	}

	return fmt.Errorf("failed to checking the gateway %s if is reachable: %w", dg.V4Gw.String(), err)
}

func (dg *DetectGateway) NDPingOverIface() error {
	ifi, err := net.InterfaceByName(dg.iface)
	if err != nil {
		return err
	}

	client, _, err := ndp.Listen(ifi, ndp.LinkLocal)
	if err != nil {
		return err
	}
	defer client.Close()

	msg := &ndp.NeighborSolicitation{
		TargetAddress: netip.MustParseAddr(dg.V6Gw.String()),
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      ifi.HardwareAddr,
			},
		},
	}

	ticker := time.NewTicker(dg.interval)
	defer ticker.Stop()

	var gwHwAddr string
	for i := 0; i < dg.retries && gwHwAddr == ""; i++ {
		<-ticker.C
		gwHwAddr, err = dg.sendReceive(client, msg)
		if err != nil {
			dg.logger.Sugar().Errorf("[retry number: %v]error detect if gateway is reachable: %v", i+1, err)
		} else if gwHwAddr != "" {
			dg.logger.Sugar().Infof("gateway %s is reachable, it's located at %s", dg.V6Gw.String(), gwHwAddr)
			return nil
		}
	}

	if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
		dg.logger.Sugar().Errorf("gateway %s is unreachable, reason: %v", dg.V6Gw.String(), err)
		return fmt.Errorf("gateway %s is %w", dg.V6Gw.String(), constant.ErrGatewayUnreachable)
	}
	return fmt.Errorf("error detect the gateway %s if is reachable: %v", dg.V6Gw.String(), err)
}

func (dg *DetectGateway) sendReceive(client *ndp.Conn, m ndp.Message) (string, error) {
	gwNetIP := netip.MustParseAddr(dg.V6Gw.String())
	// Always multicast the message to the target's solicited-node multicast
	// group as if we have no knowledge of its MAC address.
	snm, err := ndp.SolicitedNodeMulticast(gwNetIP)
	if err != nil {
		dg.logger.Error("[NDP]failed to determine solicited-node multicast address", zap.Error(err))
		return "", fmt.Errorf("failed to determine solicited-node multicast address: %v", err)
	}

	// we send a gratuitous neighbor solicitation to checking if ip is conflict
	err = client.WriteTo(m, nil, snm)
	if err != nil {
		dg.logger.Error("[NDP]failed to send message", zap.Error(err))
		return "", fmt.Errorf("failed to send message: %v", err)
	}

	if err := client.SetReadDeadline(time.Now().Add(dg.timeout)); err != nil {
		dg.logger.Error("[NDP]failed to set deadline", zap.Error(err))
		return "", fmt.Errorf("failed to set deadline: %v", err)
	}

	msg, _, _, err := client.ReadFrom()
	if err != nil {
		return "", err
	}

	gwAddr := netip.MustParseAddr(dg.V6Gw.String())
	na, ok := msg.(*ndp.NeighborAdvertisement)
	if ok && na.TargetAddress.Compare(gwAddr) == 0 && len(na.Options) == 1 {
		dg.logger.Debug("Detect gateway: found the response", zap.String("TargetAddress", na.TargetAddress.String()))
		// found ndp reply what we want
		option, ok := na.Options[0].(*ndp.LinkLayerAddress)
		if ok {
			return option.Addr.String(), nil
		}
	}
	return "", nil
}
