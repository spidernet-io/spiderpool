// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package sysctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
)

// DefaultSysctlConfig is the default sysctl config for the node
var DefaultSysctlConfig = []struct {
	Name           string
	Value          string
	IsIPv4, IsIPv6 bool
}{
	// In order to avoid large-scale cluster arp_table overflow, resulting in
	// pods not being able to communicate or pods not being able to start due
	// to the inability to insert static arp table entries, it is necessary
	// to appropriately increase and adjust its value. more details see:
	// https://github.com/spidernet-io/spiderpool/issues/3587
	{
		Name: "net.ipv4.neigh.default.gc_thresh3",
		// Assuming a node is full of underlay pods (110) and their subnet
		// mask is 16 bits ( 2 ^ 8 = 256 IPs), the value is 110 * 256 = 28160
		Value:  "28160",
		IsIPv4: true,
	},
	{
		// this sysctl may not be available at low kernel levels,
		// so we'll ignore it at this point.
		Name:   "net.ipv6.neigh.default.gc_thresh3",
		Value:  "28160",
		IsIPv6: true,
	},
	// send gratitous ARP when device or address change
	{
		Name:   "net.ipv4.conf.all.arp_notify",
		Value:  "1",
		IsIPv4: true,
	}, {
		Name:   "net.ipv4.conf.all.forwarding",
		Value:  "1",
		IsIPv4: true,
	}, {
		Name:   "net.ipv6.conf.all.forwarding",
		Value:  "1",
		IsIPv6: true,
	},
}

// SysctlRPFilter set rp_filter value for host netns and specify netns
func SysctlRPFilter(netns ns.NetNS, value int32) error {
	var err error
	if err = setRPFilter(value); err != nil {
		return fmt.Errorf("failed to set host rp_filter : %v", err)
	}
	// set pod rp_filter
	err = netns.Do(func(_ ns.NetNS) error {
		if err := setRPFilter(value); err != nil {
			return fmt.Errorf("failed to set rp_filter in pod : %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// setRPFilter set rp_filter
func setRPFilter(v int32) error {
	dirs, err := os.ReadDir("/proc/sys/net/ipv4/conf")
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		name := fmt.Sprintf("/net/ipv4/conf/%s/rp_filter", dir.Name())
		value, err := sysctl.Sysctl(name)
		if err != nil {
			continue
		}
		if value == fmt.Sprintf("%d", v) {
			continue
		}
		if _, e := sysctl.Sysctl(name, fmt.Sprintf("%d", v)); e != nil {
			return e
		}
	}
	return nil
}

// EnableIpv6Sysctl enable ipv6 for specify netns
func EnableIpv6Sysctl(netns ns.NetNS) error {
	err := netns.Do(func(_ ns.NetNS) error {
		dirs, err := os.ReadDir("/proc/sys/net/ipv6/conf")
		if err != nil {
			return err
		}

		for _, dir := range dirs {
			// Read current sysctl value
			name := fmt.Sprintf("/net/ipv6/conf/%s/disable_ipv6", dir.Name())
			value, err := sysctl.Sysctl(name)
			if err != nil {
				return fmt.Errorf("failed to read current sysctl %+v value: %v", name, err)
			}
			// make sure value=0
			if value != "0" {
				if _, err = sysctl.Sysctl(name, "0"); err != nil {
					return fmt.Errorf("failed to read current sysctl %+v value: %v ", name, err)
				}
			}
		}
		return nil
	})
	return err
}

func SetSysctl(sysConfig string, value string) error {
	// sysConfig: net.ipv6.neigh.default.gc_thresh3
	// to: net/ipv6/neigh/default/gc_thresh3
	sysConfig = strings.ReplaceAll(sysConfig, ".", "/")

	_, err := os.Stat(filepath.Join("/proc/sys", sysConfig))
	if err != nil {
		return err
	}

	if _, err := sysctl.Sysctl(sysConfig, value); err != nil {
		return err
	}

	return nil
}
