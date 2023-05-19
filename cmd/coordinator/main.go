// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"
)

func main() {
	skel.PluginMain(cmd.CmdAdd, cmdCheck, cmd.CmdDel, cniSpecVersion.All, "Coordinator")
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}
