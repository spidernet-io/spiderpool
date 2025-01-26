// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// Set up file logging for spiderpool bin.
func SetupFileLogging(conf *NetConf) (*zap.Logger, error) {
	v := logutils.ConvertLogLevel(conf.IPAM.LogLevel)
	if v == nil {
		return nil, fmt.Errorf("unsupported log level %s", conf.IPAM.LogLevel)
	}

	return logutils.InitFileLogger(
		*v,
		conf.IPAM.LogFilePath,
		conf.IPAM.LogFileMaxSize,
		conf.IPAM.LogFileMaxAge,
		conf.IPAM.LogFileMaxCount,
	)
}
