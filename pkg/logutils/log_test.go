// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package logutils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap/zapcore"
)

var _ = Describe("Log Test", func() {
	It("Test log with stdout", func() {
		err := logutils.InitStdoutLogger()
		Expect(err).NotTo(HaveOccurred())
		logutils.Logger.Info("Test the stdout logger.")
	})

	It("Test log for file output mode", func() {
		err := logutils.InitFileLogger("/tmp/cni.log")
		Expect(err).NotTo(HaveOccurred())
		logutils.LoggerFile.Info("This is log test")
	})

	It("Test log with stderr", func() {
		err := logutils.InitStderrLogger()
		Expect(err).NotTo(HaveOccurred())
		logutils.LoggerStderr.Info("This is a clean log")
		name := "tony"
		logutils.LoggerStderr.Sugar().Infof("Student %s is a good boy", name)
	})

	It("Test ConvertToZapLevel", func() {
		convertToZapLevel := logutils.ConvertToZapLevel(logutils.LogInfo)
		Expect(convertToZapLevel).Should(Equal(zapcore.InfoLevel))
	})
})
