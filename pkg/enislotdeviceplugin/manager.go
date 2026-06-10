// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/metric"
)

type Manager struct {
	config    Config
	logger    *zap.Logger
	registrar *Registrar

	grpcServer *grpc.Server
}

func NewManager(config Config, logger *zap.Logger) *Manager {
	if config.ResourceName == "" {
		config.ResourceName = defaultResourceName()
	}
	if config.KubeletRootDir == "" {
		config.KubeletRootDir = DefaultKubeletRootDir
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Manager{
		config:    config,
		logger:    logger,
		registrar: NewRegistrarWithKubeletRootDir(config.ResourceName, config.KubeletRootDir, logger),
	}
}

func (m *Manager) Start(ctx context.Context) {
	if !m.config.Enabled {
		m.logger.Info("ENI slot device plugin is disabled")
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	go m.run(ctx)
}

func (m *Manager) Stop() {
	if m.grpcServer != nil {
		m.grpcServer.Stop()
	}
	if m.registrar != nil {
		if err := m.registrar.cleanupSocket(); err != nil {
			m.logger.Warn("failed to remove ENI slot device plugin socket", zap.Error(err))
		}
	}
}

func (m *Manager) run(ctx context.Context) {
	for {
		if err := m.serveAndRegister(ctx); err != nil && ctx.Err() == nil {
			m.logger.Warn("ENI slot device plugin registration failed", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			m.Stop()
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *Manager) serveAndRegister(ctx context.Context) error {
	listener, err := m.registrar.listen()
	if err != nil {
		return err
	}

	server := grpc.NewServer()
	m.grpcServer = server
	pluginapi.RegisterDevicePluginServer(server, NewServer(m.config.ResourceName, m.config.MaxSlotsPerNode, m.logger))
	go func() {
		if err := server.Serve(listener); err != nil && ctx.Err() == nil {
			m.logger.Warn("ENI slot device plugin server stopped", zap.Error(err))
		}
	}()

	regCtx, cancel := registrationContext(ctx)
	err = m.registrar.register(regCtx)
	cancel()
	if err != nil {
		server.Stop()
		return err
	}

	m.logger.Info("ENI slot device plugin is running",
		zap.String("resourceName", m.config.ResourceName),
		zap.Int(metric.ENISlotAdvertisedTotalLogField, m.config.MaxSlotsPerNode),
		zap.String(metric.ENISlotDerivedFreeLogField, "computed by Kubernetes scheduler from allocated Pod resource requests"),
	)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			server.Stop()
			return nil
		case <-ticker.C:
			if !m.registrar.socketExists() {
				server.Stop()
				m.logger.Warn("ENI slot device plugin socket disappeared; re-registering")
				return nil
			}
		}
	}
}
