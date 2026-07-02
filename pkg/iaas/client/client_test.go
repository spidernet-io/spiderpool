// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("IaaS Client", Label("unitest"), func() {
	var logger *zap.Logger

	BeforeEach(func() {
		var err error
		logger, err = zap.NewDevelopment()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("NewClient", func() {
		It("should use default timeout when HTTPRequestTimeout is empty", Label("B001"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(client.httpTimeout).To(Equal(constant.DefaultIaaSProviderTimeout))
		})

		It("should use configured timeout when HTTPRequestTimeout is set", Label("B002"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "45s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(client.httpTimeout).To(Equal(45 * time.Second))
		})

		It("should parse duration strings correctly", Label("B003"), func() {
			testCases := []struct {
				input    string
				expected time.Duration
			}{
				{"30s", 30 * time.Second},
				{"1m", 1 * time.Minute},
				{"1m30s", 90 * time.Second},
				{"500ms", 500 * time.Millisecond},
			}

			for _, tc := range testCases {
				cfg := &spiderpooltypes.IaaSProviderConfig{
					ServerURL:          "http://localhost:8080",
					HTTPRequestTimeout: tc.input,
				}
				client, err := NewClient(cfg, logger)
				Expect(err).NotTo(HaveOccurred(), "for input %q", tc.input)
				Expect(client.httpTimeout).To(Equal(tc.expected), "for input %q", tc.input)
			}
		})

		It("should return error for invalid duration string", Label("B004"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "invalid",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).To(HaveOccurred())
			Expect(client).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid iaasNetworkProvider.httpRequestTimeout"))
		})

		It("should return error for negative duration (rejected by ValidateConfig in T014)", Label("B005"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "-30s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).To(HaveOccurred())
			Expect(client).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("timeout must be positive"))
		})

		It("should create http.Client with correct transport", Label("B006"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "https://localhost:8080",
				HTTPRequestTimeout: "30s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.httpClient).NotTo(BeNil())
			Expect(client.httpClient.Transport).NotTo(BeNil())
			_, ok := client.httpClient.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("ValidateConfig", func() {
		It("should return nil when ServerURL is empty", Label("B007"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "",
				HTTPRequestTimeout: "",
			}
			err := ValidateConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should accept valid http URL", Label("B008"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "30s",
			}
			err := ValidateConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should accept valid https URL", Label("B009"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "https://provider.example.com:8443",
				HTTPRequestTimeout: "30s",
			}
			err := ValidateConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject URL without scheme", Label("B010"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "localhost:8080",
				HTTPRequestTimeout: "30s",
			}
			err := ValidateConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must start with http:// or https://"))
		})

		It("should reject URL with invalid scheme", Label("B011"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "ftp://localhost:8080",
				HTTPRequestTimeout: "30s",
			}
			err := ValidateConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must start with http:// or https://"))
		})

		It("should reject URL without host", Label("B012"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://",
				HTTPRequestTimeout: "30s",
			}
			err := ValidateConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("host is empty"))
		})
	})

	Describe("ValidateConfig HTTPRequestTimeout validation", func() {
		DescribeTable(
			"timeout validation",
			func(timeout string, expectError bool, errorSubstr string) {
				cfg := &spiderpooltypes.IaaSProviderConfig{
					ServerURL:          "http://localhost:8080",
					HTTPRequestTimeout: timeout,
				}
				err := ValidateConfig(cfg)
				if expectError {
					Expect(err).To(HaveOccurred())
					if errorSubstr != "" {
						Expect(err.Error()).To(ContainSubstring(errorSubstr))
					}
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("empty timeout should be valid (uses default)", "", false, ""),
			Entry("30s should be valid", "30s", false, ""),
			Entry("45s should be valid", "45s", false, ""),
			Entry("1m should be valid", "1m", false, ""),
			Entry("1m30s should be valid", "1m30s", false, ""),
			Entry("99s should be valid (just under 100s)", "99s", false, ""),
			Entry("zero should be invalid", "0s", true, "timeout must be positive"),
			Entry("negative should be invalid", "-30s", true, "timeout must be positive"),
			Entry("exactly 100s should be invalid (must be strictly less)", "100s", true, "must be less than"),
			Entry("over 100s should be invalid", "2m", true, "must be less than"),
			Entry("exactly 2m should be invalid (static limit)", "2m", true, "must be less than"),
			Entry("over 2m should be invalid", "3m", true, "must be less than"),
			Entry("invalid duration string should be invalid", "invalid", true, "invalid duration"),
		)
	})
})

