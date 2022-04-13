// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-controller/pkg"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "spiderpool controller daemon",
	Long:  "run spiderpool controller daemon",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	pkg.ControllerConfig.BindControllerDaemonFlags(daemonCmd.PersistentFlags())

	rootCmd.AddCommand(daemonCmd)
}
