// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
)

// CmdAdd follows CNI SPEC cmdAdd.
func CmdAdd(args *skel.CmdArgs) error {
	conf := types.NetConf{}
	err := json.Unmarshal(args.StdinData, &conf)
	if nil != err {
		return err
	}

	// new unix client
	spiderpoolAgentAPI := cmd.NewAgentOpenAPIUnixClient()

	// GET /ipam/healthy
	_, err = spiderpoolAgentAPI.Connectivity.GetIpamHealthy(connectivity.NewGetIpamHealthyParams())
	if nil != err {
		return err
	}

	// POST /ipam/ip
	_, err = spiderpoolAgentAPI.Daemonset.PostIpamIP(daemonset.NewPostIpamIPParams())
	if nil != err {
		return err
	}

	result := &current.Result{
		CNIVersion: conf.CNIVersion,
	}

	return types.PrintResult(result, conf.CNIVersion)
}
