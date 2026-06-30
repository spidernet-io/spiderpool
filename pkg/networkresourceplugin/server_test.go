// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

type fakeListAndWatchServer struct {
	pluginapi.DevicePlugin_ListAndWatchServer

	ctx       context.Context
	mutex     lock.Mutex
	responses []*pluginapi.ListAndWatchResponse
	err       error
}

func (f *fakeListAndWatchServer) Send(response *pluginapi.ListAndWatchResponse) error {
	if f.err != nil {
		return f.err
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.responses = append(f.responses, response)
	return nil
}

func (f *fakeListAndWatchServer) Responses() []*pluginapi.ListAndWatchResponse {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return append([]*pluginapi.ListAndWatchResponse(nil), f.responses...)
}

func (f *fakeListAndWatchServer) Context() context.Context {
	return f.ctx
}

func (f *fakeListAndWatchServer) SetHeader(metadata.MD) error {
	return nil
}

func (f *fakeListAndWatchServer) SendHeader(metadata.MD) error {
	return nil
}

func (f *fakeListAndWatchServer) SetTrailer(metadata.MD) {
}

func (f *fakeListAndWatchServer) SendMsg(interface{}) error {
	return nil
}

func (f *fakeListAndWatchServer) RecvMsg(interface{}) error {
	return nil
}

var _ = Describe("network resource server", Label("networkresourceplugin_server_test"), func() {
	It("returns empty device plugin options", func() {
		server := NewServer("spidernet.io/sub-eni", nil, nil)

		options, err := server.GetDevicePluginOptions(context.Background(), &pluginapi.Empty{})

		Expect(err).NotTo(HaveOccurred())
		Expect(options).To(Equal(&pluginapi.DevicePluginOptions{}))
	})

	It("accepts allocation requests for known devices", func() {
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		resp, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIDs: []string{"sub-eni-0"}}},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(HaveLen(1))
	})

	It("accepts empty allocation requests", func() {
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		resp, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(BeEmpty())
	})

	It("rejects allocation requests for unknown devices", func() {
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		_, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIDs: []string{"sub-eni-99"}}},
		})

		Expect(err).To(HaveOccurred())
	})

	It("updates allocatable devices", func() {
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)
		server.UpdateDevices(healthySubENIDevices(2))

		_, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIDs: []string{"sub-eni-1"}}},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns preferred allocation and pre-start responses", func() {
		server := NewServer("spidernet.io/sub-eni", nil, nil)

		preferred, err := server.GetPreferredAllocation(context.Background(), &pluginapi.PreferredAllocationRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(preferred).To(Equal(&pluginapi.PreferredAllocationResponse{}))

		preStart, err := server.PreStartContainer(context.Background(), &pluginapi.PreStartContainerRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(preStart).To(Equal(&pluginapi.PreStartContainerResponse{}))
	})

	It("returns snapshot copies", func() {
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		snapshot := server.snapshotDevices()
		snapshot[0] = &pluginapi.Device{ID: "changed", Health: pluginapi.Unhealthy}

		Expect(server.snapshotDevices()).To(Equal([]*pluginapi.Device{{ID: "sub-eni-0", Health: pluginapi.Healthy}}))
	})

	It("coalesces pending device updates", func() {
		server := NewServer("spidernet.io/sub-eni", nil, nil)

		server.UpdateDevices(healthySubENIDevices(1))
		server.UpdateDevices(healthySubENIDevices(2))

		Eventually(server.updateCh).Should(Receive(Equal(healthySubENIDevices(2))))
		Consistently(server.updateCh).ShouldNot(Receive())
	})

	It("sends initial ListAndWatch devices and exits when context is canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		stream := &fakeListAndWatchServer{ctx: ctx}
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		done := make(chan error, 1)
		go func() {
			done <- server.ListAndWatch(&pluginapi.Empty{}, stream)
		}()

		Eventually(func() []*pluginapi.ListAndWatchResponse {
			return stream.Responses()
		}).Should(HaveLen(1))
		cancel()
		Eventually(done).Should(Receive(Succeed()))
		Expect(stream.Responses()[0].Devices).To(Equal(healthySubENIDevices(1)))
	})

	It("sends ListAndWatch device updates", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream := &fakeListAndWatchServer{ctx: ctx}
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		done := make(chan error, 1)
		go func() {
			done <- server.ListAndWatch(&pluginapi.Empty{}, stream)
		}()

		Eventually(func() []*pluginapi.ListAndWatchResponse {
			return stream.Responses()
		}).Should(HaveLen(1))

		server.UpdateDevices(healthySubENIDevices(2))
		Eventually(func() []*pluginapi.ListAndWatchResponse {
			return stream.Responses()
		}).Should(HaveLen(2))
		Expect(stream.Responses()[1].Devices).To(Equal(healthySubENIDevices(2)))

		cancel()
		Eventually(done).Should(Receive(Succeed()))
	})

	It("returns ListAndWatch send errors", func() {
		server := NewServer("spidernet.io/sub-eni", healthySubENIDevices(1), nil)

		err := server.ListAndWatch(&pluginapi.Empty{}, &fakeListAndWatchServer{
			ctx: context.Background(),
			err: errors.New("send failed"),
		})

		Expect(err).To(MatchError("send failed"))
	})
})
