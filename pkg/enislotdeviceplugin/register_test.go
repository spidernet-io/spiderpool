// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ENI slot device plugin registrar", Label("enislotdeviceplugin_register_test"), func() {
	It("prefers the plugins registry path when present", func() {
		root := GinkgoT().TempDir()
		registryDir := filepath.Join(root, pluginsRegistryDirName)

		selection := selectPluginDir(root, func(path string) error {
			if path == registryDir {
				return nil
			}
			return errors.New("missing")
		})

		Expect(selection.pluginDir).To(Equal(registryDir))
		Expect(selection.reason).To(Equal(pluginPathSelectionDefault))
	})

	It("falls back to the device plugin path when plugins registry is absent", func() {
		root := GinkgoT().TempDir()

		selection := selectPluginDir(root, func(string) error {
			return os.ErrNotExist
		})

		Expect(selection.pluginDir).To(Equal(filepath.Join(root, devicePluginDirName)))
		Expect(selection.reason).To(Equal("fallback-preferred-absent"))
	})

	It("derives registrar sockets from a non-default kubelet root", func() {
		root := GinkgoT().TempDir()
		Expect(os.Mkdir(filepath.Join(root, pluginsRegistryDirName), 0o755)).To(Succeed())

		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/eni-slot", root, nil)

		Expect(registrar.socketPath).To(Equal(filepath.Join(root, pluginsRegistryDirName, eniDevicePluginSock)))
		Expect(registrar.kubeletSock).To(Equal(filepath.Join(root, pluginsRegistryDirName, kubeletSocketName)))
	})

	It("cleans up an existing stale socket path", func() {
		dir := GinkgoT().TempDir()
		socketPath := filepath.Join(dir, "stale.sock")
		Expect(os.WriteFile(socketPath, []byte("stale"), 0o600)).To(Succeed())

		registrar := NewRegistrar("spidernet.io/eni-slot", nil)
		registrar.socketPath = socketPath

		Expect(registrar.cleanupSocket()).To(Succeed())
		_, err := os.Stat(socketPath)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("detects socket removal for re-registration", func() {
		dir := GinkgoT().TempDir()
		socketPath := filepath.Join(dir, "plugin.sock")
		Expect(os.WriteFile(socketPath, []byte("socket"), 0o600)).To(Succeed())
		registrar := NewRegistrar("spidernet.io/eni-slot", nil)
		registrar.socketPath = socketPath

		Expect(registrar.socketExists()).To(BeTrue())
		Expect(os.Remove(socketPath)).To(Succeed())
		Expect(registrar.socketExists()).To(BeFalse())
	})

	It("returns registration errors when the kubelet socket is unavailable", func() {
		dir := GinkgoT().TempDir()
		registrar := NewRegistrar("spidernet.io/eni-slot", nil)
		registrar.kubeletSock = filepath.Join(dir, "missing-kubelet.sock")

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		err := registrar.register(ctx)
		Expect(err).To(HaveOccurred())
	})
})
