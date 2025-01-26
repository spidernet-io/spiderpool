// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/mdlayher/ndp"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/errgroup"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
)

var (
	retryNum = 3
	timeOut  = 100 * time.Millisecond
)

type Detector struct {
	logger                                                                   *zap.Logger
	enableIPv4ConflictDetection, enableIPv6ConflictDetection                 bool
	enableIPv4GatewayReachableDetection, enableIPv6GatewayReachableDetection bool
	retries                                                                  int
	iface                                                                    string
	timeout                                                                  time.Duration
	ip4, ip6, v4Gw, v6Gw                                                     net.IP
}

func DetectIPConflictAndGatewayReachable(logger *zap.Logger, iface string, hostNs ns.NetNS, netns ns.NetNS, ipconfigs []*models.IPConfig) error {
	d := &Detector{
		retries: retryNum,
		timeout: timeOut,
		iface:   iface,
		logger:  logger,
	}

	var dectectIPs []*models.IPConfig
	for _, ipa := range ipconfigs {
		logger.Debug("IPAM Allocated Result", zap.Any("Result", ipa))
		if *ipa.Nic != iface {
			// spiderpool assigns IPs to all NICs in advance of the first call to ipam.
			// different NICs come from different pools, so we only need to focus on the current NIC's ipconfig.
			logger.Debug("In multi-cni mode, only the current CNI-assigned NIC will be detected for IPAM detection once", zap.String("nic", *ipa.Nic))
			continue
		}

		if !ipa.EnableGatewayDetection && !ipa.EnableIPConflictDetection {
			// IP conflict detection and gateway detection are disabled
			logger.Debug("IP and Gateway detection is disabled")
			continue
		}
		dectectIPs = append(dectectIPs, ipa)
	}

	if len(dectectIPs) == 0 {
		logger.Debug("IP conflict detection and gateway detection are disabled")
		return nil
	}

	errg := errgroup.Group{}
	err := netns.Do(func(_ ns.NetNS) error {
		for _, ipa := range dectectIPs {
			if ipa.Version == nil {
				return nil
			}
			ipaddress, _, err := net.ParseCIDR(*ipa.Address)
			if err != nil {
				return fmt.Errorf("failed to parse ipaddress %s: %w", *ipa.Address, err)
			}

			if *ipa.Version == int64(4) {
				d.ip4 = ipaddress
				d.enableIPv4ConflictDetection = ipa.EnableIPConflictDetection
				if ipa.Gateway != "" {
					d.enableIPv4GatewayReachableDetection = ipa.EnableGatewayDetection
					d.v4Gw = net.ParseIP(ipa.Gateway)
				}
				logger.Info("IPv4 Detection Configs",
					zap.String("iface", d.iface),
					zap.Any("IP", ipaddress.String()),
					zap.Any("Gateway", d.v4Gw),
					zap.Bool("IPConflictDetection", d.enableIPv4ConflictDetection),
					zap.Bool("GatewayDetection", d.enableIPv4GatewayReachableDetection),
				)
				if d.enableIPv4ConflictDetection || d.enableIPv4GatewayReachableDetection {
					errg.Go(hostNs, netns, d.ARPDetect)
				}
			} else if *ipa.Version == int64(6) {
				d.ip6 = ipaddress
				d.enableIPv6ConflictDetection = ipa.EnableIPConflictDetection
				if ipa.Gateway != "" {
					d.enableIPv6GatewayReachableDetection = ipa.EnableGatewayDetection
					d.v6Gw = net.ParseIP(ipa.Gateway)
				}

				logger.Info("IPv6 Detection Configs",
					zap.String("Interface", d.iface),
					zap.Any("IP", d.ip6),
					zap.Any("Gateway", d.v6Gw),
					zap.Bool("IPv6ConflictDetection", d.enableIPv6ConflictDetection),
					zap.Bool("IPv6GatewayDetection", d.enableIPv6GatewayReachableDetection),
				)
				if d.enableIPv6ConflictDetection || d.enableIPv6GatewayReachableDetection {
					errg.Go(hostNs, netns, d.NDPDetect)
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to init IP conflict and gateway detection: %w", err)
	}

	return errg.Wait()
}

func (d *Detector) ARPDetect() error {
	l, err := netlink.LinkByName(d.iface)
	if err != nil {
		d.logger.Error("failed to get link", zap.Error(err))
		return err
	}

	sock, err := networking.NewARPSockRAW(l)
	if err != nil {
		d.logger.Error("failed to create raw socket", zap.Error(err))
		return err
	}
	defer syscall.Close(sock)

	// IP conflict detection must precede gateway detection, which avoids the
	// possibility that gateway detection may update arp table entries first and cause
	// communication problems when IP conflict detection fails
	// see https://github.com/spidernet-io/spiderpool/issues/4475
	// call ip conflict detection
	if d.enableIPv4ConflictDetection {
		d.logger.Info("Detect IPAddress If Conflicts for IPv4", zap.String("IPAddress", d.ip4.String()))
		err = d.detectIP4Conflicting(sock, l)
		if err != nil {
			return err
		}
	} else {
		d.logger.Info("IPConflitingDetection is disabled for IPv4", zap.String("IPAddress", d.ip4.String()))
	}

	//  we do detect gateway connection lastly
	// Finally, there is gateway detection, which updates the correct arp table entries
	// once there are no IP address conflicts and fixed Mac addresses
	// call gateway detection
	if d.enableIPv4GatewayReachableDetection {
		d.logger.Info("Detect Gateway If reachable for IPv4", zap.String("IPAddress", d.ip4.String()), zap.String("Gateway", d.v4Gw.String()))
		if err = d.detectGateway4Reachable(sock, l); err != nil {
			return err
		}
	} else {
		d.logger.Info("GatewayDetection is disabled for IPv4", zap.String("IPAddress", d.ip4.String()), zap.String("Gateway", d.v4Gw.String()))
	}
	return nil
}

func (d *Detector) NDPDetect() error {
	ifi, err := net.InterfaceByName(d.iface)
	if err != nil {
		d.logger.Error("failed to InterfaceByName", zap.Error(err))
		return fmt.Errorf("failed to InterfaceByName %s: %w", d.iface, err)
	}

	var ndpClient *ndp.Conn
	// wait for ndp ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// When DAD(Duplicate Address Detection) is enanled, the kernel will check if this local link address is in conflict,
	// this may take a while, set the maximum timeout to 10s
	ndpReady := false
	for !ndpReady {
		select {
		case <-ctx.Done():
			d.logger.Error("Waiting for the maximum timeout of 10s, the state of the local link address is still not READY.")
			return fmt.Errorf("Waiting for the maximum timeout of 10s, the state of the local link address is still not READY.")
		default:
			ndpClient, _, err = ndp.Listen(ifi, ndp.LinkLocal)
			if err == nil {
				d.logger.Debug("ndp client is ready")
				ndpReady = true
			}
		}
	}
	defer ndpClient.Close()

	// IP conflict detection must precede gateway detection, which avoids the
	// possibility that gateway detection may update arp table entries first and cause
	// communication problems when IP conflict detection fails
	// see https://github.com/spidernet-io/spiderpool/issues/4475
	// call ip conflict detection
	if d.enableIPv6ConflictDetection {
		d.logger.Info("Detect IPAddress If conflict for IPv6", zap.String("IPAddress", d.ip6.String()))
		err = d.detectIP6Conflicting(ifi, ndpClient)
		if err != nil {
			return err
		}
	} else {
		d.logger.Info("IPConflitingDetection is disabled for IPv6", zap.String("IPAddress", d.ip6.String()))
	}

	// we do detect gateway connection lastly
	// Finally, there is gateway detection, which updates the correct arp table entries
	// once there are no IP address conflicts and fixed Mac addresses
	// call gateway detection
	if d.enableIPv6GatewayReachableDetection {
		d.logger.Info("Detecting Gateway if reachable for IPv6", zap.String("IPAddress", d.ip6.String()), zap.String("Gateway", d.v6Gw.String()))
		if err = d.detectGateway6Reachable(ifi, ndpClient); err != nil {
			return err
		}
	} else {
		d.logger.Info("GatewayDetection is disabled for IPv6", zap.String("IPAddress", d.ip6.String()), zap.String("Gateway", d.v6Gw.String()))
	}
	return nil
}

func (d *Detector) detectIP4Conflicting(sock int, l netlink.Link) error {
	var err error
	// Set a timeout of d.timeout for receiving packets
	if err = networking.SetSocketTimeout(sock, d.timeout); err != nil {
		return fmt.Errorf("failed to set socket timeout: %v", err)
	}

	// Buffer to hold incoming packets.
	buf := make([]byte, 65536)

	for i := 0; i < d.retries; i++ {
		// we send a gratuitous arp to checking if ip is conflict
		// we use dad mode(duplicate address detection mode), so
		// we set source ip to 0.0.0.0
		err = networking.SendARPReuqest(l, net.ParseIP("0.0.0.0"), d.ip4)
		if err != nil {
			d.logger.Error("failed to send ARP request, retrying...", zap.Error(err))
			continue
		}

		d.logger.Debug("success to send ARP request to detecting if the IPv4 address conflicts")

		for {
			// Read a packet from the socket.
			n, _, err := syscall.Recvfrom(sock, buf, 0)
			if err == nil {
				if n < 28 || buf[20] != 0x00 || buf[21] != 0x02 {
					// Check if the packet is an ARP response (operation code 2).
					continue
				}

				// Now we catch an ARP response
				// Extract sender's IP and MAC addresses from the ARP response.
				senderMAC := net.HardwareAddr(buf[22:28])
				senderIP := net.IP(buf[28:32])

				d.logger.Debug("Received packet from sender", zap.String("senderIP", senderIP.String()), zap.String("senderMAC", senderMAC.String()))
				// Check if the sender's MAC address is the same as the interface's address
				if senderIP.Equal(d.ip4) {
					// found ip conflicting
					d.logger.Error("IPv4 IPAddress Conflicts", zap.String("Conflicting IP", d.ip4.String()), zap.String("Host", senderMAC.String()))
					return fmt.Errorf("%w: pod's interface %s with an conflicting ip %s, %s is located at %s",
						constant.ErrIPConflict, d.iface, d.ip4.String(), d.ip4.String(), senderMAC.String())
				}
				continue
			}

			if err.(net.Error).Timeout() {
				// If an arp reply is not received within the timeout period or is not
				// a expected arp reply
				d.logger.Info("No IPv4 address conflicts")
				return nil
			}

			d.logger.Error("failed to receive packet, retrying...", zap.Error(err))
		}
	}

	if err != nil {
		d.logger.Error("after failed to send three ARP request packages, can't detect IPv4 address conflicting", zap.Error(err))
		return fmt.Errorf("after failed to send three ARP request packages, can't detect IPv4 address conflicting: %w", err)
	}
	return nil
}

func (d *Detector) detectGateway4Reachable(sock int, l netlink.Link) error {
	var err error
	// Buffer to hold incoming packets.
	buf := make([]byte, 65536)

	// Set a timeout of d.timeout for receiving packets
	if err := networking.SetSocketTimeout(sock, d.timeout); err != nil {
		return fmt.Errorf("failed to set socket timeout: %v", err)
	}

	for i := 0; i < d.retries; i++ {
		if err = networking.SendARPReuqest(l, d.ip4, d.v4Gw); err != nil {
			d.logger.Error("failed to send ARP request, retrying...", zap.Error(err))
			continue
		}
		d.logger.Debug("success to send ARP request to detect gateway reachable")

		for {
			// Read a packet from the socket.
			n, _, err := syscall.Recvfrom(sock, buf, 0)
			if err == nil {
				if n < 28 || buf[20] != 0x00 || buf[21] != 0x02 {
					// Check if the packet is an ARP response (operation code 2).
					continue
				}

				// Now we catch an ARP response
				// Extract sender's IP and MAC addresses from the ARP response.
				senderMAC := net.HardwareAddr(buf[22:28])
				senderIP := net.IP(buf[28:32])

				d.logger.Debug("Received packet from sender", zap.String("senderIP", senderIP.String()), zap.String("senderMAC", senderMAC.String()))

				// Check if the sender's MAC address is the same as the interface's address
				if senderIP.Equal(d.v4Gw) {
					d.logger.Sugar().Infof("Gateway %s is reachable, gateway is located at %v", d.v4Gw, senderMAC.String())
					return nil
				}
				continue
			}

			if err.(net.Error).Timeout() {
				// If an arp reply is not received within the timeout period or is not
				// sent from the gateway IP, it is assumed that the gateway is not reachable.
				d.logger.Sugar().Errorf("gateway %s is %v, reason: %v", d.v4Gw.String(), constant.ErrGatewayUnreachable, err)
				return fmt.Errorf("gateway %s is %w", d.v4Gw.String(), constant.ErrGatewayUnreachable)
			}
			d.logger.Error("failed to receive packet, retring", zap.Error(err))
		}
	}

	if err != nil {
		d.logger.Error("after failed to send three ARP request packages, can't detect gateway reachable", zap.Error(err))
		return fmt.Errorf("after failed to send three ARP request packages, can't detect gateway reachable: %w", err)
	}

	return nil
}

func (d *Detector) detectIP6Conflicting(ifi *net.Interface, ndpClient *ndp.Conn) error {
	if err := ndpClient.SetReadDeadline(time.Now().Add(d.timeout)); err != nil {
		d.logger.Error("failed to set read deadline", zap.Error(err))
	}
	var err error
	for i := 0; i < d.retries; i++ {
		err = networking.SendUnsolicitedNeighborAdvertisement(d.ip6, ifi, ndpClient)
		if err != nil {
			d.logger.Error("failed to send unsolicited neighbor advertisement, retrying...", zap.Error(err))
			continue
		}
		d.logger.Info("success to send unsolicited neighbor advertisement")
		for {
			msg, _, _, err := ndpClient.ReadFrom()
			if err == nil {
				na, ok := msg.(*ndp.NeighborAdvertisement)
				if !ok || na.TargetAddress.String() != d.ip6.String() || len(na.Options) != 1 {
					continue
				}

				option, ok := na.Options[0].(*ndp.LinkLayerAddress)
				if ok {
					d.logger.Error("IPv6 address conflicts", zap.String("Conflicting IP", d.ip6.String()), zap.String("Host", option.Addr.String()))
					return fmt.Errorf("%w: pod's interface %s with an conflicting ip %s, %s is located at %s", constant.ErrIPConflict, d.iface, d.ip6.String(), d.ip6.String(), option.Addr.String())
				}
				continue
			}

			// no ndp response unitil timeout, indicates gateway unreachable
			if err.(net.Error).Timeout() {
				d.logger.Info("No IPv6 address conflicts")
				return nil
			}
			// retry it if is other error
			d.logger.Error("failed to receive unsolicited neighbor advertisement message, retrying...", zap.Error(err))
		}
	}

	if err != nil {
		d.logger.Error("after failed to send three unsolicited neighbor advertisement packages, can't detect IPv6 address conflicting", zap.Error(err))
		return fmt.Errorf("after failed to send three unsolicited neighbor advertisement packages, can't detect IPv6 address conflicting: %w", err)
	}

	return nil
}

func (d *Detector) detectGateway6Reachable(ifi *net.Interface, ndpClient *ndp.Conn) error {
	err := ndpClient.SetReadDeadline(time.Now().Add(d.timeout))
	if err != nil {
		d.logger.Error("failed to set read deadline", zap.Error(err))
	}
	for i := 0; i < d.retries; i++ {
		err = networking.SendUnsolicitedNeighborAdvertisement(d.v6Gw, ifi, ndpClient)
		if err != nil {
			d.logger.Error("failed to send unsolicited neighbor advertisement, retrying...", zap.Error(err))
			continue
		}

		d.logger.Info("success to send unsolicited neighbor advertisement")
		for {
			msg, _, _, err := ndpClient.ReadFrom()
			if err == nil {
				na, ok := msg.(*ndp.NeighborAdvertisement)
				if !ok || na.TargetAddress.String() != d.v6Gw.String() || len(na.Options) != 1 {
					continue
				}

				option, ok := na.Options[0].(*ndp.LinkLayerAddress)
				if ok {
					d.logger.Sugar().Infof("gateway %s is located at %s", d.v6Gw.String(), option.Addr.String())
					return nil
				}
				continue
			}

			// no ndp response unitil timeout, indicates gateway unreachable
			if err.(net.Error).Timeout() {
				d.logger.Sugar().Errorf("gateway %s is %s, reason: %v", d.v6Gw.String(), constant.ErrGatewayUnreachable, err)
				return fmt.Errorf("gateway %s is %w", d.v6Gw.String(), constant.ErrGatewayUnreachable)
			}
			// retry it if is other error
			d.logger.Error("failed to receive unsolicited neighbor advertisement message, retrying...", zap.Error(err))
		}
	}

	if err != nil {
		d.logger.Error("after failed to send three unsolicited neighbor advertisement packages, can't detect IPv6 Gateway if reachable", zap.Error(err))
		return fmt.Errorf("after failed to send three unsolicited neighbor advertisement packages, can't detect IPv6 Gateway if reachable: %w", err)
	}
	return nil
}
