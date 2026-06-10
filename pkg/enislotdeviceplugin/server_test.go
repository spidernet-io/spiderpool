// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package enislotdeviceplugin

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/metadata"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

type listAndWatchStream struct {
	ctx       context.Context
	mu        lock.Mutex
	responses []*pluginapi.ListAndWatchResponse
}

func (s *listAndWatchStream) Send(resp *pluginapi.ListAndWatchResponse) error {
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
		server := NewServer("spidernet.io/eni-slot", 2, nil)

		done := make(chan error, 1)
		go func() {
			done <- server.ListAndWatch(&pluginapi.Empty{}, stream)
		}()

		Eventually(stream.responseCount).Should(Equal(1))
		Expect(stream.response(0).Devices).To(HaveLen(2))
		Expect(stream.response(0).Devices[0].ID).To(Equal("eni-slot-0"))
		cancel()
		Eventually(done).Should(Receive(Succeed()))
	})

	It("sends zero devices when maxSlotsPerNode is zero", func() {
		ctx, cancel := context.WithCancel(context.Background())
		stream := &listAndWatchStream{ctx: ctx}
		server := NewServer("spidernet.io/eni-slot", 0, nil)

		done := make(chan error, 1)
		go func() {
			done <- server.ListAndWatch(&pluginapi.Empty{}, stream)
		}()

		Eventually(stream.responseCount).Should(Equal(1))
		Expect(stream.response(0).Devices).To(BeEmpty())
		cancel()
		Eventually(done).Should(Receive(Succeed()))
	})

	It("handles Allocate idempotently for known slot IDs", func() {
		server := NewServer("spidernet.io/eni-slot", 2, nil)
		req := &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{
				{DevicesIDs: []string{"eni-slot-0"}},
				{DevicesIDs: []string{"eni-slot-0", "eni-slot-1"}},
			},
		}

		resp, err := server.Allocate(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.ContainerResponses).To(HaveLen(2))
	})

	It("rejects unknown slot IDs", func() {
		server := NewServer("spidernet.io/eni-slot", 1, nil)

		_, err := server.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{
				{DevicesIDs: []string{"eni-slot-2"}},
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown ENI slot device ID"))
	})
})
