// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

var version string

func main() {
	skel.PluginMain(cmd.CmdAdd, nil, cmd.CmdDel,
		cniSpecVersion.PluginSupports("0.3.1"),
		"Spiderpool IPAM"+version)
}
