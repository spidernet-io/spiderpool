// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/pkg/cmdgenmd"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

const AGENT_CTL = "agent-cli"

var logger = logutils.Logger.Named(AGENT_CTL)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "spiderpool-agent",
	Short: "spiderpoll agent cli",
	Long:  `spiderpoll agent cli for interacting with the spiderpool agent`,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}

func init() {
	rootCmd.AddCommand(cmdgenmd.GenMarkDownCmd(AGENT_CTL, rootCmd, logger))
}
