// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package parentnic

import (
	"context"
	"net"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func mustParseMAC(s string) net.HardwareAddr {
	mac, err := net.ParseMAC(s)
	Expect(err).NotTo(HaveOccurred())
	return mac
}

var _ = Describe("ParentNic", Label("unitest"), func() {
	var tmpSysDir string

	newLink := func(name, mac string) netlink.Link {
		attrs := netlink.NewLinkAttrs()
		attrs.Name = name
		if mac != "" {
			attrs.HardwareAddr = mustParseMAC(mac)
		}
		return &netlink.GenericLink{LinkAttrs: attrs}
	}

	// makePhysical creates the sysfs backing-device marker for a NIC.
	makePhysical := func(name string) {
		Expect(os.MkdirAll(filepath.Join(tmpSysDir, name, "device"), 0o755)).To(Succeed())
	}

	BeforeEach(func() {
		tmpSysDir = GinkgoT().TempDir()
		originalSysPath := sysClassNetPath
		originalLister := linkLister
		sysClassNetPath = tmpSysDir
		DeferCleanup(func() {
			sysClassNetPath = originalSysPath
			linkLister = originalLister
		})
	})

	Describe("ListPhysicalNics", func() {
		It("returns only physical NICs with MAC, honoring exclusions", func() {
			linkLister = func() ([]netlink.Link, error) {
				return []netlink.Link{
					newLink("eth0", "fa:16:3e:aa:bb:cc"),
					newLink("eth1", "fa:16:3e:dd:ee:ff"),
					newLink("mgmt0", "fa:16:3e:11:22:33"),
					newLink("veth-abc", "aa:bb:cc:dd:ee:01"), // virtual: no sysfs device
					newLink("lo", ""),                        // no MAC
				}, nil
			}
			makePhysical("eth0")
			makePhysical("eth1")
			makePhysical("mgmt0")

			nics, err := ListPhysicalNics([]string{"mgmt0"})
			Expect(err).NotTo(HaveOccurred())
			Expect(nics).To(Equal(map[string]string{
				"eth0": "fa:16:3e:aa:bb:cc",
				"eth1": "fa:16:3e:dd:ee:ff",
			}))
		})
	})

	Describe("ReportParentNics", func() {
		It("patches the parent-nics annotation onto the Node", func() {
			linkLister = func() ([]netlink.Link, error) {
				return []netlink.Link{newLink("eth0", "fa:16:3e:aa:bb:cc")}, nil
			}
			makePhysical("eth0")

			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}}
			clientSet := k8sfake.NewSimpleClientset(node)

			err := ReportParentNics(context.TODO(), clientSet, "node1", nil, logutils.Logger)
			Expect(err).NotTo(HaveOccurred())

			got, err := clientSet.CoreV1().Nodes().Get(context.TODO(), "node1", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Annotations).To(HaveKeyWithValue(
				constant.AnnoNodeParentNics, `{"eth0":"fa:16:3e:aa:bb:cc"}`))
		})

		It("fails when no physical NIC is found", func() {
			linkLister = func() ([]netlink.Link, error) {
				return []netlink.Link{newLink("veth0", "aa:bb:cc:dd:ee:02")}, nil
			}

			clientSet := k8sfake.NewSimpleClientset()
			err := ReportParentNics(context.TODO(), clientSet, "node1", nil, logutils.Logger)
			Expect(err).To(MatchError(ContainSubstring("no physical NIC found")))
		})
	})
})
