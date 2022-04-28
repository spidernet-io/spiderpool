// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	cniVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

var version string

func main() {
	skel.PluginMain(cmd.CmdAdd, nil, cmd.CmdDel,
		cniVersion.All,
		"Spiderpool IPAM"+version)
}
