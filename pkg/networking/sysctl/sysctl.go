// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package sysctl

import (
	"fmt"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"os"
)

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
