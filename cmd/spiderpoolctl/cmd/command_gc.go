// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// gcCmd represents the gc command.
var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "spiderpool gc",
	Long:  `trigger GC request to spiderpool-controller`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("This is spiderpool ctl gc...")
	},
}

func init() {
	rootCmd.AddCommand(gcCmd)
}
