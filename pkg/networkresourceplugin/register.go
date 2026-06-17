// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	devicePluginDirName     = "device-plugins"
	pluginsRegistryDirName  = "plugins_registry"
	kubeletSocketName       = "kubelet.sock"
	legacyDevicePluginDir   = pluginapi.DevicePluginPath
	legacyKubeletSocketPath = pluginapi.KubeletSocket
)

type pluginPathSelection struct {
	pluginDir string
	reason    string
}

func deriveDevicePluginDir(kubeletRootDir string) string {
	if kubeletRootDir == "" || filepath.Clean(kubeletRootDir) == DefaultKubeletRootDir {
		return filepath.Clean(legacyDevicePluginDir)
	}
	return filepath.Join(filepath.Clean(kubeletRootDir), devicePluginDirName)
}

func derivePluginRegistrationDir(kubeletRootDir string) string {
	return filepath.Join(filepath.Clean(kubeletRootDir), pluginsRegistryDirName)
}

func selectPluginDir(kubeletRootDir string, stat func(string) error) pluginPathSelection {
	if kubeletRootDir == "" {
		kubeletRootDir = DefaultKubeletRootDir
	}
	if stat == nil {
		stat = func(path string) error {
			_, err := os.Stat(path)
			return err
		}
	}

	registrationDir := derivePluginRegistrationDir(kubeletRootDir)
	if err := stat(registrationDir); err == nil {
		return pluginPathSelection{pluginDir: registrationDir, reason: "preferred-present"}
	}
	return pluginPathSelection{pluginDir: deriveDevicePluginDir(kubeletRootDir), reason: "fallback-preferred-absent"}
}

type Registrar struct {
	resourceName string
	socketPath   string
	kubeletSock  string
	logger       *zap.Logger
}

func NewRegistrarWithKubeletRootDir(resourceName, kubeletRootDir string, logger *zap.Logger) *Registrar {
	if logger == nil {
		logger = zap.NewNop()
	}
	selection := selectPluginDir(kubeletRootDir, nil)
	// kubelet always listens for plugin registration on kubelet.sock under the
	// device-plugins directory, and looks up plugin sockets under that same
	// directory. The plugins_registry directory is not used by the v1beta1
	// device-plugin socket lookup, so the plugin socket must also live under
	// the device-plugins directory regardless of the preferred registration path.
	pluginSocketDir := deriveDevicePluginDir(kubeletRootDir)
	socketPath := filepath.Join(pluginSocketDir, socketName(resourceName))
	kubeletSock := filepath.Join(pluginSocketDir, kubeletSocketName)

	logger.Info("selected network resource plugin path",
		zap.String("resourceName", resourceName),
		zap.String("pluginDir", selection.pluginDir),
		zap.String("reason", selection.reason),
	)
	return &Registrar{resourceName: resourceName, socketPath: socketPath, kubeletSock: kubeletSock, logger: logger}
}

func socketName(resourceName string) string {
	name := strings.NewReplacer("/", "-", "_", "-", ".", "-").Replace(resourceName)
	return "spiderpool-" + name + ".sock"
}

func (r *Registrar) cleanupSocket() error {
	if err := os.Remove(r.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (r *Registrar) listen() (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(r.socketPath), 0o755); err != nil {
		return nil, err
	}
	if err := r.cleanupSocket(); err != nil {
		return nil, err
	}
	return net.Listen("unix", r.socketPath)
}

func (r *Registrar) register(ctx context.Context) error {
	conn, err := grpc.DialContext(
		ctx,
		r.kubeletSock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	_, err = pluginapi.NewRegistrationClient(conn).Register(ctx, &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     filepath.Base(r.socketPath),
		ResourceName: r.resourceName,
		Options:      &pluginapi.DevicePluginOptions{},
	})
	if err != nil {
		return fmt.Errorf("register network resource plugin %s: %w", r.resourceName, err)
	}
	return nil
}

func (r *Registrar) socketExists() bool {
	_, err := os.Stat(r.socketPath)
	return err == nil
}

func registrationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, 10*time.Second)
}
