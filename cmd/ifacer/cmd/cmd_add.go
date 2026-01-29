// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/vishvananda/netlink"
)

func CmdAdd(args *skel.CmdArgs) error {
	conf, err := ParseConfig(args.StdinData)
	if err != nil {
		return err
	}

	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
	}

	switch len(conf.Interfaces) {
	case 0:
		return types.PrintResult(result, conf.CNIVersion)
	case 1:
		if conf.VlanID == 0 {
			return types.PrintResult(result, conf.CNIVersion)
		}

		if err := checkInterfaceWithSameVlan(conf.VlanID, getVlanIfaceName(conf.Interfaces[0], conf.VlanID)); err != nil {
			return err
		}

		if err = createVlanDevice(conf); err != nil {
			return fmt.Errorf("failed to createVlanDevice: %w", err)
		}

		return types.PrintResult(result, conf.CNIVersion)
	default:
		if conf.Bond == nil {
			return types.PrintResult(result, conf.CNIVersion)
		}

		bond, err := createBondDevice(conf)
		if err != nil {
			return fmt.Errorf("failed to createBondDevice: %w", err)
		}

		if conf.VlanID == 0 {
			return types.PrintResult(result, conf.CNIVersion)
		}

		vlanName := getVlanIfaceName(conf.Bond.Name, conf.VlanID)
		if err := checkInterfaceWithSameVlan(conf.VlanID, vlanName); err != nil {
			return err
		}

		vlanLink, err := netlink.LinkByName(vlanName)
		if err == nil {
			if vlanLink.Attrs().Flags != net.FlagUp {
				if err = netlink.LinkSetUp(vlanLink); err != nil {
					return fmt.Errorf("failed to set %s up: %w", vlanLink.Attrs().Name, err)
				}
			}
			return types.PrintResult(result, conf.CNIVersion)
		}

		var notFoundErr netlink.LinkNotFoundError
		if !errors.As(err, &notFoundErr) {
			return fmt.Errorf("failed to LinkByName %s: %w", vlanName, err)
		}

		// create vlan interface
		if err = networking.LinkAdd(&netlink.Vlan{
			LinkAttrs: netlink.LinkAttrs{
				Name:        vlanName,
				ParentIndex: bond.Index,
			},
			VlanId: conf.VlanID,
		}); err != nil {
			return fmt.Errorf("failed to create vlan interface %s: %w", vlanName, err)
		}

		return types.PrintResult(result, conf.CNIVersion)
	}
}

func createBondDevice(conf *Ifacer) (*netlink.Bond, error) {
	var err error
	var bondLink netlink.Link
	bondLink, err = netlink.LinkByName(conf.Bond.Name)
	if err == nil && bondLink.Attrs().Flags != net.FlagUp {
		if err = netlink.LinkSetUp(bondLink); err != nil {
			return nil, fmt.Errorf("failed to set %s up: %w", bondLink.Attrs().Name, err)
		}

		if bondLink.Type() != "bond" {
			return nil, fmt.Errorf("createBondDevice failure: a non-bond type interface named %s already exists on the host", conf.Bond.Name)
		}

		if bond, ok := bondLink.(*netlink.Bond); ok {
			return bond, nil
		}
		return nil, fmt.Errorf("createBondDevice failure: invalid bond device")
	}

	var notFoundErr netlink.LinkNotFoundError
	if !errors.As(err, &notFoundErr) {
		return nil, fmt.Errorf("failed to LinkByName %s: %w", conf.Bond.Name, err)
	}

	if err = validateBondMode(conf.Bond.Mode); err != nil {
		return nil, err
	}

	bond := netlink.NewLinkBond(netlink.NewLinkAttrs())
	bond.Name = conf.Bond.Name
	bond.Mode = netlink.BondMode(conf.Bond.Mode)

	if conf.Bond.Options != "" {
		bondOptions, err := parseString2BondOptions(conf.Bond.Options)
		if err != nil {
			return nil, fmt.Errorf("parseString2BondOptions: %w", err)
		}

		if err = parseBondOptions2NetlinkBond(bondOptions, bond); err != nil {
			return nil, fmt.Errorf("parseBondOptions2NetlinkBond: %w", err)
		}
	}

	if err = netlink.LinkAdd(bond); err != nil {
		return nil, err
	}

	for _, slave := range conf.Interfaces {
		link, err := netlink.LinkByName(slave)
		if err != nil {
			return nil, fmt.Errorf("failed to InterfaceByName %s: %w", slave, err)
		}

		if err = netlink.LinkSetDown(link); err != nil {
			return nil, fmt.Errorf("failed to set slave %s down: %w", slave, err)
		}

		if err = networking.LinkSetBondSlave(slave, bond); err != nil {
			return nil, err
		}

		if err = netlink.LinkSetUp(link); err != nil {
			return nil, err
		}
	}

	if err = netlink.LinkSetUp(bond); err != nil {
		return nil, fmt.Errorf("failed to set %s up", bond.Name)
	}

	// create vlan interface base on bond
	return bond, nil
}

func createVlanDevice(conf *Ifacer) error {
	var err error
	// If the parent interface is down, we set it to up.
	var parentLink netlink.Link
	parentLink, err = netlink.LinkByName(conf.Interfaces[0])
	if err != nil {
		return fmt.Errorf("failed to LinkByName %s: %w", conf.Interfaces[0], err)
	}

	if parentLink.Attrs().Flags != net.FlagUp {
		if err = netlink.LinkSetUp(parentLink); err != nil {
			return fmt.Errorf("failed to set %s up: %w", parentLink.Attrs().Name, err)
		}
	}

	var vlanLink netlink.Link
	vlanIfName := getVlanIfaceName(conf.Interfaces[0], conf.VlanID)
	vlanLink, err = netlink.LinkByName(vlanIfName)
	if err == nil {
		if vlanLink.Attrs().Flags != net.FlagUp {
			if err = netlink.LinkSetUp(vlanLink); err != nil {
				return fmt.Errorf("failed to set %s up: %w", vlanLink.Attrs().Name, err)
			}
		}
		return nil
	}

	var notFoundErr netlink.LinkNotFoundError
	if !errors.As(err, &notFoundErr) {
		return fmt.Errorf("failed to LinkByName %s: %w", vlanIfName, err)
	}

	// we only create if vlanIf not present
	return networking.LinkAdd(&netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        vlanIfName,
			ParentIndex: parentLink.Attrs().Index,
		},
		VlanId: conf.VlanID,
	})
}
