// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"os"
	"regexp"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

// OverwriteHwAddress override the hardware address of the specified interface.
func OverwriteHwAddress(logger *zap.Logger, netns ns.NetNS, macPrefix, iface string) (string, error) {
	ips, err := IPAddressByName(netns, iface, netlink.FAMILY_ALL)
	if err != nil {
		logger.Error("failed to get IPAddressByName", zap.String("interface", iface), zap.Error(err))
		return "", err
	}

	// we only focus on first element
	nAddr, err := netip.ParseAddr(ips[0].IP.String())
	if err != nil {
		logger.Error("failed to ParsePrefix", zap.Error(err))
		return "", err
	}

	suffix, err := inetAton(nAddr)
	if err != nil {
		logger.Error("failed to inetAton", zap.Error(err))
		return "", err
	}

	// newmac = xx:xx + xx:xx:xx:xx
	hwAddr := macPrefix + ":" + suffix
	err = netns.Do(func(netNS ns.NetNS) error {
		link, err := netlink.LinkByName(iface)
		if err != nil {
			logger.Error(err.Error())
			return err
		}
		return netlink.LinkSetHardwareAddr(link, parseMac(hwAddr))
	})

	if err != nil {
		logger.Error("failed to OverrideHwAddress", zap.String("hardware address", hwAddr), zap.Error(err))
		return "", err
	}
	return hwAddr, nil
}

// parseMac parse hardware addr from given string
func parseMac(s string) net.HardwareAddr {
	hardwareAddr, err := net.ParseMAC(s)
	if err != nil {
		panic(err)
	}
	return hardwareAddr
}

// inetAton converts an IP Address (IPv4 or IPv6) netip.addr object to a hexadecimal representation.
// for ipv4: convert a full IP address(length: 4 B) to hexadecimal representation.
// for ipv6: convert
func inetAton(ip netip.Addr) (string, error) {
	if ip.AsSlice() == nil {
		return "", fmt.Errorf("invalid ip address")
	}

	ipInt := big.NewInt(0)
	// 32 bit -> 4 B
	hexCode := make([]byte, hex.EncodedLen(ip.BitLen()))
	ipInt.SetBytes(ip.AsSlice()[:])
	hex.Encode(hexCode, ipInt.Bytes())

	if ip.Is6() {
		// for ipv6: 128 bit = 32 hex
		// take the last 8 hex as the hardware address
		return convertHex2Mac(hexCode[24:]), nil
	}

	return convertHex2Mac(hexCode), nil
}

// convertHex2Mac convert hexcode to 4B hardware address
// convert ip(hex) to "xx:xx:xx:xx"
func convertHex2Mac(hexCode []byte) string {
	regexSpilt, err := regexp.Compile(".{2}")
	if err != nil {
		panic(err)
	}
	return string(bytes.Join(regexSpilt.FindAll(hexCode, 4), []byte(":")))
}

// GetHwAddressByName get hardware address of veth pair device
func GetHwAddressByName(netns ns.NetNS, podVethPairName, hostVethPairName string) (net.HardwareAddr, net.HardwareAddr, error) {
	var containerVethHwAddress net.HardwareAddr
	err := netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(podVethPairName)
		if err != nil {
			return err
		}
		containerVethHwAddress = link.Attrs().HardwareAddr
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	hostLink, err := netlink.LinkByName(hostVethPairName)
	if err != nil {
		return nil, nil, err
	}
	return hostLink.Attrs().HardwareAddr, containerVethHwAddress, nil
}

// GetHostVethName get veth name in host side
func GetHostVethName(netns ns.NetNS, podVethPairName string) (string, error) {
	parentIndex := -1
	err := netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(podVethPairName)
		if err != nil {
			return err
		}
		// get link index of host veth-peer and pod veth-peer mac-address
		parentIndex = link.Attrs().ParentIndex
		if parentIndex < 0 {
			return fmt.Errorf("failed to get parentIndex")
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	hostLink, err := netlink.LinkByIndex(parentIndex)
	if err != nil {
		return "", err
	}

	return hostLink.Attrs().Name, nil
}

// AddStaticNeighborTable add a static neighborhood table
func AddStaticNeighborTable(linkIndex int, dstIP net.IP, hwAddress net.HardwareAddr) error {
	neigh := &netlink.Neigh{
		LinkIndex:    linkIndex,
		State:        netlink.NUD_PERMANENT,
		Type:         netlink.NDA_LLADDR,
		IP:           dstIP,
		HardwareAddr: hwAddress,
	}

	if err := netlink.NeighAdd(neigh); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to add neigh table: %v ", err)
	}

	return nil
}
