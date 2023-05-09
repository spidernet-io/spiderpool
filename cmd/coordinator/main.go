// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime"

	"github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"

	"github.com/containernetworking/cni/pkg/skel"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func main() {
	skel.PluginMain(cmd.CmdAdd, cmdCheck, cmd.CmdDel, cniSpecVersion.All, "Coordinator")
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}
