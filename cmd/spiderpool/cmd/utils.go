// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// Set up logging for spiderpool plugin
func setupFileLogging(conf *NetConf) error {
	var logLevel logutils.LogLevel
	if strings.EqualFold(conf.IPAM.LogLevel, constant.LogDebugLevelStr) {
		logLevel = logutils.DebugLevel
	} else if strings.EqualFold(conf.IPAM.LogLevel, constant.LogInfoLevelStr) {
		logLevel = logutils.InfoLevel
	} else if strings.EqualFold(conf.IPAM.LogLevel, constant.LogWarnLevelStr) {
		logLevel = logutils.WarnLevel
	} else if strings.EqualFold(conf.IPAM.LogLevel, constant.LogErrorLevelStr) {
		logLevel = logutils.ErrorLevel
	} else if strings.EqualFold(conf.IPAM.LogLevel, constant.LogFatalLevelStr) {
		logLevel = logutils.FatalLevel
	} else if strings.EqualFold(conf.IPAM.LogLevel, constant.LogPanicLevelStr) {
		logLevel = logutils.PanicLevel
	} else {
		return fmt.Errorf("There's no match %s log level", conf.IPAM.LogLevel)
	}

	err := logutils.InitFileLogger(logLevel, conf.IPAM.LogFilePath,
		conf.IPAM.LogFileMaxSize, conf.IPAM.LogFileMaxAge, conf.IPAM.LogFileMaxCount)
	if nil != err {
		return err
	}

	return nil
}
