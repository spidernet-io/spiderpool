// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

var DefaultBondName = "sp_bond0"

type Ifacer struct {
	types.NetConf
	Interfaces []string `json:"interfaces,omitempty"`
	VlanID     int      `json:"vlanID,omitempty"`
	Bond       *Bond    `json:"bond,omitempty"`
}

type Bond struct {
	Name    string `json:"name,omitempty"`
	Mode    int    `json:"mode,omitempty"`
	Options string `json:"options,omitempty"`
}

func ParseConfig(stdin []byte) (*Ifacer, error) {
	var err error
	conf := Ifacer{}
	if err = json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if len(conf.Interfaces) == 0 {
		return nil, fmt.Errorf("invalid config: interfaces have at least one interface")
	}

	if conf.VlanID < 0 || conf.VlanID > 4094 {
		return nil, fmt.Errorf("invalid vlan tag %v: vlan tag must be in range [0,4094]", conf.VlanID)
	}

	if conf.Bond != nil && conf.Bond.Name == "" {
		conf.Bond.Name = DefaultBondName
	}

	return &conf, nil
}

func validateBondMode(mode int) error {
	matched := false
	for _, m := range []int{0, 1, 2, 3, 4, 5, 6} {
		if m == mode {
			matched = true
			break
		}
	}

	if !matched {
		return fmt.Errorf("unknown bond mode: %v, available mode: [0,6]", mode)
	}
	return nil
}

// parseBondOptions convert options string to BondOptions object.
// incorrect options are ignored without an error return
// options input-formatted: "k1=v1;k2=v2;k3=v3"
func parseString2BondOptions(options string) (*BondOptions, error) {
	options = strings.TrimSpace(options)
	optionsList := strings.Split(options, ";")

	var rawOptionsJSONStr string
	for idx, option := range optionsList {
		kv := strings.Split(option, "=")
		if len(kv) != 2 {
			continue
		}

		var op string
		if strings.HasPrefix(kv[1], "[") {
			op = fmt.Sprintf("\"%v\":%v", kv[0], kv[1])
		} else {
			_, err := strconv.Atoi(kv[1])
			if err != nil {
				op = fmt.Sprintf("\"%v\":\"%v\"", kv[0], kv[1])
			} else {
				op = fmt.Sprintf("\"%v\":%v", kv[0], kv[1])
			}
		}

		if idx == 0 {
			rawOptionsJSONStr = op
		} else {
			rawOptionsJSONStr = fmt.Sprintf("%s,%s", rawOptionsJSONStr, op)
		}
	}

	optionsJSONStr := fmt.Sprintf("{%s}", rawOptionsJSONStr)

	bondOptions := &BondOptions{}
	if err := json.Unmarshal([]byte(optionsJSONStr), bondOptions); err != nil {
		return nil, fmt.Errorf("failed to convert options to BondOptions: %w", err)
	}

	return bondOptions, nil
}

// parseBondOptions2NetlinkBond convert BondOptions to netlink.Bond object
func parseBondOptions2NetlinkBond(bondOptions *BondOptions, bond *netlink.Bond) error {
	bondOptionsFuncs := make([]BondOptionFunc, 0)
	if bondOptions != nil {
		if bondOptions.Primary != "" {
			link, err := netlink.LinkByName(bondOptions.Primary)
			if err != nil {
				return fmt.Errorf("failed to LinkByName bond primary %s: %w", bondOptions.Primary, err)
			}
			bondOptionsFuncs = append(bondOptionsFuncs, PrimaryOption(link.Attrs().Index))
		}

		if bondOptions.ActiveSlave != "" {
			link, err := netlink.LinkByName(bondOptions.ActiveSlave)
			if err != nil {
				return fmt.Errorf("failed to LinkByName bond active_slave %s: %w", bondOptions.ActiveSlave, err)
			}
			bondOptionsFuncs = append(bondOptionsFuncs, ActiveSlaveOption(link.Attrs().Index))
		}

		if bondOptions.AdActorSystem != "" {
			hwAddr, err := net.ParseMAC(bondOptions.AdActorSystem)
			if err != nil {
				return fmt.Errorf("invalid bond ad_actor_system: %s", bondOptions.AdActorSystem)
			}
			bondOptionsFuncs = append(bondOptionsFuncs, AdActorSystemOption(hwAddr))
		}

		if len(bondOptions.ArpIPTargets) > 0 {
			var arpIPTargets []net.IP
			for _, arpIPTargetStr := range bondOptions.ArpIPTargets {
				arpIPTarget := net.ParseIP(arpIPTargetStr)
				if arpIPTarget == nil {
					return fmt.Errorf("invalid bond arp_ip_target: %s", arpIPTargetStr)
				}
				arpIPTargets = append(arpIPTargets, arpIPTarget)
			}
			bondOptionsFuncs = append(bondOptionsFuncs, ArpIPTargetsOption(arpIPTargets))
		}
		bondOptionsFuncs = GetAllIntBondOptions(bondOptions, bondOptionsFuncs)
	}

	for _, option := range bondOptionsFuncs {
		option(bond)
	}

	return nil
}
