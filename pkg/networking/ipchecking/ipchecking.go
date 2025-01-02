// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipchecking

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/ndp"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/errgroup"
	"go.uber.org/zap"
)

type IPChecker struct {
	retries       int
	interval      time.Duration
	timeout       time.Duration
	netns, hostNs ns.NetNS
	ip4, ip6      netip.Addr
	ifi           *net.Interface
	arpClient     *arp.Client
	ndpClient     *ndp.Conn
	logger        *zap.Logger
}

func NewIPChecker(retries int, interval, timeout string, hostNs, netns ns.NetNS, logger *zap.Logger) (*IPChecker, error) {
	var err error

	ipc := new(IPChecker)
	ipc.retries = retries
	ipc.interval, err = time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interval %v: %v", interval, err)
	}

	ipc.timeout, err = time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timeoute %v: %v", timeout, err)
	}

	if err != nil {
		return nil, err
	}

	ipc.hostNs = hostNs
	ipc.netns = netns
	ipc.logger = logger
	return ipc, nil
}

func (ipc *IPChecker) DoIPConflictChecking(ipconfigs []*types100.IPConfig, iface string, errg *errgroup.Group) {
	ipc.logger.Debug("DoIPConflictChecking", zap.String("interval", ipc.interval.String()), zap.Int("retries", ipc.retries), zap.String("timeout", ipc.timeout.String()))
	if len(ipconfigs) == 0 {
		ipc.logger.Info("No ips found in pod, ignore pod ip's conflict checking")
		return
	}

	var err error
	_ = ipc.netns.Do(func(netNS ns.NetNS) error {
		ipc.ifi, err = net.InterfaceByName(iface)
		if err != nil {
			return fmt.Errorf("failed to InterfaceByName %s: %w", iface, err)
		}

		for idx := range ipconfigs {
			target := netip.MustParseAddr(ipconfigs[idx].Address.IP.String())
			if target.Is4() {
				ipc.logger.Debug("IPCheckingByARP", zap.String("ipv4 address", target.String()))
				ipc.ip4 = target
				ipc.arpClient, err = arp.Dial(ipc.ifi)
				if err != nil {
					return fmt.Errorf("failed to init arp client: %w", err)
				}
				errg.Go(ipc.hostNs, ipc.netns, ipc.ipCheckingByARP)
			} else {
				ipc.logger.Debug("IPCheckingByNDP", zap.String("ipv6 address", target.String()))
				ipc.ip6 = target
				ipc.ndpClient, _, err = ndp.Listen(ipc.ifi, ndp.LinkLocal)
				if err != nil {
					return fmt.Errorf("failed to init ndp client: %w", err)
				}
				errg.Go(ipc.hostNs, ipc.netns, ipc.ipCheckingByNDP)
			}
		}
		return nil
	})
}

func (ipc *IPChecker) ipCheckingByARP() error {
	defer ipc.arpClient.Close()

	var err error
	for i := 0; i < ipc.retries; i++ {
		ipc.logger.Sugar().Debugf("[Retry: %v]try to arping the ip", i+1)
		if err = ipc.arpClient.SetDeadline(time.Now().Add(ipc.timeout)); err != nil {
			ipc.logger.Error("[ARP]failed to set deadline", zap.Error(err))
			continue
		}

		// we send a gratuitous arp to checking if ip is conflict
		// we use dad mode(duplicate address detection mode), so
		// we set source ip to 0.0.0.0
		packet, err := arp.NewPacket(arp.OperationRequest, ipc.ifi.HardwareAddr, netip.MustParseAddr("0.0.0.0"), ethernet.Broadcast, ipc.ip4)
		if err != nil {
			return err
		}

		err = ipc.arpClient.WriteTo(packet, ethernet.Broadcast)
		if err != nil {
			ipc.logger.Error("[ARP]failed to send message", zap.Error(err))
			continue
		}

		packet, _, err = ipc.arpClient.Read()
		if err != nil {
			ipc.logger.Error("[ARP]failed to receive message", zap.Error(err))
			continue
		}

		if packet.Operation != arp.OperationReply || packet.SenderIP.Compare(ipc.ip4) != 0 {
			continue
		}

		// found ip conflicting
		ipc.logger.Error("Found IPv4 address conflicting", zap.String("Conflicting IP", ipc.ip4.String()), zap.String("Host", packet.SenderHardwareAddr.String()))
		return fmt.Errorf("%w: pod's interface %s with an conflicting ip %s, %s is located at %s",
			constant.ErrIPConflict, ipc.ifi.Name, ipc.ip4.String(), ipc.ip4.String(), packet.SenderHardwareAddr.String())
	}

	if err != nil {
		if neterr, ok := err.(net.Error); ok && !neterr.Timeout() {
			return fmt.Errorf("failed to checking ip %s if it's conflicting: %v", ipc.ip4.String(), err)
		}
	}

	ipc.logger.Debug("No ipv4 address conflict", zap.String("IPv4 address", ipc.ip4.String()))
	return nil
}

