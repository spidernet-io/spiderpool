// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Note: The following source files are come from the latest version of https://github.com/k8snetworkplumbingwg/sriov-cni/blob/master/pkg/utils/packet.go.
// We can't directly go mod import package, it reports error:
// "require github.com/k8snetworkplumbingwg/sriov-cni: version “v2.8.0” invalid: should be v0 or v1, not v2.""

// So we copied the source files here and made some code changes.

package networking

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/vishvananda/netlink"

	"github.com/mdlayher/ndp"
)

var (
	arpPacketName    = "ARP"
	icmpV6PacketName = "ICMPv6"
)

// SetSocketTimeout sets the timeout for a socket in nanoseconds.
func SetSocketTimeout(sock int, timeout time.Duration) error {
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	return syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
}

// SendARPReuqest sends a gratuitous ARP packet with the provided source IP over the provided interface.
// UPDATE: the golang arp library requires an IPv4 address to exist for the NIC to send ARP request packets.
func SendARPReuqest(l netlink.Link, srcIP, dstIP net.IP) error {
	/* As per RFC 5944 section 4.6, a gratuitous ARP packet can be sent by a node in order to spontaneously cause other nodes to update
	 * an entry in their ARP cache. In the case of SRIOV-CNI, an address can be reused for different pods. Each pod could likely have a
	 * different link-layer address in this scenario, which makes the ARP cache entries residing in the other nodes to be an invalid.
	 * The gratuitous ARP packet should update the link-layer address accordingly for the invalid ARP cache.
	 */

	// Construct the ARP packet following RFC 5944 section 4.6.
	arpPacket := new(bytes.Buffer)
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint16(1)); writeErr != nil { // Hardware Type: 1 is Ethernet
		return formatPacketFieldWriteError("Hardware Type", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint16(syscall.ETH_P_IP)); writeErr != nil { // Protocol Type: 0x0800 is IPv4
		return formatPacketFieldWriteError("Protocol Type", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint8(6)); writeErr != nil { // Hardware address Length: 6 bytes for MAC address
		return formatPacketFieldWriteError("Hardware address Length", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint8(4)); writeErr != nil { // Protocol address length: 4 bytes for IPv4 address
		return formatPacketFieldWriteError("Protocol address length", arpPacketName, writeErr)
	}
	if writeErr := binary.Write(arpPacket, binary.BigEndian, uint16(1)); writeErr != nil { // Operation: 1 is request, 2 is response
		return formatPacketFieldWriteError("Operation", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write(l.Attrs().HardwareAddr); writeErr != nil { // Sender hardware address
		return formatPacketFieldWriteError("Sender hardware address", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write(srcIP.To4()); writeErr != nil { // Sender protocol address
		return formatPacketFieldWriteError("Sender protocol address", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); writeErr != nil { // Target hardware address is the Broadcast MAC.
		return formatPacketFieldWriteError("Target hardware address", arpPacketName, writeErr)
	}
	if _, writeErr := arpPacket.Write(dstIP.To4()); writeErr != nil { // Target protocol address
		return formatPacketFieldWriteError("Target protocol address", arpPacketName, writeErr)
	}

	sockAddr := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),                                // Ethertype of ARP (0x0806)
		Ifindex:  l.Attrs().Index,                                         // Interface Index
		Hatype:   1,                                                       // Hardware Type: 1 is Ethernet
		Pkttype:  0,                                                       // Packet Type.
		Halen:    6,                                                       // Hardware address Length: 6 bytes for MAC address
		Addr:     [8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // Address is the broadcast MAC address.
	}

	// Create a socket such that the Ethernet header would constructed by the OS. The arpPacket only contains the ARP payload.
	soc, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return fmt.Errorf("failed to create AF_PACKET datagram socket: %v", err)
	}
	defer syscall.Close(soc)

	if err := syscall.Sendto(soc, arpPacket.Bytes(), 0, &sockAddr); err != nil {
		return fmt.Errorf("failed to send ARP request for IPv4 %s on Interface %s: %v", srcIP.String(), l.Attrs().Name, err)
	}

	return nil
}

func SendUnsolicitedNeighborAdvertisement(dstIP net.IP, ifi *net.Interface, ndpClient *ndp.Conn) error {
	nDstIP := netip.MustParseAddr(dstIP.String())
	m := &ndp.NeighborSolicitation{
		TargetAddress: nDstIP,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      ifi.HardwareAddr,
			},
		},
	}

	// Always multicast the message to the target's solicited-node multicast
	// group as if we have no knowledge of its MAC address.
	snm, err := ndp.SolicitedNodeMulticast(nDstIP)
	if err != nil {
		return err
	}

	// we send a gratuitous neighbor solicitation to checking if ip is conflict
	err = ndpClient.WriteTo(m, nil, snm)
	if err != nil {
		return fmt.Errorf("failed to send ndp message: %v", err)
	}

	return nil
}

// SendUnsolicitedNeighborAdvertisement sends an unsolicited neighbor advertisement packet with the provided source IP over the provided interface.
func SendUnsolicitedNeighborAdvertisement1(dstIP net.IP, l netlink.Link) error {
	/* As per RFC 4861, a link-layer address change can multicast a few unsolicited neighbor advertisements to all nodes to quickly
	 * update the cached link-layer addresses that have become invalid. In the case of SRIOV-CNI, an address can be reused for
	 * different pods. Each pod could likely have a different link-layer address in this scenario, which makes the Neighbor Cache
	 * entries residing in the neighbors to be an invalid. The unsolicited neighbor advertisement should update the link-layer address
	 * accordingly for the IPv6 entry.
	 * However if any of these conditions are true:
	 *  - The IPv6 address was not reused for the new pod.
	 *  - No prior established communication with the neighbor.
	 * Then the neighbor receiving this unsolicited neighbor advertisement would be silently discard. This behavior is described
	 * in RFC 4861 section 7.2.5. This is acceptable behavior since the purpose of sending an unsolicited neighbor advertisement
	 * is not to create a new entry but rather update already existing invalid entries.
	 */

	// Construct the ICMPv6 Neighbor Advertisement packet following RFC 4861.
	//payload := new(bytes.Buffer)
	// ICMPv6 Flags: As per RFC 4861, the solicited flag must not be set and the override flag should be set (to
	// override existing cache entry) for unsolicited advertisements.
	// if writeErr := binary.Write(payload, binary.BigEndian, uint32(0x20000000)); writeErr != nil {
	// 	return formatPacketFieldWriteError("Flags", icmpV6PacketName, writeErr)
	// }
	// if _, writeErr := payload.Write(dstIP.To16()); writeErr != nil { // ICMPv6 Target IPv6 Address.
	// 	return formatPacketFieldWriteError("Target IPv6 Address", icmpV6PacketName, writeErr)
	// }
	// if writeErr := binary.Write(payload, binary.BigEndian, uint8(2)); writeErr != nil { // ICMPv6 Option Type: 2 is target link-layer address.
	// 	return formatPacketFieldWriteError("Option Type", icmpV6PacketName, writeErr)
	// }
	// if writeErr := binary.Write(payload, binary.BigEndian, uint8(1)); writeErr != nil { // ICMPv6 Option Length. Units of 8 bytes.
	// 	return formatPacketFieldWriteError("Option Length", icmpV6PacketName, writeErr)
	// }
	// if _, writeErr := payload.Write(l.Attrs().HardwareAddr); writeErr != nil { // ICMPv6 Option Link-layer Address.
	// 	return formatPacketFieldWriteError("Option Link-layer Address", icmpV6PacketName, writeErr)
	// }
	// Construct ICMPv6 Neighbor Solicitation message
	payload := new(bytes.Buffer)
	if _, writeErr := payload.Write(dstIP.To16()); writeErr != nil { // Target IPv6 address
		return formatPacketFieldWriteError("Target IPv6 Address", icmpV6PacketName, writeErr) //"failed to write target IPv6 address: %v", writeErr)
	}
	if writeErr := binary.Write(payload, binary.BigEndian, uint8(1)); writeErr != nil { // Source link-layer address option type
		return formatPacketFieldWriteError("Option Type", icmpV6PacketName, writeErr)
	}
	if writeErr := binary.Write(payload, binary.BigEndian, uint8(1)); writeErr != nil { // Option length
		return formatPacketFieldWriteError("Option length", icmpV6PacketName, writeErr)
	}
	if _, writeErr := payload.Write(l.Attrs().HardwareAddr); writeErr != nil { // Source link-layer address
		return formatPacketFieldWriteError("Source link-layer address", icmpV6PacketName, writeErr)
	}

	icmpv6Msg := icmp.Message{
		Type:     ipv6.ICMPTypeNeighborSolicitation, // ICMPv6 type is neighbor advertisement.
		Code:     0,                                 // ICMPv6 Code: As per RFC 4861 section 7.1.2, the code is always 0.
		Checksum: 0,                                 // Checksum is calculated later.
		Body: &icmp.RawBody{
			Data: payload.Bytes(),
		},
	}

	// Get the byte array of the ICMPv6 Message.
	icmpv6Bytes, err := icmpv6Msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("failed to Marshal ICMPv6 Message: %v", err)
	}

	// Create a socket such that the Ethernet header and IPv6 header would constructed by the OS.
	soc, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, syscall.IPPROTO_ICMPV6)
	if err != nil {
		return fmt.Errorf("failed to create AF_INET6 raw socket: %v", err)
	}
	defer syscall.Close(soc)

	// As per RFC 4861 section 7.1.2, the IPv6 hop limit is always 255.
	if err := syscall.SetsockoptInt(soc, syscall.IPPROTO_IPV6, syscall.IPV6_MULTICAST_HOPS, 255); err != nil {
		return fmt.Errorf("failed to set IPv6 multicast hops to 255: %v", err)
	}

	// Set the destination IPv6 address to the IPv6 link-local all nodes multicast address (ff02::1).
	var r [16]byte
	copy(r[:], net.IPv6linklocalallnodes.To16())
	sockAddr := syscall.SockaddrInet6{Addr: r}
	if err := syscall.Sendto(soc, icmpv6Bytes, 0, &sockAddr); err != nil {
		return fmt.Errorf("failed to send Unsolicited Neighbor Advertisement for IPv6 %s on Interface %s: %v", dstIP.String(), l.Attrs().Name, err)
	}

	return nil
}

// Blocking wait for interface ifName to have carrier (!NO_CARRIER flag).
func WaitForCarrier(l netlink.Link, waitTime time.Duration) bool {
	var nextSleepDuration time.Duration

	start := time.Now()

	for nextSleepDuration == 0 || time.Since(start) < waitTime {
		if nextSleepDuration == 0 {
			nextSleepDuration = 2 * time.Millisecond
		} else {
			time.Sleep(nextSleepDuration)
			/* Grow wait time exponentionally (factor 1.5). */
			nextSleepDuration += nextSleepDuration / 2
		}

		/* Wait for carrier, i.e. IFF_UP|IFF_RUNNING. Note that there is also
		 * IFF_LOWER_UP, but we follow iproute2 ([1]).
		 *
		 * [1] https://git.kernel.org/pub/scm/network/iproute2/iproute2.git/tree/ip/ipaddress.c?id=f9601b10c21145f76c3d46c163bac39515ed2061#n86
		 */

		if l.Attrs().RawFlags&(unix.IFF_UP|unix.IFF_RUNNING) == (unix.IFF_UP | unix.IFF_RUNNING) {
			return true
		}
	}

	return false
}

// htons converts an uint16 from host to network byte order.
func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}

