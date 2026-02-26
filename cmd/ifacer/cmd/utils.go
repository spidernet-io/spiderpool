// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// BondOptions  for the bonding driver are supplied as parameters to the
// bonding module at load time, or are specified via sysfs.
// refer to https://www.kernel.org/doc/Documentation/networking/bonding.txt
type BondOptions struct {
	ActiveSlave     string   `json:"active_slave,omitempty"`
	AdActorSysPrio  int      `json:"ad_actor_sys_prio,omitempty"`
	AdActorSystem   string   `json:"ad_actor_system,omitempty"`
	AdSelect        int      `json:"ad_select,omitempty"`
	AdUserPortKey   int      `json:"ad_user_port_key,omitempty"`
	AllSlavesActive int      `json:"all_slaves_active,omitempty"`
	ArpInterval     int      `json:"arp_interval,omitempty"`
	ArpIPTargets    []string `json:"arp_ip_target,omitempty"`
	ArpValidate     int      `json:"arp_validate,omitempty"`
	ArpAllTargets   int      `json:"arp_all_targets,omitempty"`
	DownDelay       int      `json:"downdelay,omitempty"`
	FailOverMac     int      `json:"fail_over_mac,omitempty"`
	LacpRate        int      `json:"lacp_rate,omitempty"`
	Miimon          int      `json:"miimon,omitempty"`
	MinLinks        int      `json:"min_links,omitempty"`
	PacketsPerSlave int      `json:"packets_per_slave,omitempty"`
	// The primary option is only valid for active-backup(1),
	// balance-tlb (5) and balance-alb (6) mode
	Primary         string `json:"primary,omitempty"`
	PrimaryReselect int    `json:"primary_reselect,omitempty"`
	TlbDynamicLb    int    `json:"tlb_dynamic_lb,omitempty"`
	UpDelay         int    `json:"up_delay,omitempty"`
	UseCarrier      int    `json:"use_carrier,omitempty"`
	XmitHashPolicy  int    `json:"xmit_hash_policy,omitempty"`
	LpInterval      int    `json:"lp_interval,omitempty"`
	ResendIgmp      int    `json:"resend_igmp,omitempty"`
	NumPeerNotif    int    `json:"peer_notif_delay,omitempty"`
}

type BondOptionFunc func(bond *netlink.Bond)

func ActiveSlaveOption(activeSlave int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.ActiveSlave = activeSlave
	}
}

func AdActorSystemOption(adActorSystem net.HardwareAddr) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.AdActorSystem = adActorSystem
	}
}

func AdActorSysPrioOption(adActorSysPrio int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.AdActorSysPrio = adActorSysPrio
	}
}

func AdSelectOption(adSelect int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.AdSelect = netlink.BondAdSelect(adSelect)
	}
}

func AdUserPortKeyOption(adUserPortKey int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.AdUserPortKey = adUserPortKey
	}
}

func AllSlavesActiveOption(allSlavesActive int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.AllSlavesActive = allSlavesActive
	}
}

func ArpIntervalOption(arpInterval int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.ArpInterval = arpInterval
	}
}

func ArpIPTargetsOption(arpIPTargets []net.IP) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.ArpIpTargets = arpIPTargets
	}
}

func ArpValidateOption(arpValidate int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.ArpValidate = netlink.BondArpValidate(arpValidate)
	}
}

func ArpAllTargetsOption(arpAllTargets int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.ArpAllTargets = netlink.BondArpAllTargets(arpAllTargets)
	}
}

func DownDelayOption(downDelay int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.DownDelay = downDelay
	}
}

func FailOverMacOption(failOverMac int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.FailOverMac = netlink.BondFailOverMac(failOverMac)
	}
}

func LacpRateOption(lacpRate int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.LacpRate = netlink.BondLacpRate(lacpRate)
	}
}

func MiimonOption(miimon int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.Miimon = miimon
	}
}

func MinLinksOption(minLinks int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.MinLinks = minLinks
	}
}

func PacketsPerSlaveOption(packetsPerSlave int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.PacketsPerSlave = packetsPerSlave
	}
}

func PrimaryOption(primary int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.Primary = primary
	}
}

func PrimaryReselectOption(primaryReselect int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.PrimaryReselect = netlink.BondPrimaryReselect(primaryReselect)
	}
}

func TlbDynamicLbOption(tlbDynamicLb int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.TlbDynamicLb = tlbDynamicLb
	}
}

func UpDelayOption(upDelay int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.UpDelay = upDelay
	}
}

func UseCarrierOption(useCarrier int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.UseCarrier = useCarrier
	}
}

func XmitHashPolicyOption(xmitHashPolicy int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.XmitHashPolicy = netlink.BondXmitHashPolicy(xmitHashPolicy)
	}
}

func LpIntervalOption(lpInterval int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.LpInterval = lpInterval
	}
}

