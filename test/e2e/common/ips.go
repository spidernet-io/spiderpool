// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package common provides test utilities for E2E tests.
package common

import (
	"fmt"
	"net"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

var generatedIPs map[string]bool

var generateIPsLock = new(lock.Mutex)

func GenerateIPs(cidr string, num int) ([]string, error) {
	generateIPsLock.Lock()
	defer generateIPsLock.Unlock()

	if generatedIPs == nil {
		generatedIPs = make(map[string]bool)
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	// Increment the network IP to start from 192.168.0.1 instead of 192.168.0.0
	inc(ipNet.IP)

	cidrIPNumber := ipCount(ipNet)

	if cidrIPNumber < num {
		return nil, fmt.Errorf("cidr %s only has %d IP addresses and it doesn't match required IP number %d", cidr, cidrIPNumber, num)
	}

	var ips []string
	for ip := ipNet.IP; len(ips) < num; inc(ip) {
		_, ok := generatedIPs[ip.String()]
		if !ok {
			ips = append(ips, ip.String())
			generatedIPs[ip.String()] = true
		}
	}

	resultLen := len(ips)
	if resultLen < num {
		for _, tmpIP := range ips {
			delete(generatedIPs, tmpIP)
		}
		return nil, fmt.Errorf("cidr %s is already allocated by other calls and just has %d IP addresses which unmactes your required IP number %d", cidr, resultLen, num)
	}

	return ips, nil
}

func ipCount(network *net.IPNet) int {
	prefixLen, bits := network.Mask.Size()
	return 1 << (bits - prefixLen)
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
