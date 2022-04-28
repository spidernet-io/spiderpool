// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

const ifname string = "eth0"
const nspath string = "/some/where"
const containerID string = "dummy"

var cniVersion string
var args *skel.CmdArgs
var netConf = func(cniVersion string) string {
	return "{\"cniVersion\": \"" + cniVersion + "\",\"name\": \"spiderpool-ipam\",\"ipam\":{\"type\": \"spiderpool\"}}"
}

var _ = Describe("spiderpool plugin", Label("unitest", "ipam_plugin_test"), func() {
	BeforeEach(func() {
		args = &skel.CmdArgs{
			ContainerID: containerID,
			Netns:       nspath,
			IfName:      ifname,
		}

		cniVersion = "0.3.1"
	})

	It(fmt.Sprintf("[%s] allocates addresses with ADD/DEL", cniVersion), func() {
		conf := netConf(cniVersion)
		args.StdinData = []byte(conf)

		// Allocate the IP
		_, _, err := testutils.CmdAddWithArgs(args, func() error {
			return cmd.CmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())

		// Release the IP
		err = testutils.CmdDelWithArgs(args, func() error {
			return cmd.CmdDel(args)
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It(fmt.Sprintf("[%s] is returning an error on conf broken with ADD/DEL", cniVersion), func() {
		conf := netConf(cniVersion) + "}"
		args.StdinData = []byte(conf)

		// Allocate the IP
		_, _, err := testutils.CmdAddWithArgs(args, func() error {
			return cmd.CmdAdd(args)
		})
		Expect(err).To(HaveOccurred())

		// Release the IP
		err = testutils.CmdDelWithArgs(args, func() error {
			return cmd.CmdDel(args)
		})
		Expect(err).To(HaveOccurred())
	})
})
