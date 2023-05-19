package ipchecking

import (
	"context"
	"errors"
	"fmt"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/ndp"
	"go.uber.org/zap"
	"net"
	"net/netip"
	"time"
)

func DoIPConflictChecking(logger *zap.Logger, netns ns.NetNS, retries int, interval, iface string, ipconfigs []*types100.IPConfig) error {
	logger.Debug("DoIPConflictChecking", zap.String("interval", interval), zap.Int("retries", retries))

	if len(ipconfigs) == 0 {
		return fmt.Errorf("interface %s has no any ip configured", iface)
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("failed to parse interval %v: %v", interval, err)
	}

	return netns.Do(func(netNS ns.NetNS) error {
		ifi, err := net.InterfaceByName(iface)
		if err != nil {
			return fmt.Errorf("failed to get interface by name %s: %v", iface, err)
		}

		for idx, _ := range ipconfigs {
			target := netip.MustParseAddr(ipconfigs[idx].Address.IP.String())
			if target.Is4() {
				logger.Debug("IPCheckingByARP", zap.String("address", target.String()))
				err = IPCheckingByARP(ifi, target, retries, duration)
				if err != nil {
					return err
				}
				logger.Debug("No IPv4 address conflicting", zap.String("address", target.String()))
			} else {
				logger.Debug("IPCheckingByNDP", zap.String("address", target.String()))
				err = IPCheckingByNDP(ifi, target, retries, duration)
				if err != nil {
					return err
				}
				logger.Debug("No IPv6 address conflicting", zap.String("address", target.String()))
			}
		}
		return nil
	})
}

func IPCheckingByARP(ifi *net.Interface, targetIP netip.Addr, retry int, interval time.Duration) error {
	client, err := arp.Dial(ifi)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var conflictingMac string
	// start a goroutine to receive arp response
	go func() {
		var packet *arp.Packet
		for {
			select {
			case <-ctx.Done():
				return
			default:
				packet, _, err = client.Read()
				if err != nil {
					cancel()
					return
				}

				if packet.Operation == arp.OperationReply {
					// found reply and simple check if the reply packet is we want.
					if packet.SenderIP.Compare(targetIP) == 0 {
						conflictingMac = packet.SenderHardwareAddr.String()
						cancel()
						return
					}
				}
			}
		}
	}()

	// we send a gratuitous arp to checking if ip is conflict
	// we use dad mode(duplicate address detection mode), so
	// we set source ip to 0.0.0.0
	packet, err := arp.NewPacket(arp.OperationRequest, ifi.HardwareAddr, netip.MustParseAddr("0.0.0.0"), ethernet.Broadcast, targetIP)
	if err != nil {
		cancel()
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	stop := false
	for i := 0; i < retry && !stop; i++ {
		select {
		case <-ctx.Done():
			stop = true
		case <-ticker.C:
			err = client.WriteTo(packet, ethernet.Broadcast)
			if err != nil {
				stop = true
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to checking ip %s if it's conflicting: %v", targetIP.String(), err)
	}

	if conflictingMac != "" {
		// found ip conflicting
		return fmt.Errorf("pod's interface %s with an conflicting ip %s, %s is located at %s", ifi.Name,
			targetIP.String(), targetIP.String(), conflictingMac)
	}

	return nil
}

var errRetry = errors.New("retry")
var NDPFoundReply error = errors.New("found ndp reply")
var NDPFoundError error = errors.New("found err")
var NDPRetryError error = errors.New("ip conflicting check fails with more than maximum number of retries")

func IPCheckingByNDP(ifi *net.Interface, target netip.Addr, retry int, interval time.Duration) error {
	client, _, err := ndp.Listen(ifi, ndp.LinkLocal)
	if err != nil {
		return err
	}
	defer client.Close()

	m := &ndp.NeighborSolicitation{
		TargetAddress: target,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      ifi.HardwareAddr,
			},
		},
	}

	var replyMac string
	replyMac, err = sendReceiveLoop(retry, interval, client, m, target)
	switch err {
	case NDPFoundReply:
		if replyMac != ifi.HardwareAddr.String() {
			return fmt.Errorf("pod's interface %s with an conflicting ip %s, %s is located at %s", ifi.Name,
				target.String(), target.String(), replyMac)
		}
	case NDPRetryError:
		return NDPRetryError
	default:
		return fmt.Errorf("failed to checking ip conflicting: %v", err)
	}

	return nil
}

func sendReceiveLoop(retry int, interval time.Duration, client *ndp.Conn, msg ndp.Message, dst netip.Addr) (string, error) {
	var hwAddr string
	var err error
	for i := 0; i < retry; i++ {
		hwAddr, err = sendReceive(client, msg, dst, interval)
		switch err {
		case errRetry:
			continue
		case nil:
			return hwAddr, NDPFoundReply
		default:
			return "", err
		}
	}

	return "", NDPRetryError
}

func sendReceive(client *ndp.Conn, m ndp.Message, target netip.Addr, interval time.Duration) (string, error) {
	// Always multicast the message to the target's solicited-node multicast
	// group as if we have no knowledge of its MAC address.
	snm, err := ndp.SolicitedNodeMulticast(target)
	if err != nil {
		return "", fmt.Errorf("failed to determine solicited-node multicast address: %v", err)
	}

	// we send a gratuitous neighbor solicitation to checking if ip is conflict
	err = client.WriteTo(m, nil, snm)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %v", err)
	}

	if err := client.SetReadDeadline(time.Now().Add(interval)); err != nil {
		return "", fmt.Errorf("failed to set deadline: %v", err)
	}

	msg, _, _, err := client.ReadFrom()
	if err == nil {
		na, ok := msg.(*ndp.NeighborAdvertisement)
		if ok && na.TargetAddress.Compare(target) == 0 && len(na.Options) == 1 {
			// found ndp reply what we want
			option, ok := na.Options[0].(*ndp.LinkLayerAddress)
			if ok {
				return option.Addr.String(), nil
			}
		}
		return "", errRetry

	}

	// Was the error caused by a read timeout, and should the loop continue?
	if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
		return "", errRetry
	}

	return "", fmt.Errorf("failed to read message: %v", err)
}
