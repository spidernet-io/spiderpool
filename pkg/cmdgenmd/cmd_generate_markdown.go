// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmdgenmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"go.uber.org/zap"
)

var markdownPath string

// GenMarkDownCmd returns cobra.Command that help to generate markdown.
// The first param is the root cmd component name, the second one is the root cmd,
// the third one should be the root cmd logger.
func GenMarkDownCmd(component string, rootCmd *cobra.Command, logger *zap.Logger) *cobra.Command {
	var genMarkDownCmd = &cobra.Command{
		Use:   "generate-markdown",
		Short: "generate markdown for cli " + component,
		Run: func(cmd *cobra.Command, args []string) {
			if markdownPath != "" {
				if err := doc.GenMarkdownTree(rootCmd, markdownPath); nil != err {
					logger.Fatal(err.Error())
				}
			}
		},
	}

	genMarkDownCmd.Flags().StringVar(&markdownPath, "markdown-path", "", "generate markdown for cli "+component)
	err := genMarkDownCmd.MarkFlagRequired("markdown-path")
	if nil != err {
		logger.Error(err.Error())
	}

	return genMarkDownCmd
}