// formatPacketFieldWriteError builds an error string for the cases when writing to a field of a packet fails.
func formatPacketFieldWriteError(field string, packetType string, writeErr error) error {
	return fmt.Errorf("failed to write the %s field in the %s packet: %v", field, packetType, writeErr)
}

// NewSock returns a new raw socket to listen for ARP packets on the specified network interface.
func NewARPSockRAW(l netlink.Link) (fd int, err error) {
	// Create a raw socket to listen for ARP packets.
	sock, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return fd, fmt.Errorf("failed to create raw socket: %v", err)
	}
	//defer syscall.Close(sock)

	// Bind the socket to the network interface.
	if err := syscall.Bind(sock, &syscall.SockaddrLinklayer{Ifindex: l.Attrs().Index}); err != nil {
		return fd, fmt.Errorf("failed to bind socket to interface: %v", err)
	}
	return sock, nil
}

func NewNDPSockRaw(iface string) (int, error) {
	// Create a raw socket for ICMPv6
	sock, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, syscall.IPPROTO_ICMPV6)
	if err != nil {
		return sock, fmt.Errorf("failed to create raw socket: %v", err)
	}

	// Bind the socket to the network interface
	if err := syscall.BindToDevice(sock, iface); err != nil {
		return sock, fmt.Errorf("failed to bind socket to interface: %v", err)
	}
	return sock, nil
}

func ParseIPv6NeighborAdvertisementMsg(n int, buf []byte) (srcIP net.IP, mac net.HardwareAddr, err error) {
	if n != 32 && buf[0] != 136 {
		// this isn't a ICMPv6 NA message
		return nil, nil, fmt.Errorf("this not a ICMPv6 neighbor advertisement message")
	}

	// Extract the source IP address (offset 8-24 in the IPv6 header)
	srcIP = net.IP(buf[8:24])

	// Options start after the 24-byte ICMPv6 header
	options := buf[24:n]

	// Check if the target link-layer address option is present
	// Iterate over options to find the target link-layer address option
	for i := 0; i < len(options); {
		optionType := options[i]
		optionLength := options[i+1] * 8 // Length is in units of 8 octets

		if optionType == 2 { // Type 2 is the target link-layer address
			if optionLength < 8 {
				return nil, nil, fmt.Errorf("")
			}
			mac = net.HardwareAddr(options[i+2 : i+8])
		}

		i += int(optionLength)
	}
	return
}
