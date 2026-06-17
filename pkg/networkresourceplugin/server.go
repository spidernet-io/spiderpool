// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

type Server struct {
	pluginapi.UnimplementedDevicePluginServer

	resourceName string
	mutex        lock.RWMutex
	devices      []*pluginapi.Device
	deviceIDs    map[string]struct{}
	updateCh     chan []*pluginapi.Device
	logger       *zap.Logger
}

func NewServer(resourceName string, devices []*pluginapi.Device, logger *zap.Logger) *Server {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Server{
		resourceName: resourceName,
		devices:      devices,
		deviceIDs:    deviceIDSet(devices),
		updateCh:     make(chan []*pluginapi.Device, 1),
		logger:       logger,
	}
}

func (s *Server) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (s *Server) ListAndWatch(_ *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.logger.Info("advertising network resource devices",
		zap.String("resourceName", s.resourceName),
		zap.Int("devices", len(s.snapshotDevices())),
	)
	if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: s.snapshotDevices()}); err != nil {
		return err
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case devices := <-s.updateCh:
			if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: devices}); err != nil {
				return err
			}
		}
	}
}

func (s *Server) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

func (s *Server) Allocate(_ context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	response := &pluginapi.AllocateResponse{ContainerResponses: make([]*pluginapi.ContainerAllocateResponse, 0, len(req.GetContainerRequests()))}
	for _, containerReq := range req.GetContainerRequests() {
		for _, id := range containerReq.GetDevicesIDs() {
			s.mutex.RLock()
			if _, ok := s.deviceIDs[id]; !ok {
				s.mutex.RUnlock()
				return nil, fmt.Errorf("unknown network resource device ID %q for %s", id, s.resourceName)
			}
			s.mutex.RUnlock()
		}
		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{})
	}
	return response, nil
}

func (s *Server) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (s *Server) UpdateDevices(devices []*pluginapi.Device) {
	s.mutex.Lock()
	s.devices = devices
	s.deviceIDs = deviceIDSet(devices)
	s.mutex.Unlock()

	select {
	case s.updateCh <- devices:
	default:
		select {
		case <-s.updateCh:
		default:
		}
		select {
		case s.updateCh <- devices:
		default:
		}
	}
}

func (s *Server) snapshotDevices() []*pluginapi.Device {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return append([]*pluginapi.Device(nil), s.devices...)
}
