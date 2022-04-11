// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package logutils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
)

var _ = Describe("logutils", Label("unitest", "logutils_test"), Ordered, func() {
	var badFileLoggerConf logutils.FileOutputOption

	BeforeAll(func() {
		badFileLoggerConf.Filename = ""
		badFileLoggerConf.MaxSize = -1
		badFileLoggerConf.MaxAge = -2
		badFileLoggerConf.MaxBackups = -3
	})

	It("Test log with stdout", func() {
		err := logutils.InitStdoutLogger(logutils.InfoLevel)
		Expect(err).NotTo(HaveOccurred())
		logutils.Logger.Info("Test the stdout logger.", zap.String("pkg name", "logutils"))
		logutils.Logger.Sugar().Infow("Test stdout logger sugar.", "key", "value")
		logutils.Logger.Sugar().Info("Info uses fmt.Sprint to construct and log a message. ", "args1 , ", "args2, ", "args3")
	})

	It("Test log for file output mode", func() {
		err := logutils.InitFileLogger(logutils.InfoLevel, logutils.DefaultLogFilePath, logutils.DefaultLogFileMaxSize, logutils.DefaultLogFileMaxAge, logutils.DefaultLogFileMaxBackups)
		Expect(err).NotTo(HaveOccurred())
		logutils.LoggerFile.Info("This is log test.", zap.Int("log mode", 1))
	})

	It("Test log with stderr", func() {
		err := logutils.InitStderrLogger(logutils.InfoLevel)
		Expect(err).NotTo(HaveOccurred())
		logutils.LoggerStderr.Info("This is a clean log.")
		name := "tony"
		logutils.LoggerStderr.Sugar().Infof("Student %s is a good boy", name)
	})

	It("Test NewLoggerWithOption with wrong output mode", func() {
		_, err := logutils.NewLoggerWithOption(logutils.JsonLogFormat, logutils.OUTPUT_STDOUT|logutils.OUTPUT_STDERR,
			nil, false, false, false, logutils.DebugLevel)
		Expect(err).To(HaveOccurred())
	})

	It("Test NewLoggerWithOption with wrong params", func() {
		By("If fortmat and outputMode all wrong, then it will return the default config logger")
		logger, err := logutils.NewLoggerWithOption(logutils.ConsoleLogFormat+logutils.JsonLogFormat, logutils.OUTPUT_STDOUT|logutils.OUTPUT_FILE|logutils.OUTPUT_STDOUT|logutils.OUTPUT_STDERR,
			&badFileLoggerConf, true, true, true, logutils.WarnLevel)
		Expect(err).NotTo(HaveOccurred())
		logger.Warn("hi.")
	})

})
