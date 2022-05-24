// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

var version string

// TODO (Icarus9913): add one doc for spiderpool-plugin configuration file explanation
// TODO (Icarus9913): implement cmdCheck to support CNI 0.3.1 ?
func main() {
	skel.PluginMain(cmd.CmdAdd, nil, cmd.CmdDel,
		cniSpecVersion.PluginSupports(cmd.SupportCNIVersion),
		"Spiderpool IPAM"+version)
}
