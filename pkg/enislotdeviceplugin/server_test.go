// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

type listAndWatchStream struct {
	ctx       context.Context
	mu        lock.Mutex
	sendErr   error
	responses []*pluginapi.ListAndWatchResponse
}

func (s *listAndWatchStream) Send(resp *pluginapi.ListAndWatchResponse) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responses = append(s.responses, resp)
	return nil
}

func (s *listAndWatchStream) responseCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.responses)
}

func (s *listAndWatchStream) response(index int) *pluginapi.ListAndWatchResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.responses[index]
}

func (s *listAndWatchStream) SetHeader(metadata.MD) error  { return nil }
func (s *listAndWatchStream) SendHeader(metadata.MD) error { return nil }
func (s *listAndWatchStream) SetTrailer(metadata.MD)       {}
func (s *listAndWatchStream) Context() context.Context     { return s.ctx }
func (s *listAndWatchStream) SendMsg(interface{}) error    { return nil }
func (s *listAndWatchStream) RecvMsg(interface{}) error    { return nil }

var _ = Describe("ENI slot device plugin server", Label("enislotdeviceplugin_server_test"), func() {
	It("sends healthy devices through ListAndWatch", func() {
		ctx, cancel := context.WithCancel(context.Background())
		stream := &listAndWatchStream{ctx: ctx}
		server := NewServer("spidernet.io/sub-eni", 2, nil)

		done := make(chan error, 1)
		go func() {
			done <- server.ListAndWatch(&pluginapi.Empty{}, stream)
		}()

		Eventually(stream.responseCount).Should(Equal(1))
		Expect(stream.response(0).Devices).To(HaveLen(2))
		Expect(stream.response(0).Devices[0].ID).To(Equal("sub-eni-0"))
		cancel()
		Eventually(done).Should(Receive(Succeed()))
	})

	It("sends zero devices when maxSlotsPerNode is zero", func() {
		ctx, cancel := context.WithCancel(context.Background())
		stream := &listAndWatchStream{ctx: ctx}
		server := NewServer("spidernet.io/sub-eni", 0, nil)

		done := make(chan error, 1)
		go func() {
			done <- server.ListAndWatch(&pluginapi.Empty{}, stream)
		}()

		Eventually(stream.responseCount).Should(Equal(1))
		Expect(stream.response(0).Devices).To(BeEmpty())
		cancel()
		Eventually(done).Should(Receive(Succeed()))
	})

	It("returns stream send errors from ListAndWatch", func() {
		expectedErr := errors.New("send failed")
		stream := &listAndWatchStream{
			ctx:     context.Background(),
			sendErr: expectedErr,
		}
		server := NewServer("spidernet.io/sub-eni", 1, nil)

		err := server.ListAndWatch(&pluginapi.Empty{}, stream)

		Expect(err).To(MatchError(expectedErr))
		Expect(stream.responseCount()).To(Equal(0))
	})

	It("handles Allocate idempotently for known slot IDs", func() {
		server := NewServer("spidernet.io/sub-eni", 2, nil)
		req := &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{
				{DevicesIDs: []string{"sub-eni-0"}},
				{DevicesIDs: []string{"sub-eni-0", "sub-eni-1"}},
			},
		}

		resp, err := server.Allocate(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(HaveLen(2))
	})

	It("rejects unknown slot IDs", func() {
		server := NewServer("spidernet.io/sub-eni", 1, nil)

		_, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{
				{DevicesIDs: []string{"sub-eni-2"}},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown ENI slot device ID"))
	})

	It("uses the default resource name when none is given", func() {
		server := NewServer("", 1, nil)

		Expect(server.resourceName).To(Equal(defaultResourceName()))
	})

	It("defaults a nil logger and initializes device IDs", func() {
		server := NewServer("spidernet.io/sub-eni", 2, nil)

		Expect(server.logger).NotTo(BeNil())
		Expect(server.devices).To(HaveLen(2))
		Expect(server.deviceIDs).To(HaveKey("sub-eni-0"))
		Expect(server.deviceIDs).To(HaveKey("sub-eni-1"))
	})

	It("initializes no devices for negative maxSlotsPerNode", func() {
		server := NewServer("spidernet.io/sub-eni", -1, nil)

		Expect(server.devices).To(BeNil())
		Expect(server.deviceIDs).To(BeEmpty())
	})

	It("GetDevicePluginOptions returns an empty options object", func() {
		server := NewServer("spidernet.io/sub-eni", 1, nil)

		resp, err := server.GetDevicePluginOptions(context.Background(), &pluginapi.Empty{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
	})

	It("GetPreferredAllocation returns an empty response", func() {
		server := NewServer("spidernet.io/sub-eni", 1, nil)

		resp, err := server.GetPreferredAllocation(context.Background(), &pluginapi.PreferredAllocationRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
	})

	It("PreStartContainer returns an empty response", func() {
		server := NewServer("spidernet.io/sub-eni", 1, nil)

		resp, err := server.PreStartContainer(context.Background(), &pluginapi.PreStartContainerRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())
	})

	It("Allocate succeeds with an empty container request list", func() {
		server := NewServer("spidernet.io/sub-eni", 2, nil)

		resp, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(BeEmpty())
	})

	It("Allocate succeeds with a nil request", func() {
		server := NewServer("spidernet.io/sub-eni", 2, nil)

		resp, err := server.Allocate(context.Background(), nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(BeEmpty())
	})

	It("Allocate accepts containers that request no devices", func() {
		server := NewServer("spidernet.io/sub-eni", 2, nil)

		resp, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{
				nil,
				{},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(HaveLen(2))
	})
})
