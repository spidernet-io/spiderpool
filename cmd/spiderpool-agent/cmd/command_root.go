// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/pkg/cmdgenmd"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var BinNameAgent = filepath.Base(os.Args[0])
var logger = logutils.Logger.Named(BinNameAgent)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   BinNameAgent,
	Short: BinNameAgent + " cli",
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

	rootCmd.AddCommand(cmdgenmd.GenMarkDownCmd(BinNameAgent, rootCmd, logger))
}
