// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/utils/cmdgenmd"
)

const SPIDERPOOL_CTL = "spiderpoolctl"

var logger = logutils.Logger.Named(SPIDERPOOL_CTL)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "spiderpoolctl",
	Short: "spiderpoolctl cli",
	Long:  `spiderpoolctl cli for debug`,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddCommand(cmdgenmd.GenMarkDownCmd(SPIDERPOOL_CTL, rootCmd, logger))
}
