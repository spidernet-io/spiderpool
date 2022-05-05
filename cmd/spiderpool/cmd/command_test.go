// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

const ifname string = "eth0"
const nspath string = "/some/where"
const containerID string = "dummy"
const CNITimeoutSec = 220

var cniVersion string
var args *skel.CmdArgs
var netConf = func(cniVersion string) string {
	return "{\"cniVersion\": \"" + cniVersion + "\",\"name\": \"spiderpool-ipam\",\"ipam\":{\"type\": \"spiderpool\"}}"
}
var addChan, delChan chan struct{}

var _ = Describe("spiderpool plugin", Label("unitest", "ipam_plugin_test"), func() {
	BeforeEach(func() {
		args = &skel.CmdArgs{
			ContainerID: containerID,
			Netns:       nspath,
			IfName:      ifname,
		}

		cniVersion = "0.3.1"

		addChan = make(chan struct{})
		delChan = make(chan struct{})
	})

	It(fmt.Sprintf("[%s] allocates addresses with ADD/DEL", cniVersion), func() {
		conf := netConf(cniVersion)
		args.StdinData = []byte(conf)

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())
			close(addChan)
		}()
		Eventually(addChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())

		// Release the IP
		go func() {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmd.CmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			close(delChan)
		}()
		Eventually(delChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})

	It(fmt.Sprintf("[%s] is returning an error on conf broken with ADD/DEL", cniVersion), func() {
		conf := netConf(cniVersion) + "}"
		args.StdinData = []byte(conf)

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).To(HaveOccurred())
			close(addChan)
		}()
		Eventually(addChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())

		// Release the IP
		go func() {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmd.CmdDel(args)
			})
			Expect(err).To(HaveOccurred())
			close(delChan)
		}()
		Eventually(delChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})
})
