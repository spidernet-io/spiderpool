// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type DeviceTrafficClass struct {
	NetDevName   string
	IfName       string
	TrafficClass int
}

const prefix = "Global tclass="

var (
	statFunc     = os.Stat
	readFileFunc = os.ReadFile
)

func GetDeviceTrafficClass(impl NetlinkImpl) ([]DeviceTrafficClass, error) {
	m, err := getIfnameNetDevMap(impl)
	if err != nil {
		return nil, err
	}

	res := make([]DeviceTrafficClass, 0)
	for ifname, dev := range m {
		if dev.IsRoot {
			path := fmt.Sprintf("/sys/class/infiniband/%s/tc/1/traffic_class", ifname)

			tos := 0

			_, err := statFunc(path)

			if !os.IsNotExist(err) {
				data, err := readFileFunc(path)
				if err != nil {
					return nil, fmt.Errorf("failed to read traffic_class file %s: %w", path, err)
				}

				line := strings.TrimSpace(string(data))

				if strings.HasPrefix(line, prefix) {
					tosStr := strings.TrimPrefix(line, prefix)
					tos, err = strconv.Atoi(tosStr)
					if err != nil {
						return nil, fmt.Errorf("invalid tclass value for %s: %w", ifname, err)
					}
				}
			}

			res = append(res, DeviceTrafficClass{
				NetDevName:   dev.NetDevName,
				IfName:       ifname,
				TrafficClass: tos,
			})
		}
	}

	return res, nil
}
