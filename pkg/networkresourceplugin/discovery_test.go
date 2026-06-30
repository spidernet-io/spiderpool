// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("master NIC selection", Label("networkresourceplugin_discovery_test"), func() {
	It("treats missing sysfs devices as non-physical", func() {
		Expect(hasPhysicalDevice("spiderpool-missing-interface")).To(BeFalse())
	})

	It("filters loopback, empty, and common virtual interface names", func() {
		Expect(isMasterInterfaceCandidate("", 0)).To(BeFalse())
		Expect(isMasterInterfaceCandidate("lo", net.FlagLoopback)).To(BeFalse())

		for _, name := range []string{"cni0", "flannel.1", "veth123", "docker0", "br-test", "virbr0", "tun0", "tap0"} {
			Expect(isMasterInterfaceCandidate(name, 0)).To(BeFalse(), name)
		}

		Expect(isMasterInterfaceCandidate("eth0", 0)).To(BeTrue())
		Expect(isMasterInterfaceCandidate("ens1", net.FlagUp)).To(BeTrue())
	})

	It("does not select master NICs when advertisement is disabled", func() {
		nics, err := selectMasterNICs(nil, []string{"eth0"}, MasterNICAdvertisementConfig{})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(BeNil())
	})

	It("selects all physical NICs when an empty rule is configured", func() {
		nics, err := selectMasterNICs(nil, []string{"eth0", "ens1"}, MasterNICAdvertisementConfig{Rules: []MasterNICRuleConfig{{}}})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(Equal([]MasterNICSelection{
			{Interface: "ens1", Devices: DefaultMasterNICMaxCount},
			{Interface: "eth0", Devices: DefaultMasterNICMaxCount},
		}))
	})

	It("returns a copy when an empty rule is configured", func() {
		interfaces := []string{"eth0", "ens1"}
		nics, err := selectMasterNICs(nil, interfaces, MasterNICAdvertisementConfig{Rules: []MasterNICRuleConfig{{}}})

		Expect(err).NotTo(HaveOccurred())
		interfaces[0] = "changed"
		Expect(nics).To(Equal([]MasterNICSelection{
			{Interface: "ens1", Devices: DefaultMasterNICMaxCount},
			{Interface: "eth0", Devices: DefaultMasterNICMaxCount},
		}))
	})

	It("applies include and exclude patterns deterministically", func() {
		nics, err := selectMasterNICs(nil, []string{"ens1", "ens2", "eth0"}, MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{
				IncludeInterfaces: []string{"ens*"},
				ExcludeInterfaces: []string{"ens2"},
			}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(Equal([]MasterNICSelection{{Interface: "ens1", Devices: DefaultMasterNICMaxCount}}))
	})

	It("uses node selectors to choose matching rules", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"profile": "eth"}}}
		nics, err := selectMasterNICs(node, []string{"ens1", "eth0"}, MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{
				NodeSelector:      metav1.LabelSelector{MatchLabels: map[string]string{"profile": "eth"}},
				IncludeInterfaces: []string{"eth*"},
			}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(Equal([]MasterNICSelection{{Interface: "eth0", Devices: DefaultMasterNICMaxCount}}))
	})

	It("selects no interfaces when no rule matches the node", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"profile": "default"}}}
		nics, err := selectMasterNICs(node, []string{"ens1", "eth0"}, MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{
				NodeSelector:      metav1.LabelSelector{MatchLabels: map[string]string{"profile": "special"}},
				IncludeInterfaces: []string{"eth*"},
			}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(BeNil())
	})

	It("unions matching rules and sorts selected interfaces", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"profile": "multi"}}}
		nics, err := selectMasterNICs(node, []string{"eth1", "ens2", "eth0"}, MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{
				{
					NodeSelector:      metav1.LabelSelector{MatchLabels: map[string]string{"profile": "multi"}},
					DefaultMaxCount:   5,
					IncludeInterfaces: []string{"eth*"},
				},
				{
					NodeSelector:      metav1.LabelSelector{MatchLabels: map[string]string{"profile": "multi"}},
					DefaultMaxCount:   7,
					IncludeInterfaces: []string{"ens*"},
					ExcludeInterfaces: []string{"ens9"},
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(Equal([]MasterNICSelection{
			{Interface: "ens2", Devices: 7},
			{Interface: "eth0", Devices: 5},
			{Interface: "eth1", Devices: 5},
		}))
	})

	It("uses match expressions to choose matching master NIC rules", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"profile": "eth", "zone": "east"}}}
		nics, err := selectMasterNICs(node, []string{"ens1", "eth0"}, MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{
				NodeSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "profile",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"eth", "default"},
					},
					{
						Key:      "maintenance",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
				}},
				IncludeInterfaces: []string{"eth*"},
			}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(Equal([]MasterNICSelection{{Interface: "eth0", Devices: DefaultMasterNICMaxCount}}))
	})

	It("matches nil node selectors and empty include patterns", func() {
		nics, err := selectMasterNICs(nil, []string{"ens1", "eth0"}, MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{ExcludeInterfaces: []string{"ens*"}}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(nics).To(Equal([]MasterNICSelection{{Interface: "eth0", Devices: DefaultMasterNICMaxCount}}))
	})

	It("reports invalid selectors during affinity checks", func() {
		invalid := metav1.LabelSelector{MatchLabels: map[string]string{"bad/key/again": "value"}}

		_, err := nodeSelectorMatches(nil, &invalid)
		Expect(err).To(HaveOccurred())
	})

	It("matches node affinity selectors", func() {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"resource": "enabled"}}}

		matched, err := nodeSelectorMatches(node, &metav1.LabelSelector{MatchLabels: map[string]string{"resource": "enabled"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())

		matched, err = nodeSelectorMatches(node, &metav1.LabelSelector{MatchLabels: map[string]string{"resource": "disabled"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeFalse())

		matched, err = nodeSelectorMatches(node, &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "role",
			Operator: metav1.LabelSelectorOpDoesNotExist,
		}}})
		Expect(err).NotTo(HaveOccurred())
		Expect(matched).To(BeTrue())
	})

	It("matches glob patterns and builds master NIC resource names", func() {
		Expect(matchAny([]string{"eth*", "ens*"}, "eth0")).To(BeTrue())
		Expect(matchAny([]string{"eth["}, "eth0")).To(BeFalse())
		Expect(matchAny(nil, "eth0")).To(BeFalse())
		Expect(masterNICResourceName("eth0")).To(Equal("spidernet.io/eth0-nic"))
	})

	It("uses virtual interface discovery only when rules explicitly include interfaces", func() {
		Expect(masterNICRulesUseExplicitIncludes(MasterNICAdvertisementConfig{})).To(BeFalse())

		Expect(masterNICRulesUseExplicitIncludes(MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{
				ExcludeInterfaces: []string{"eth1"},
			}},
		})).To(BeFalse())

		Expect(masterNICRulesUseExplicitIncludes(MasterNICAdvertisementConfig{
			Rules: []MasterNICRuleConfig{{
				IncludeInterfaces: []string{"nrpdm*"},
			}},
		})).To(BeTrue())
	})
})