var errRetry = errors.New("retry")
var NDPFoundReply error = errors.New("found ndp reply")

func (ipc *IPChecker) ipCheckingByNDP() error {
	var err error
	defer ipc.ndpClient.Close()

	m := &ndp.NeighborSolicitation{
		TargetAddress: ipc.ip6,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      ipc.ifi.HardwareAddr,
			},
		},
	}

	var replyMac string
	replyMac, err = ipc.sendReceiveLoop(m)
	if err != nil {
		if err.Error() == NDPFoundReply.Error() {
			if replyMac != ipc.ifi.HardwareAddr.String() {
				ipc.logger.Error("Found IPv6 address conflicting", zap.String("Conflicting IP", ipc.ip6.String()), zap.String("Host", replyMac))
				return fmt.Errorf("pod's interface %s with an conflicting ip %s, %s is located at %s", ipc.ifi.Name,
					ipc.ip6.String(), ipc.ip6.String(), replyMac)
			}
		}
	}

	// no ipv6 conflicting
	ipc.logger.Debug("No ipv6 address conflicting", zap.String("ipv6 address", ipc.ip6.String()))
	return nil
}

// sendReceiveLoop send ndp message and waiting for receive.
// Copyright Authors of mdlayher/ndp: https://github.com/mdlayher/ndp/
func (ipc *IPChecker) sendReceiveLoop(msg ndp.Message) (string, error) {
	var hwAddr string
	var err error

	for i := 0; i < ipc.retries; i++ {
		ipc.logger.Sugar().Debugf("[Retry: %v]try to ndping the ip", i+1)
		hwAddr, err = ipc.sendReceive(msg)
		switch err {
		case errRetry:
			continue
		case nil:
			return hwAddr, NDPFoundReply
		default:
			// Was the error caused by a read timeout, and should the loop continue?
			if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				ipc.logger.Error(err.Error())
				continue
			}
			return "", err
		}
	}

	return "", nil
}

// sendReceive send and receive ndp message,return error if error occurred.
// if the returned string isn't empty, it indicates that there are an
// IPv6 address conflict.
// Copyright Authors of mdlayher/ndp: https://github.com/mdlayher/ndp/
func (ipc *IPChecker) sendReceive(m ndp.Message) (string, error) {
	// Always multicast the message to the target's solicited-node multicast
	// group as if we have no knowledge of its MAC address.
	snm, err := ndp.SolicitedNodeMulticast(ipc.ip6)
	if err != nil {
		ipc.logger.Error("[NDP]failed to determine solicited-node multicast address", zap.Error(err))
		return "", fmt.Errorf("failed to determine solicited-node multicast address: %v", err)
	}

	// we send a gratuitous neighbor solicitation to checking if ip is conflict
	err = ipc.ndpClient.WriteTo(m, nil, snm)
	if err != nil {
		ipc.logger.Error("[NDP]failed to send message", zap.Error(err))
		return "", fmt.Errorf("failed to send message: %v", err)
	}

	if err := ipc.ndpClient.SetReadDeadline(time.Now().Add(ipc.timeout)); err != nil {
		ipc.logger.Error("[NDP]failed to set deadline", zap.Error(err))
		return "", fmt.Errorf("failed to set deadline: %v", err)
	}

	msg, _, _, err := ipc.ndpClient.ReadFrom()
	if err == nil {
		na, ok := msg.(*ndp.NeighborAdvertisement)
		if ok && na.TargetAddress.Compare(ipc.ip6) == 0 && len(na.Options) == 1 {
			// found ndp reply what we want
			option, ok := na.Options[0].(*ndp.LinkLayerAddress)
			if ok {
				return option.Addr.String(), nil
			}
		}
		return "", errRetry
	}
	return "", err
}
