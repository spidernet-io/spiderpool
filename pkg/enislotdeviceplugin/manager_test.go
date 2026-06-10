// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("ENI slot device plugin manager", Label("enislotdeviceplugin_manager_test"), func() {
	It("applies resource name default when empty", func() {
		mgr := NewManager(Config{KubeletRootDir: DefaultKubeletRootDir}, nil)

		Expect(mgr.config.ResourceName).To(Equal(defaultResourceName()))
	})

	It("applies kubelet root dir default when empty", func() {
		mgr := NewManager(Config{ResourceName: defaultResourceName()}, nil)

		Expect(mgr.config.KubeletRootDir).To(Equal(DefaultKubeletRootDir))
	})

	It("initializes the registrar with defaulted manager config", func() {
		mgr := NewManager(Config{}, nil)

		Expect(mgr.logger).NotTo(BeNil())
		Expect(mgr.registrar).NotTo(BeNil())
		Expect(mgr.registrar.resourceName).To(Equal(defaultResourceName()))
		Expect(mgr.registrar.kubeletSock).To(Equal(legacyKubeletSocketPath))
	})

	It("initializes the registrar with custom manager config", func() {
		root := GinkgoT().TempDir()
		registryDir := filepath.Join(root, pluginsRegistryDirName)
		Expect(os.Mkdir(registryDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(registryDir, kubeletSocketName), nil, 0o600)).To(Succeed())

		mgr := NewManager(Config{
			ResourceName:   "example.com/custom-eni",
			KubeletRootDir: root,
		}, nil)

		Expect(mgr.config.ResourceName).To(Equal("example.com/custom-eni"))
		Expect(mgr.config.KubeletRootDir).To(Equal(root))
		Expect(mgr.registrar.resourceName).To(Equal("example.com/custom-eni"))
		Expect(mgr.registrar.socketPath).To(Equal(filepath.Join(root, pluginsRegistryDirName, eniDevicePluginSock)))
	})

	It("Start returns immediately when the plugin is disabled", func() {
		mgr := NewManager(Config{
			Enabled:        false,
			ResourceName:   defaultResourceName(),
			KubeletRootDir: DefaultKubeletRootDir,
		}, nil)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Must not block.
		done := make(chan struct{})
		go func() {
			mgr.Start(ctx)
			close(done)
		}()
		Eventually(done).Should(BeClosed())
	})

	It("Start tolerates a nil context", func() {
		dir := GinkgoT().TempDir()
		mgr := NewManager(Config{
			Enabled:         true,
			ResourceName:    defaultResourceName(),
			MaxSlotsPerNode: 1,
			KubeletRootDir:  DefaultKubeletRootDir,
		}, nil)
		mgr.registrar.socketPath = filepath.Join(dir, eniDevicePluginSock)
		mgr.registrar.kubeletSock = filepath.Join(dir, "missing-kubelet.sock")

		var ctx context.Context
		Expect(func() { mgr.Start(ctx) }).NotTo(Panic())
		Eventually(func() bool {
			_, err := os.Stat(mgr.registrar.socketPath)
			return os.IsNotExist(err)
		}, 2*time.Second).Should(BeTrue())
	})

	It("Stop is safe when grpcServer is nil", func() {
		mgr := NewManager(Config{
			ResourceName:   defaultResourceName(),
			KubeletRootDir: DefaultKubeletRootDir,
		}, nil)

		Expect(func() { mgr.Stop() }).NotTo(Panic())
	})

	It("Stop is safe when registrar is nil", func() {
		mgr := NewManager(Config{
			ResourceName:   defaultResourceName(),
			KubeletRootDir: DefaultKubeletRootDir,
		}, nil)
		mgr.registrar = nil

		Expect(func() { mgr.Stop() }).NotTo(Panic())
	})

	It("Stop stops an initialized grpc server", func() {
		mgr := NewManager(Config{
			ResourceName:   defaultResourceName(),
			KubeletRootDir: DefaultKubeletRootDir,
		}, nil)
		mgr.grpcServer = grpc.NewServer()

		Expect(func() { mgr.Stop() }).NotTo(Panic())
	})

	It("Stop cleans up the socket path", func() {
		dir := GinkgoT().TempDir()
		mgr := NewManager(Config{
			ResourceName:   defaultResourceName(),
			KubeletRootDir: DefaultKubeletRootDir,
		}, nil)
		mgr.registrar.socketPath = dir + "/test.sock"

		Expect(mgr.Stop).NotTo(Panic())
	})

	It("serveAndRegister returns an error when the kubelet socket is unavailable", func() {
		dir := GinkgoT().TempDir()
		mgr := NewManager(Config{
			Enabled:         true,
			ResourceName:    defaultResourceName(),
			MaxSlotsPerNode: 1,
			KubeletRootDir:  DefaultKubeletRootDir,
		}, nil)
		mgr.registrar.socketPath = filepath.Join(dir, eniDevicePluginSock)
		mgr.registrar.kubeletSock = filepath.Join(dir, "missing-kubelet.sock")

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := mgr.serveAndRegister(ctx)

		Expect(err).To(HaveOccurred())
		_, statErr := os.Stat(mgr.registrar.socketPath)
		Expect(os.IsNotExist(statErr)).To(BeTrue(), "socket should be cleaned up after failure")
	})

	It("run exits cleanly when context is cancelled before registration succeeds", func() {
		dir := GinkgoT().TempDir()
		mgr := NewManager(Config{
			Enabled:         true,
			ResourceName:    defaultResourceName(),
			MaxSlotsPerNode: 1,
			KubeletRootDir:  DefaultKubeletRootDir,
		}, nil)
		mgr.registrar.socketPath = filepath.Join(dir, eniDevicePluginSock)
		mgr.registrar.kubeletSock = filepath.Join(dir, "missing-kubelet.sock")

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			mgr.run(ctx)
			close(done)
		}()

		Eventually(done, 2*time.Second).Should(BeClosed())
	})
})
