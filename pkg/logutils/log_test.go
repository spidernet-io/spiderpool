// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package logutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

const tmpLogFilePath = "/tmp/cni.log"

var _ = Describe("Log", Label("unittest", "log_test"), func() {
	Context("check log with stdout/stderr/file", func() {
		It("log with stdout", func() {
			err := logutils.InitStdoutLogger(logutils.InfoLevel)
			Expect(err).NotTo(HaveOccurred())
			logutils.Logger.Info("Test the stdout logger.", zap.String("pkg name", "logutils"))
			logutils.Logger.Sugar().Infow("Test stdout logger sugar.", "key", "value")
			logutils.Logger.Sugar().Info("Info uses fmt.Sprint to construct and log a message. ", "args1 , ", "args2, ", "args3")
		})

		It("log for file output mode", func() {
			loggerFile, err := logutils.InitFileLogger(logutils.InfoLevel, tmpLogFilePath, logutils.DefaultLogFileMaxSize, logutils.DefaultLogFileMaxAge, logutils.DefaultLogFileMaxBackups)
			Expect(err).NotTo(HaveOccurred())
			loggerFile.Info("This is log test.", zap.Int("log mode", 1))
		})

		It("log with stderr", func() {
			err := logutils.InitStderrLogger(logutils.InfoLevel)
			Expect(err).NotTo(HaveOccurred())
			logutils.LoggerStderr.Info("This is a clean log.")
			name := "tony"
			logutils.LoggerStderr.Sugar().Infof("Student %s is a good boy", name)
		})
	})

	Context("When NewLoggerWithOption with wrong input", func() {
		var badFileLoggerConf logutils.FileOutputOption

		BeforeEach(func() {
			badFileLoggerConf.Filename = ""
			badFileLoggerConf.MaxSize = -1
			badFileLoggerConf.MaxAge = -2
			badFileLoggerConf.MaxBackups = -3
		})

		It("wrong output mode only", func() {
			_, err := logutils.NewLoggerWithOption(logutils.JSONLogFormat, logutils.OutputStdout|logutils.OutputStderr,
				nil, false, false, false, logutils.DebugLevel)
			Expect(err).To(HaveOccurred())
		})

		It("wrong params", func() {
			// If fortmat and outputMode all wrong, then it will return the default config logger
			logger, err := logutils.NewLoggerWithOption(logutils.ConsoleLogFormat+logutils.JSONLogFormat, logutils.OutputStdout|logutils.OutputFile|logutils.OutputStdout|logutils.OutputStderr,
				&badFileLoggerConf, true, true, true, logutils.WarnLevel)
			Expect(err).NotTo(HaveOccurred())
			logger.Warn("hi.")
		})
	})

	Context("When logger used with context", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		It("none value context", func() {
			logger := logutils.FromContext(ctx)
			Expect(logger).To(Equal(logutils.Logger))
		})

		It("retrieve logger from context", func() {
			original := logutils.Logger.Named("TEST")
			newCtx := logutils.IntoContext(ctx, original)

			logger := logutils.FromContext(newCtx)
			Expect(logger).To(Equal(original))
		})
	})

	DescribeTable("check convert log level", func(levelStr string, expectedLogLevel logutils.LogLevel) {
		logLevel := logutils.ConvertLogLevel(levelStr)
		if logLevel != nil {
			Expect(*logLevel).Should(Equal(expectedLogLevel))
		}
	},
		Entry("expected debug level", "debug", logutils.DebugLevel),
		Entry("expected info level;", "info", logutils.InfoLevel),
		Entry("expected warn level", "warn", logutils.WarnLevel),
		Entry("expected error level", "error", logutils.ErrorLevel),
		Entry("expected panic level", "panic", logutils.PanicLevel),
		Entry("expected fatal level", "fatal", logutils.FatalLevel),
	)
})
