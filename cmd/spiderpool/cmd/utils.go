// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
)

// Set up logging for spiderpool plugin
func setupFileLogging(conf *NetConf) (*zap.Logger, error) {
	v := logutils.ConvertLogLevel(conf.IPAM.LogLevel)
	if v == nil {
		return nil, fmt.Errorf("wrong log level %s ", conf.IPAM.LogLevel)
	}

	return logutils.InitFileLogger(*v, conf.IPAM.LogFilePath,
		conf.IPAM.LogFileMaxSize, conf.IPAM.LogFileMaxAge, conf.IPAM.LogFileMaxCount)
}
