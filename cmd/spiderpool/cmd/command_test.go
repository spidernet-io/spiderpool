// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("spiderpool plugin", Label("unitest", "ipam_plugin_test"), func() {
	for _, ver := range testutils.AllSpecVersions {
		// Redefine ver inside for scope so real value is picked up by each dynamically defined It()
		// See Gingkgo's "Patterns for dynamically generating tests" documentation.
		tmpVersion := ver

		It(fmt.Sprintf("[%s] allocates addresses with with ADD/DEL", tmpVersion), func() {
			const ifname string = "eth0"
			const nspath string = "/some/where"

			conf := fmt.Sprintf(`{
				"cniVersion": "%s",
				"name": "spiderpool-ipam",
				"ipam": {
					"type": "spiderpool"
				}
			}`, tmpVersion)

			args := &skel.CmdArgs{
				ContainerID: "dummy",
				Netns:       nspath,
				IfName:      ifname,
				StdinData:   []byte(conf),
			}

			// Allocate the IP
			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return CmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())

			// Release the IP
			err = testutils.CmdDelWithArgs(args, func() error {
				return CmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
		})

	}
})
