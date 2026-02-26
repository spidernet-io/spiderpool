// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/utils/cmdgenmd"
)

var (
	binNameController = filepath.Base(os.Args[0])
	logger            = logutils.Logger.Named(binNameController)
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   binNameController,
	Short: binNameController,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cleanCmd.Flags().String("validate", "", "Specify validate parameter")
	cleanCmd.Flags().String("mutating", "", "Specify mutating parameter")

	rootCmd.AddCommand(cleanCmd)
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err.Error())
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.AddCommand(cmdgenmd.GenMarkDownCmd(binNameController, rootCmd, logger))
}
