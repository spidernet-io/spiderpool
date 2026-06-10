// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/metric"
)

type Server struct {
	pluginapi.UnimplementedDevicePluginServer

	resourceName string
	devices      []*pluginapi.Device
	deviceIDs    map[string]struct{}
	logger       *zap.Logger
}

func NewServer(resourceName string, maxSlots int, logger *zap.Logger) *Server {
	if resourceName == "" {
		resourceName = defaultResourceName()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	devices := healthyDevices(maxSlots)
	return &Server{
		resourceName: resourceName,
		devices:      devices,
		deviceIDs:    deviceIDSet(devices),
		logger:       logger,
	}
}

func (s *Server) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (s *Server) ListAndWatch(_ *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.logger.Info("advertising ENI slot devices",
		zap.String("resourceName", s.resourceName),
		zap.Int(metric.ENISlotAdvertisedTotalLogField, len(s.devices)),
	)
	if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: s.devices}); err != nil {
		return err
	}

	<-stream.Context().Done()
	return nil
}

func (s *Server) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

func (s *Server) Allocate(_ context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	response := &pluginapi.AllocateResponse{
		ContainerResponses: make([]*pluginapi.ContainerAllocateResponse, 0, len(req.GetContainerRequests())),
	}
	for _, containerReq := range req.GetContainerRequests() {
		for _, id := range containerReq.GetDevicesIDs() {
			if _, ok := s.deviceIDs[id]; !ok {
				return nil, fmt.Errorf("unknown ENI slot device ID %q", id)
			}
		}
		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{})
	}

	s.logger.Debug("allocated ENI slot devices", zap.Int("containers", len(response.ContainerResponses)))
	return response, nil
}

func (s *Server) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func defaultResourceName() string {
	return "spidernet.io/sub-eni"
}
