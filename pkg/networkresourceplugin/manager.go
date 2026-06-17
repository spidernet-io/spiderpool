// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type Manager struct {
	config          Config
	providerEnabled bool
	node            *corev1.Node
	nodeGetter      func(context.Context) (*corev1.Node, error)
	discoverer      InterfaceDiscoverer
	logger          *zap.Logger
	servers         []*runningServer
}

type runningServer struct {
	registrar    *Registrar
	grpcServer   *grpc.Server
	deviceServer *Server
	desired      DesiredResource
}

func NewManager(config Config, providerEnabled bool, node *corev1.Node, discoverer InterfaceDiscoverer, logger *zap.Logger) *Manager {
	if discoverer == nil {
		discoverer = NetInterfaceDiscoverer{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{config: config, providerEnabled: providerEnabled, node: node, discoverer: discoverer, logger: logger}
}

func NewManagerWithNodeGetter(config Config, providerEnabled bool, nodeGetter func(context.Context) (*corev1.Node, error), discoverer InterfaceDiscoverer, logger *zap.Logger) *Manager {
	manager := NewManager(config, providerEnabled, nil, discoverer, logger)
	manager.nodeGetter = nodeGetter
	return manager
}

func (m *Manager) Start(ctx context.Context) {
	if !m.config.Enabled {
		m.logger.Info("network resource plugin is disabled")
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	go m.run(ctx)
}

func (m *Manager) Stop() {
	for _, running := range m.servers {
		if running.grpcServer != nil {
			running.grpcServer.Stop()
		}
		if running.registrar != nil {
			if err := running.registrar.cleanupSocket(); err != nil {
				m.logger.Warn("failed to remove network resource plugin socket", zap.Error(err))
			}
		}
	}
	m.servers = nil
}

func (m *Manager) run(ctx context.Context) {
	for {
		if err := m.serveAndRegister(ctx); err != nil && ctx.Err() == nil {
			m.logger.Warn("network resource plugin registration failed", zap.Error(err))
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
	desired, err := m.desiredResources(ctx)
	if err != nil {
		return err
	}
	if len(desired) == 0 {
		m.logger.Info("no network resources selected for advertisement")
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return nil
	}

	for _, resource := range desired {
		devices := healthySubENIDevices(resource.Devices)
		if resource.Interface != "" {
			devices = healthyMasterNICDevices(resource.Interface, resource.Devices)
		}
		running, err := m.startResourceServer(ctx, resource, devices)
		if err != nil {
			m.Stop()
			return err
		}
		m.servers = append(m.servers, running)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return nil
		case <-ticker.C:
			for _, running := range m.servers {
				if !running.registrar.socketExists() {
					m.logger.Warn("network resource plugin socket disappeared; re-registering")
					m.Stop()
					return nil
				}
			}
			desired, err := m.desiredResources(ctx)
			if err != nil {
				m.logger.Warn("failed to reconcile network resource plugin resources; keeping current advertisement", zap.Error(err))
				continue
			}
			if !sameResourceSet(m.servers, desired) {
				m.logger.Info("network resource plugin resource set changed; re-registering")
				m.Stop()
				return nil
			}
			updateServers(m.servers, desired)
		}
	}
}

func (m *Manager) desiredResources(ctx context.Context) ([]DesiredResource, error) {
	node := m.node
	if m.nodeGetter != nil {
		latest, err := m.nodeGetter(ctx)
		if err != nil {
			m.logger.Warn("failed to refresh local Node for network resource plugin; using cached Node metadata", zap.Error(err))
		} else {
			node = latest
			m.node = latest
		}
	}
	interfaces, err := m.discoverer.Interfaces(masterNICRulesUseExplicitIncludes(m.config.ResourceAdvertisement.MasterNIC))
	if err != nil {
		return nil, err
	}
	return ComputeDesiredResources(m.providerEnabled, node, interfaces, m.config)
}

func (m *Manager) startResourceServer(ctx context.Context, resource DesiredResource, devices []*pluginapi.Device) (*runningServer, error) {
	registrar := NewRegistrarWithKubeletRootDir(resource.ResourceName, m.config.KubeletRootDir, m.logger)
	listener, err := registrar.listen()
	if err != nil {
		return nil, err
	}
	grpcServer := grpc.NewServer()
	deviceServer := NewServer(resource.ResourceName, devices, m.logger)
	pluginapi.RegisterDevicePluginServer(grpcServer, deviceServer)
	go func() {
		if err := grpcServer.Serve(listener); err != nil && ctx.Err() == nil {
			m.logger.Warn("network resource plugin server stopped", zap.String("resourceName", resource.ResourceName), zap.Error(err))
		}
	}()

	regCtx, cancel := registrationContext(ctx)
	err = registrar.register(regCtx)
	cancel()
	if err != nil {
		grpcServer.Stop()
		return nil, err
	}

	m.logger.Info("network resource plugin is running",
		zap.String("resourceName", resource.ResourceName),
		zap.Int("devices", len(devices)),
	)
	return &runningServer{registrar: registrar, grpcServer: grpcServer, deviceServer: deviceServer, desired: resource}, nil
}

func sameResourceSet(servers []*runningServer, desired []DesiredResource) bool {
	if len(servers) != len(desired) {
		return false
	}
	desiredByName := desiredResourceByName(desired)
	for _, server := range servers {
		if _, ok := desiredByName[server.desired.ResourceName]; !ok {
			return false
		}
	}
	return true
}

func updateServers(servers []*runningServer, desired []DesiredResource) {
	desiredByName := desiredResourceByName(desired)
	for _, running := range servers {
		next := desiredByName[running.desired.ResourceName]
		if running.desired.Devices == next.Devices && running.desired.Interface == next.Interface {
			continue
		}
		devices := healthySubENIDevices(next.Devices)
		if next.Interface != "" {
			devices = healthyMasterNICDevices(next.Interface, next.Devices)
		}
		running.desired = next
		running.deviceServer.UpdateDevices(devices)
	}
}

func desiredResourceByName(desired []DesiredResource) map[string]DesiredResource {
	result := make(map[string]DesiredResource, len(desired))
	for _, resource := range desired {
		result[resource.ResourceName] = resource
	}
	return result
}
