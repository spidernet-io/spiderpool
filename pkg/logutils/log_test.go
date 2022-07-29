// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package logutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
)

var _ = Describe("logutils", Label("unitest", "logutils_test"), func() {
	Context("When on a nice day", func() {
		It("log with stdout", func() {
			err := logutils.InitStdoutLogger(logutils.InfoLevel)
			Expect(err).NotTo(HaveOccurred())
			logutils.Logger.Info("Test the stdout logger.", zap.String("pkg name", "logutils"))
			logutils.Logger.Sugar().Infow("Test stdout logger sugar.", "key", "value")
			logutils.Logger.Sugar().Info("Info uses fmt.Sprint to construct and log a message. ", "args1 , ", "args2, ", "args3")
		})

		It("log for file output mode", func() {
			err := logutils.InitFileLogger(logutils.InfoLevel, logutils.DefaultLogFilePath, logutils.DefaultLogFileMaxSize, logutils.DefaultLogFileMaxAge, logutils.DefaultLogFileMaxBackups)
			Expect(err).NotTo(HaveOccurred())
			logutils.LoggerFile.Info("This is log test.", zap.Int("log mode", 1))
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
			_, err := logutils.NewLoggerWithOption(logutils.JsonLogFormat, logutils.OUTPUT_STDOUT|logutils.OUTPUT_STDERR,
				nil, false, false, false, logutils.DebugLevel)
			Expect(err).To(HaveOccurred())
		})

		It("wrong params", func() {
			// If fortmat and outputMode all wrong, then it will return the default config logger
			logger, err := logutils.NewLoggerWithOption(logutils.ConsoleLogFormat+logutils.JsonLogFormat, logutils.OUTPUT_STDOUT|logutils.OUTPUT_FILE|logutils.OUTPUT_STDOUT|logutils.OUTPUT_STDERR,
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
})
