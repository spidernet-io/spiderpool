// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"net"
	"os"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
)

// Label(K00002)

func TestGetDeviceTrafficClass(t *testing.T) {
	tests := []struct {
		name         string
		netlink      NetlinkImpl
		want         []DeviceTrafficClass
		wantErr      bool
		statsFunc    func(name string) (os.FileInfo, error)
		readFileFunc func(name string) ([]byte, error)
	}{
		{
			name: "test",
			netlink: NetlinkImpl{
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					rdmaList := []*netlink.RdmaLink{
						{
							Attrs: netlink.RdmaLinkAttrs{
								Name:     "mlx5_34",
								NodeGuid: "1d:c9:d1:fe:ff:ac:36:ae",
							},
						},
						{
							Attrs: netlink.RdmaLinkAttrs{
								Name:         "mlx5_6",
								NodeGuid:     "b6:65:05:0c:9c:5c:f6:08",
								SysImageGuid: "b6:65:05:0c:9c:5c:f6:00",
							},
						},
						{
							Attrs: netlink.RdmaLinkAttrs{
								Name:         "mlx5_1",
								NodeGuid:     "ea:6d:2d:00:03:c0:63:9c",
								SysImageGuid: "ea:6d:2d:00:03:c0:63:9c",
							},
						},
					}
					return rdmaList, nil
				},
				LinkList: func() ([]netlink.Link, error) {
					linkList := []netlink.Link{
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: ""}},
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "mock-empty-addr", HardwareAddr: nil}},
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{
							Name: "enp5s0f0v6",
							HardwareAddr: func() net.HardwareAddr {
								mac, _ := net.ParseMAC("ae:36:ac:d1:c9:1d")
								return mac
							}(),
						}},
						&netlink.IPoIB{LinkAttrs: netlink.LinkAttrs{
							Name: "ibp13s0v7",
							HardwareAddr: func() net.HardwareAddr {
								mac, _ := net.ParseMAC("00:00:00:68:fe:80:00:00:00:00:00:00:08:f6:5c:9c:0c:05:65:b6")
								return mac
							}(),
						}},
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{
							Name: "ens841np0",
							HardwareAddr: func() net.HardwareAddr {
								mac, _ := net.ParseMAC("9c:63:c0:2d:6d:ea")
								return mac
							}(),
						}},
					}
					return linkList, nil
				},
			},
			want: []DeviceTrafficClass{
				{
					NetDevName:   "ens841np0",
					IfName:       "mlx5_1",
					TrafficClass: 160,
				},
			},
			wantErr: false,
			readFileFunc: func(name string) ([]byte, error) {
				if name == "/sys/class/infiniband/mlx5_1/tc/1/traffic_class" {
					return []byte("Global tclass=160"), nil
				}
				return nil, os.ErrNotExist
			},
			statsFunc: func(name string) (os.FileInfo, error) {
				if name == "/sys/class/infiniband/mlx5_1/tc/1/traffic_class" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statFunc = tt.statsFunc
			readFileFunc = tt.readFileFunc
			got, err := GetDeviceTrafficClass(tt.netlink)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeviceTrafficClass() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDeviceTrafficClass() got = %v, want %v", got, tt.want)
			}
		})
	}
}