func ResendIgmpOption(resendIgmp int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.ResendIgmp = resendIgmp
	}
}

func NumPeerNotifOption(numPeerNotif int) BondOptionFunc {
	return func(bond *netlink.Bond) {
		bond.NumPeerNotif = numPeerNotif
	}
}

func GetAllIntBondOptions(bondOptions *BondOptions, bondOptionFuncs []BondOptionFunc) []BondOptionFunc {
	if bondOptions.ArpAllTargets > 0 {
		bondOptionFuncs = append(bondOptionFuncs, ArpAllTargetsOption(bondOptions.ArpAllTargets))
	}
	if bondOptions.MinLinks > 0 {
		bondOptionFuncs = append(bondOptionFuncs, MinLinksOption(bondOptions.MinLinks))
	}
	if bondOptions.AdActorSysPrio > 0 {
		bondOptionFuncs = append(bondOptionFuncs, AdActorSysPrioOption(bondOptions.AdActorSysPrio))
	}
	if bondOptions.ArpInterval > 0 {
		bondOptionFuncs = append(bondOptionFuncs, ArpIntervalOption(bondOptions.ArpInterval))
	}
	if bondOptions.AdSelect > 0 {
		bondOptionFuncs = append(bondOptionFuncs, AdSelectOption(bondOptions.AdSelect))
	}
	if bondOptions.AdUserPortKey > 0 {
		bondOptionFuncs = append(bondOptionFuncs, AdUserPortKeyOption(bondOptions.AdUserPortKey))
	}
	if bondOptions.ArpValidate > 0 {
		bondOptionFuncs = append(bondOptionFuncs, ArpValidateOption(bondOptions.ArpValidate))
	}
	if bondOptions.AllSlavesActive > 0 {
		bondOptionFuncs = append(bondOptionFuncs, AllSlavesActiveOption(bondOptions.AllSlavesActive))
	}
	if bondOptions.DownDelay > 0 {
		bondOptionFuncs = append(bondOptionFuncs, DownDelayOption(bondOptions.DownDelay))
	}
	if bondOptions.FailOverMac > 0 {
		bondOptionFuncs = append(bondOptionFuncs, FailOverMacOption(bondOptions.FailOverMac))
	}
	if bondOptions.LacpRate > 0 {
		bondOptionFuncs = append(bondOptionFuncs, LacpRateOption(bondOptions.LacpRate))
	}
	if bondOptions.Miimon > 0 {
		bondOptionFuncs = append(bondOptionFuncs, MiimonOption(bondOptions.Miimon))
	}
	if bondOptions.PacketsPerSlave > 0 {
		bondOptionFuncs = append(bondOptionFuncs, PacketsPerSlaveOption(bondOptions.PacketsPerSlave))
	}
	if bondOptions.PrimaryReselect > 0 {
		bondOptionFuncs = append(bondOptionFuncs, PrimaryReselectOption(bondOptions.PrimaryReselect))
	}
	if bondOptions.TlbDynamicLb > 0 {
		bondOptionFuncs = append(bondOptionFuncs, TlbDynamicLbOption(bondOptions.TlbDynamicLb))
	}
	if bondOptions.UpDelay > 0 {
		bondOptionFuncs = append(bondOptionFuncs, UpDelayOption(bondOptions.UpDelay))
	}
	if bondOptions.UseCarrier > 0 {
		bondOptionFuncs = append(bondOptionFuncs, UseCarrierOption(bondOptions.UseCarrier))
	}
	if bondOptions.XmitHashPolicy > 0 {
		bondOptionFuncs = append(bondOptionFuncs, XmitHashPolicyOption(bondOptions.XmitHashPolicy))
	}
	if bondOptions.LpInterval > 0 {
		bondOptionFuncs = append(bondOptionFuncs, LpIntervalOption(bondOptions.LpInterval))
	}
	if bondOptions.ResendIgmp > 0 {
		bondOptionFuncs = append(bondOptionFuncs, ResendIgmpOption(bondOptions.ResendIgmp))
	}
	if bondOptions.NumPeerNotif > 0 {
		bondOptionFuncs = append(bondOptionFuncs, NumPeerNotifOption(bondOptions.NumPeerNotif))
	}
	return bondOptionFuncs
}

func getVlanIfaceName(master string, vlanID int) string {
	return fmt.Sprintf("%s.%d", master, vlanID)
}

func checkInterfaceWithSameVlan(vlanID int, vlanInterface string) error {
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to LinkList: %w", err)
	}

	for _, link := range links {
		if link.Type() == "vlan" {
			if vlan, ok := link.(*netlink.Vlan); ok && vlan.VlanId == vlanID && vlan.Name != vlanInterface {
				return fmt.Errorf("cannot have multiple different vlan interfaces with the same vlanId %v on node at the same time", vlanID)
			}
		}
	}
	return nil
}
