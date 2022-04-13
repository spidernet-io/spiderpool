// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/pkg"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "spiderpool agent daemon",
	Long:  "run spiderpool agent daemon",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	pkg.AgentConfig.BindAgentDaemonFlags(daemonCmd.PersistentFlags())

	rootCmd.AddCommand(daemonCmd)
}
