// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// ipCmd represents the base command.
var ipCmd = &cobra.Command{
	Use:   "ip",
	Short: "spiderpoolclt ip cli",
	Long:  `spiderpoolclt ip cli to interact with ip`,
}

// ipShowCmd represents the show command.
var ipShowCmd = &cobra.Command{
	Use:   "show",
	Short: "show ip related data",
	Long:  `show pod who is taking this ip`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("This is spiderpool ctl ip show...")
	},
}

// ipReleaseCmd represents the release command.
var ipReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "try to release ip",
	Long:  `try to release ip and other related data`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("This is spiderpool ctl ip release...")
	},
}

// ipSetCmd represents the set command.
var ipSetCmd = &cobra.Command{
	Use:   "set",
	Short: "set ip to be taken by a pod",
	Long:  `set ip to be taken by a pod , this will update ippool and workloadendpoint resource`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("This is spiderpool ctl ip set...")
	},
}

func init() {
	// show flags
	ipShowCmd.PersistentFlags().String("ip", "", "[optional] ip")

	// release flags
	ipReleaseCmd.PersistentFlags().String("ip", "", "[required] ip")
	err := ipReleaseCmd.MarkPersistentFlagRequired("ip")
	if nil != err {
		logger.Error(err.Error())
	}
	ipReleaseCmd.PersistentFlags().BoolP("force", "f", false, "force release ip")

	// set flags
	ipSetCmd.PersistentFlags().String("ip", "", "[required] ip")
	ipSetCmd.PersistentFlags().String("pod", "", "[required] pod name")
	ipSetCmd.PersistentFlags().String("namespace", "", "[required] pod namespace")
	ipSetCmd.PersistentFlags().String("containerid", "", "[required] pod container id")
	ipSetCmd.PersistentFlags().String("node", "", "[required] the node name who the pod locates")
	ipSetCmd.PersistentFlags().String("interface", "", "[required] pod interface who taking effect the ip")

	err = ipSetCmd.MarkPersistentFlagRequired("ip")
	if nil != err {
		logger.Error(err.Error())
	}
	err = ipSetCmd.MarkPersistentFlagRequired("pod")
	if nil != err {
		logger.Error(err.Error())
	}
	err = ipSetCmd.MarkPersistentFlagRequired("namespace")
	if nil != err {
		logger.Error(err.Error())
	}
	err = ipSetCmd.MarkPersistentFlagRequired("containerid")
	if nil != err {
		logger.Error(err.Error())
	}
	err = ipSetCmd.MarkPersistentFlagRequired("node")
	if nil != err {
		logger.Error(err.Error())
	}
	err = ipSetCmd.MarkPersistentFlagRequired("interface")
	if nil != err {
		logger.Error(err.Error())
	}

	rootCmd.AddCommand(ipCmd)
	ipCmd.AddCommand(ipShowCmd)
	ipCmd.AddCommand(ipReleaseCmd)
	ipCmd.AddCommand(ipSetCmd)
}
