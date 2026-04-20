// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MultusCNIConfig utils", Label("multuscniconfig_utils_test"), func() {
	Describe("Test ParsePodNetworkAnnotation", func() {
		It("parses pretty-printed JSON array", func() {
			const prettyJSON = `[
  {
    "name": "sriov-net-a",
    "namespace": "kube-system",
    "interface": "net1"
  },
  {
    "name": "bond-net",
    "namespace": "kube-system",
    "interface": "bond0"
  }
]`
			elems, err := ParsePodNetworkAnnotation(prettyJSON, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(elems).To(HaveLen(2))
			Expect(elems[0].Name).To(Equal("sriov-net-a"))
			Expect(elems[0].Namespace).To(Equal("kube-system"))
			Expect(elems[0].InterfaceRequest).To(Equal("net1"))
			Expect(elems[1].Name).To(Equal("bond-net"))
			Expect(elems[1].InterfaceRequest).To(Equal("bond0"))
		})

		It("parses compact JSON array", func() {
			const compact = `[{"name":"net-a","namespace":"ns1","interface":"eth1"}]`
			elems, err := ParsePodNetworkAnnotation(compact, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(elems).To(HaveLen(1))
			Expect(elems[0].Name).To(Equal("net-a"))
			Expect(elems[0].Namespace).To(Equal("ns1"))
		})

		It("parses comma-delimited list", func() {
			const csv = "kube-system/sriov-net-a,kube-system/sriov-net-b,kube-system/bond-net"
			elems, err := ParsePodNetworkAnnotation(csv, "default")
			Expect(err).NotTo(HaveOccurred())
			Expect(elems).To(HaveLen(3))
			Expect(elems[0].Name).To(Equal("sriov-net-a"))
			Expect(elems[2].Name).To(Equal("bond-net"))
		})

		It("fills default namespace for JSON without namespace", func() {
			const jsonNoNS = `[{"name":"macvlan","interface":"net1"}]`
			elems, err := ParsePodNetworkAnnotation(jsonNoNS, "my-ns")
			Expect(err).NotTo(HaveOccurred())
			Expect(elems).To(HaveLen(1))
			Expect(elems[0].Namespace).To(Equal("my-ns"))
		})

		It("returns error for empty input", func() {
			_, err := ParsePodNetworkAnnotation("", "default")
			Expect(err).To(HaveOccurred())
		})

		It("returns error for invalid JSON", func() {
			_, err := ParsePodNetworkAnnotation(`[{"name":}]`, "default")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Test ParsePodNetworkObjectName", func() {
		It("parses plain text with namespace and interface", func() {
			ns, name, iface, err := ParsePodNetworkObjectName("kube-system/macvlan-conf@net1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ns).To(Equal("kube-system"))
			Expect(name).To(Equal("macvlan-conf"))
			Expect(iface).To(Equal("net1"))
		})

		It("parses plain text without namespace", func() {
			ns, name, iface, err := ParsePodNetworkObjectName("macvlan-conf@net1")
			Expect(err).NotTo(HaveOccurred())
			Expect(ns).To(BeEmpty())
			Expect(name).To(Equal("macvlan-conf"))
			Expect(iface).To(Equal("net1"))
		})

		It("parses plain text without interface", func() {
			ns, name, iface, err := ParsePodNetworkObjectName("kube-system/macvlan-conf")
			Expect(err).NotTo(HaveOccurred())
			Expect(ns).To(Equal("kube-system"))
			Expect(name).To(Equal("macvlan-conf"))
			Expect(iface).To(BeEmpty())
		})

		It("returns error for invalid plain text with multiple slashes", func() {
			_, _, _, err := ParsePodNetworkObjectName("a/b/c")
			Expect(err).To(HaveOccurred())
		})

		It("returns error for invalid plain text with multiple @", func() {
			_, _, _, err := ParsePodNetworkObjectName("net@if1@if2")
			Expect(err).To(HaveOccurred())
		})

		It("accepts underscore in interface name", func() {
			_, name, iface, err := ParsePodNetworkObjectName("my-net@my_iface")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("my-net"))
			Expect(iface).To(Equal("my_iface"))
		})

		It("returns error when interface name contains a space", func() {
			_, _, _, err := ParsePodNetworkObjectName("my-net@bad iface")
			Expect(err).To(HaveOccurred())
		})

		It("returns error when interface name exceeds IFNAMSIZ-1", func() {
			longIface := strings.Repeat("a", 16) // IFNAMSIZ is 16, so max name length is 15
			_, _, _, err := ParsePodNetworkObjectName("my-net@" + longIface)
			Expect(err).To(HaveOccurred())
		})
	})
})
