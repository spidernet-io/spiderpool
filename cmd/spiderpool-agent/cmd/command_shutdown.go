// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// shutdownCmd represents the shutdown command
var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "shutdown spiderpool agent",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("shutdown spiderpool agent...")
	},
}

func init() {
	rootCmd.AddCommand(shutdownCmd)
}
