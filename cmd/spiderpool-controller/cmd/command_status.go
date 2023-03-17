// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// shutdownCmd represents the shutdown command.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: binNameController + " status",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO (Icarus9913)
		logger.Sugar().Infof("This is %s status...", binNameController)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
