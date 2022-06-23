// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// Set up logging for spiderpool plugin
func setupFileLogging(conf *NetConf) error {

	v := logutils.ConvertLogLevel(conf.IPAM.LogLevel)
	if v == nil {
		return fmt.Errorf("wrong log level %s ", conf.IPAM.LogLevel)
	}

	err := logutils.InitFileLogger(*v, conf.IPAM.LogFilePath,
		conf.IPAM.LogFileMaxSize, conf.IPAM.LogFileMaxAge, conf.IPAM.LogFileMaxCount)
	if nil != err {
		return err
	}

	return nil
}
