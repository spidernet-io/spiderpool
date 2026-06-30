// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("kubelet plugin path selection", Label("networkresourceplugin_register_test"), func() {
	It("derives device plugin directories from kubelet roots", func() {
		Expect(deriveDevicePluginDir("")).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		Expect(deriveDevicePluginDir(DefaultKubeletRootDir)).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		Expect(deriveDevicePluginDir("/var/lib/custom-kubelet/")).To(Equal(filepath.Join("/var/lib/custom-kubelet", devicePluginDirName)))
	})

	It("derives plugin registration directories from kubelet roots", func() {
		Expect(derivePluginRegistrationDir(DefaultKubeletRootDir)).To(Equal(filepath.Join(DefaultKubeletRootDir, pluginsRegistryDirName)))
		Expect(derivePluginRegistrationDir("/var/lib/custom-kubelet/")).To(Equal(filepath.Join("/var/lib/custom-kubelet", pluginsRegistryDirName)))
	})

	It("uses the default kubelet device-plugin path for the default root fallback", func() {
		selection := selectPluginDir(DefaultKubeletRootDir, func(string) error {
			return errors.New("missing")
		})

		Expect(selection.pluginDir).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		Expect(selection.reason).To(Equal("fallback-preferred-absent"))
	})

	It("prefers plugins_registry when present", func() {
		root := GinkgoT().TempDir()
		selection := selectPluginDir(root, func(path string) error {
			if path == filepath.Join(root, pluginsRegistryDirName) {
				return nil
			}
			return errors.New("missing")
		})

		Expect(selection.pluginDir).To(Equal(filepath.Join(root, pluginsRegistryDirName)))
		Expect(selection.reason).To(Equal("preferred-present"))
	})

	It("falls back to device-plugins for non-default roots", func() {
		root := GinkgoT().TempDir()
		selection := selectPluginDir(root, func(string) error {
			return errors.New("missing")
		})

		Expect(selection.pluginDir).To(Equal(filepath.Join(root, devicePluginDirName)))
		Expect(selection.reason).To(Equal("fallback-preferred-absent"))
	})

	It("uses default kubelet root when selecting with an empty root", func() {
		selection := selectPluginDir("", func(path string) error {
			Expect(path).To(Equal(filepath.Join(DefaultKubeletRootDir, pluginsRegistryDirName)))
			return errors.New("missing")
		})

		Expect(selection.pluginDir).To(Equal(filepath.Clean(legacyDevicePluginDir)))
		Expect(selection.reason).To(Equal("fallback-preferred-absent"))
	})

	It("builds stable socket names", func() {
		Expect(socketName("spidernet.io/sub-eni")).To(Equal("spiderpool-spidernet-io-sub-eni.sock"))
		Expect(socketName("example.com/eth0_nic")).To(Equal("spiderpool-example-com-eth0-nic.sock"))
	})

	It("configures registrar paths for legacy fallback", func() {
		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/sub-eni", DefaultKubeletRootDir, nil)

		Expect(registrar.socketPath).To(Equal(filepath.Join(filepath.Clean(legacyDevicePluginDir), "spiderpool-spidernet-io-sub-eni.sock")))
		Expect(registrar.kubeletSock).To(Equal(legacyKubeletSocketPath))
	})

	It("configures registrar paths for non-default kubelet roots", func() {
		root := GinkgoT().TempDir()
		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/sub-eni", root, nil)

		Expect(registrar.socketPath).To(Equal(filepath.Join(root, devicePluginDirName, "spiderpool-spidernet-io-sub-eni.sock")))
		Expect(registrar.kubeletSock).To(Equal(filepath.Join(root, devicePluginDirName, kubeletSocketName)))
	})

	It("prefers plugins_registry for registrar paths when it exists", func() {
		root := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(root, pluginsRegistryDirName), 0o755)).To(Succeed())

		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/sub-eni", root, nil)

		Expect(registrar.socketPath).To(Equal(filepath.Join(root, devicePluginDirName, "spiderpool-spidernet-io-sub-eni.sock")))
		Expect(registrar.kubeletSock).To(Equal(filepath.Join(root, devicePluginDirName, kubeletSocketName)))
	})

	It("detects and cleans up socket paths", func() {
		root := GinkgoT().TempDir()
		registrar := NewRegistrarWithKubeletRootDir("spidernet.io/sub-eni", root, nil)
		Expect(os.MkdirAll(filepath.Dir(registrar.socketPath), 0o755)).To(Succeed())
		Expect(os.WriteFile(registrar.socketPath, []byte("stale socket"), 0o644)).To(Succeed())

		Expect(registrar.socketExists()).To(BeTrue())
		Expect(registrar.cleanupSocket()).To(Succeed())
		Expect(registrar.socketExists()).To(BeFalse())
		Expect(registrar.cleanupSocket()).To(Succeed())
	})

	It("creates bounded registration contexts from nil and parent contexts", func() {
		var parent context.Context
		ctx, cancel := registrationContext(parent)
		defer cancel()
		deadline, ok := ctx.Deadline()
		Expect(ok).To(BeTrue())
		Expect(time.Until(deadline)).To(BeNumerically(">", 0))

		parent, parentCancel := context.WithCancel(context.Background())
		ctx, cancel = registrationContext(parent)
		parentCancel()
		defer cancel()
		Eventually(ctx.Done()).Should(BeClosed())
	})
})