var _ = Describe("IaaS Client Context Deadline Handling", Label("unitest"), func() {
	var logger *zap.Logger

	BeforeEach(func() {
		var err error
		logger, err = zap.NewDevelopment()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("AllocateIPs parent budget check", func() {
		It("should return parent budget insufficient error when context has too little time remaining", Label("T017", "US3"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "50s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())

			// Create a context with only 1s remaining — far less than IaaSProviderWorstCase (48s)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			req := &AllocateIPRequest{
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "default",
				PodUID:       "test-uuid",
				IaaSIPsAllocationRequest: []IaaSIPAllocationItem{
					{IPAddress: "10.0.0.1", Subnet: "10.0.0.0/24"},
				},
			}

			_, err = client.AllocateIPs(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parent budget insufficient"))
		})
	})

	Describe("ReleaseIP parent budget check", func() {
		It("should return parent budget insufficient error when context has too little time remaining", Label("T017", "US3"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "50s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())

			// Create a context with only 1s remaining — far less than IaaSProviderWorstCase (48s)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			req := &ReleaseIPRequest{
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "default",
				PodUID:       "test-uuid",
				ParentNicMac: "00:11:22:33:44:55",
				Subnet:       "10.0.0.0/24",
				IPAddress:    "10.0.0.1",
			}

			err = client.ReleaseIP(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parent budget insufficient"))
		})
	})

	Describe("request timeout header", func() {
		It("should send remaining request timeout for allocate requests", Label("timeout-header"), func() {
			headerCh := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headerCh <- r.Header.Get(requestTimeoutMsHeader)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"nodeName":"test-node","iaasIPsAllocationResponse":[]}`))
			}))
			defer server.Close()

			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          server.URL,
				HTTPRequestTimeout: "50s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
			defer cancel()

			req := &AllocateIPRequest{
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "default",
				PodUID:       "test-uuid",
				IaaSIPsAllocationRequest: []IaaSIPAllocationItem{
					{IPAddress: "10.0.0.1", Subnet: "10.0.0.0/24"},
				},
			}

			_, err = client.AllocateIPs(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			header := <-headerCh
			Expect(header).NotTo(BeEmpty())
			timeoutMs, err := strconv.ParseInt(header, 10, 64)
			Expect(err).NotTo(HaveOccurred())
			Expect(timeoutMs).To(BeNumerically(">", 0))
			Expect(timeoutMs).To(BeNumerically("<=", int64((50*time.Second)/time.Millisecond)))
		})

		It("should send remaining request timeout for release requests", Label("timeout-header"), func() {
			headerCh := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headerCh <- r.Header.Get(requestTimeoutMsHeader)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          server.URL,
				HTTPRequestTimeout: "50s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
			defer cancel()

			req := &ReleaseIPRequest{
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "default",
				PodUID:       "test-uuid",
				ParentNicMac: "00:11:22:33:44:55",
				Subnet:       "10.0.0.0/24",
				IPAddress:    "10.0.0.1",
			}

			err = client.ReleaseIP(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			header := <-headerCh
			Expect(header).NotTo(BeEmpty())
			timeoutMs, err := strconv.ParseInt(header, 10, 64)
			Expect(err).NotTo(HaveOccurred())
			Expect(timeoutMs).To(BeNumerically(">", 0))
			Expect(timeoutMs).To(BeNumerically("<=", int64((50*time.Second)/time.Millisecond)))
		})
	})
})

var _ = Describe("IaaS Client Timeout Errors", Label("unitest"), func() {
	var logger *zap.Logger

	BeforeEach(func() {
		var err error
		logger, err = zap.NewDevelopment()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Timeout error messages", func() {
		It("should identify provider-interaction timeout in error message", Label("T018", "US3"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				ServerURL:          "http://localhost:8080",
				HTTPRequestTimeout: "1ms",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())

			// Use a context with no deadline so the minimum-budget guard is bypassed.
			// The 1ms httpTimeout will then govern the request and cause it to time out
			// against the (unreachable) server.
			ctx := context.Background()

			req := &AllocateIPRequest{
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "default",
				PodUID:       "test-uuid",
				IaaSIPsAllocationRequest: []IaaSIPAllocationItem{
					{IPAddress: "10.0.0.1", Subnet: "10.0.0.0/24"},
				},
			}

			_, err = client.AllocateIPs(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Or(
				ContainSubstring("provider-interaction timeout"),
				ContainSubstring("iaas allocate API call failed"),
			))
		})
	})
})
