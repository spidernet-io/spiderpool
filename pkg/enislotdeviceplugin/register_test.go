// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ENI slot device plugin registrar", Label("enislotdeviceplugin_register_test"), func() {
	Describe("deriveDevicePluginDir", func() {
		It("returns the legacy path for the default kubelet root", func() {
			Expect(deriveDevicePluginDir(DefaultKubeletRootDir)).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		})

		It("returns the legacy path for an empty kubelet root", func() {
			Expect(deriveDevicePluginDir("")).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		})

		It("returns a custom path for a non-default kubelet root", func() {
			Expect(deriveDevicePluginDir("/custom/kubelet")).To(Equal("/custom/kubelet/" + devicePluginDirName))
		})
	})

	Describe("derivePluginRegistrationDir", func() {
		It("appends plugins_registry to the kubelet root", func() {
			Expect(derivePluginRegistrationDir("/var/lib/kubelet")).To(Equal("/var/lib/kubelet/" + pluginsRegistryDirName))
		})

		It("cleans the result path", func() {
			Expect(derivePluginRegistrationDir("/var/lib/kubelet/")).To(Equal("/var/lib/kubelet/" + pluginsRegistryDirName))
		})
	})

	It("prefers the plugins registry path when present", func() {
		root := GinkgoT().TempDir()
		registryDir := filepath.Join(root, pluginsRegistryDirName)
		kubeletSock := filepath.Join(registryDir, kubeletSocketName)

		selection := selectPluginDir(root, func(path string) error {
			if path == kubeletSock {
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

	It("falls back to the device plugin path when plugins registry has no kubelet socket", func() {
		root := GinkgoT().TempDir()
		registryDir := filepath.Join(root, pluginsRegistryDirName)

		selection := selectPluginDir(root, func(path string) error {
			if path == registryDir {
				return nil
			}
			return os.ErrNotExist
		})

		Expect(selection.pluginDir).To(Equal(filepath.Join(root, devicePluginDirName)))
		Expect(selection.reason).To(Equal("fallback-preferred-absent"))
	})

	It("uses the default kubelet root when selecting with an empty root", func() {
		selection := selectPluginDir("", func(string) error {
			return os.ErrNotExist
		})

		Expect(selection.pluginDir).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		Expect(selection.reason).To(Equal("fallback-preferred-absent"))
	})

	It("derives registrar sockets from a non-default kubelet root", func() {
		root := GinkgoT().TempDir()
		registryDir := filepath.Join(root, pluginsRegistryDirName)
		Expect(os.Mkdir(registryDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(registryDir, kubeletSocketName), nil, 0o600)).To(Succeed())

		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/sub-eni", root, nil)

		Expect(registrar.socketPath).To(Equal(filepath.Join(root, pluginsRegistryDirName, eniDevicePluginSock)))
		Expect(registrar.kubeletSock).To(Equal(filepath.Join(root, pluginsRegistryDirName, kubeletSocketName)))
	})

	It("defaults registrar resource name and logger", func() {
		registrar := NewRegistrarWithKubeletRootDir("", DefaultKubeletRootDir, nil)

		Expect(registrar.resourceName).To(Equal(defaultResourceName()))
		Expect(registrar.logger).NotTo(BeNil())
	})

	It("cleans up an existing stale socket path", func() {
		dir := GinkgoT().TempDir()
		socketPath := filepath.Join(dir, "stale.sock")
		Expect(os.WriteFile(socketPath, []byte("stale"), 0o600)).To(Succeed())

		registrar := NewRegistrar("spidernet.io/sub-eni", nil)
		registrar.socketPath = socketPath

		Expect(registrar.cleanupSocket()).To(Succeed())
		_, err := os.Stat(socketPath)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("detects socket removal for re-registration", func() {
		dir := GinkgoT().TempDir()
		socketPath := filepath.Join(dir, "plugin.sock")
		Expect(os.WriteFile(socketPath, []byte("socket"), 0o600)).To(Succeed())
		registrar := NewRegistrar("spidernet.io/sub-eni", nil)
		registrar.socketPath = socketPath

		Expect(registrar.socketExists()).To(BeTrue())
		Expect(os.Remove(socketPath)).To(Succeed())
		Expect(registrar.socketExists()).To(BeFalse())
	})

	It("returns registration errors when the kubelet socket is unavailable", func() {
		dir := GinkgoT().TempDir()
		registrar := NewRegistrar("spidernet.io/sub-eni", nil)
		registrar.kubeletSock = filepath.Join(dir, "missing-kubelet.sock")

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		err := registrar.register(ctx)
		Expect(err).To(HaveOccurred())
	})

	It("listen creates the socket directory and returns a listener", func() {
		root := GinkgoT().TempDir()
		registrar := NewRegistrar("spidernet.io/sub-eni", nil)
		registrar.socketPath = filepath.Join(root, "sub", "plugin.sock")

		ln, err := registrar.listen()
		if err != nil && errors.Is(err, syscall.EPERM) {
			Skip("Unix socket creation is not permitted in this test environment")
		}

		Expect(err).NotTo(HaveOccurred())
		Expect(ln).NotTo(BeNil())
		_ = ln.Close()
		_, statErr := os.Stat(filepath.Dir(registrar.socketPath))
		Expect(statErr).NotTo(HaveOccurred())
	})

	It("NewRegistrar uses the legacy device plugin path when plugins_registry is absent", func() {
		registrar := NewRegistrar("spidernet.io/sub-eni", nil)

		Expect(registrar.socketPath).To(Equal(filepath.Join(filepath.Clean(legacyDevicePluginDir), eniDevicePluginSock)))
		Expect(registrar.kubeletSock).To(Equal(legacyKubeletSocketPath))
	})

	It("registrationContext is cancelled when the parent context is cancelled", func() {
		parent, parentCancel := context.WithCancel(context.Background())
		ctx, cancel := registrationContext(parent)
		defer cancel()

		parentCancel()

		Eventually(ctx.Done()).Should(BeClosed())
		Expect(ctx.Err()).To(MatchError(context.Canceled))
	})

	It("registrationContext tolerates a nil parent context", func() {
		var parent context.Context
		ctx, cancel := registrationContext(parent)
		defer cancel()

		Expect(ctx).NotTo(BeNil())
		Expect(ctx.Err()).NotTo(HaveOccurred())
	})
})
