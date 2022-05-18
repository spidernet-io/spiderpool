// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/api/v1/agent/server"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
)

const socketPathEnvName = "SPIDER_AGENT_SOCKET_PATH"

var (
	unixServer *server.Server
	err        error
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}

var _ = BeforeSuite(func() {
	// restore the ENV
	DeferCleanup(os.Setenv, socketPathEnvName, os.Getenv(socketPathEnvName))

	t := GinkgoT()
	tempDir := t.TempDir()

	// In concurrency situation, we need use different unix socket path.
	err = os.Setenv(socketPathEnvName, tempDir+"/tmp.sock")
	Expect(err).NotTo(HaveOccurred())

	// refresh singleton agentContext unix socket path.
	cmd.GetAgentContext().RegisterEnv()

	unixServer, err = cmd.NewAgentOpenAPIUnixServer()
	Expect(err).NotTo(HaveOccurred())

	err := os.RemoveAll(string(unixServer.SocketPath))
	Expect(err).NotTo(HaveOccurred())

	startWg := sync.WaitGroup{}
	startWg.Add(1)

	// start unix server.
	go func() {
		defer GinkgoRecover()

		startWg.Done()
		if err = unixServer.Serve(); nil != err {
			if err == net.ErrClosed {
				return
			}
			By("Error: spider agent server finished with error: " + err.Error())
		}
	}()

	startWg.Wait()

	httpClient := &http.Client{Transport: &http.Transport{
		DisableCompression: true,
		Dial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", string(unixServer.SocketPath))
		},
	}}

	// Check unix server to make sure it runs good in BeforeSuite phase.
	// If it starts failed, we will stop the next unit test.
	Eventually(func() bool {
		resp, err := httpClient.Get("http://localhost/")
		if nil != err {
			return false
		}

		// because we do not set a correct route.
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
			return true
		}
		return false
	}).WithPolling(time.Second * 1).WithTimeout(time.Second * 5).Should(BeTrue())
})

var _ = AfterSuite(func() {
	if nil != unixServer {
		err = unixServer.Shutdown()
		Expect(err).NotTo(HaveOccurred())
	} else {
		By("Watch out goroutine leak: The unixServer is nil in 'AfterSuite', please check whether it starts successfully!")
	}

	err := os.RemoveAll(string(unixServer.SocketPath))
	Expect(err).NotTo(HaveOccurred())
})
