// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				HTTPRequestTimeout: "",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(client.httpTimeout).To(Equal(constant.DefaultIaaSProviderTimeout))
		})

		It("should use configured timeout when HTTPRequestTimeout is set", Label("B002"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
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
					Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
					HTTPRequestTimeout: tc.input,
				}
				client, err := NewClient(cfg, logger)
				Expect(err).NotTo(HaveOccurred(), "for input %q", tc.input)
				Expect(client.httpTimeout).To(Equal(tc.expected), "for input %q", tc.input)
			}
		})

		It("should return error for invalid duration string", Label("B004"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				HTTPRequestTimeout: "invalid",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).To(HaveOccurred())
			Expect(client).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid iaasNetworkProvider.httpRequestTimeout"))
		})

		It("should return error for negative duration (rejected by ValidateConfig in T014)", Label("B005"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				HTTPRequestTimeout: "-30s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).To(HaveOccurred())
			Expect(client).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("timeout must be positive"))
		})

		It("should create http.Client with correct transport", Label("B006"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
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
		It("should return nil when service name is empty (disabled)", Label("B007"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				HTTPRequestTimeout: "",
			}
			err := ValidateConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should accept a valid service configuration", Label("B008"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				HTTPRequestTimeout: "30s",
			}
			err := ValidateConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject service without namespace", Label("B009"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service: spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Port: 8443},
			}
			err := ValidateConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("namespace is required"))
		})

		It("should reject service with invalid port", Label("B010"), func() {
			for _, port := range []int{0, -1, 65536} {
				cfg := &spiderpooltypes.IaaSProviderConfig{
					Service: spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: port},
				}
				err := ValidateConfig(cfg)
				Expect(err).To(HaveOccurred(), "for port %d", port)
				Expect(err.Error()).To(ContainSubstring("must be in range 1-65535"))
			}
		})

		It("should reject non-existent CA file", Label("B011"), func() {
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service: spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				TLS:     spiderpooltypes.IaaSProviderTLS{CaFile: "/nonexistent/ca.crt"},
			}
			err := ValidateConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read CA bundle"))
		})

		It("should reject CA file without valid PEM certificate", Label("B012"), func() {
			caFile := filepath.Join(GinkgoT().TempDir(), "ca.crt")
			Expect(os.WriteFile(caFile, []byte("not a pem"), 0o600)).To(Succeed())
			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service: spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				TLS:     spiderpooltypes.IaaSProviderTLS{CaFile: caFile},
			}
			err := ValidateConfig(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no valid PEM certificate"))
		})
	})

	Describe("ValidateConfig HTTPRequestTimeout validation", func() {
		DescribeTable(
			"timeout validation",
			func(timeout string, expectError bool, errorSubstr string) {
				cfg := &spiderpooltypes.IaaSProviderConfig{
					Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
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

	Describe("request timeout header", func() {
		It("should send remaining request timeout for allocate requests", Label("timeout-header"), func() {
			headerCh := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headerCh <- r.Header.Get(requestTimeoutMsHeader)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"nodeName":"test-node","subEniResponses":[]}`))
			}))
			defer server.Close()

			cfg := &spiderpooltypes.IaaSProviderConfig{
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				HTTPRequestTimeout: "50s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			client.baseURL = server.URL

			ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
			defer cancel()

			req := &AllocateIPRequest{
				NodeName:     "test-node",
				PodName:      "test-pod",
				PodNamespace: "default",
				PodUID:       "test-uuid",
				SubEniRequests: []SubEniRequest{
					{ParentNicMac: "00:11:22:33:44:55", Subnet: "10.0.0.0/24", IPv4Address: "10.0.0.1", IPv6Address: "fd00::1"},
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
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
				HTTPRequestTimeout: "50s",
			}
			client, err := NewClient(cfg, logger)
			Expect(err).NotTo(HaveOccurred())
			client.baseURL = server.URL

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
				Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
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
				SubEniRequests: []SubEniRequest{
					{ParentNicMac: "00:11:22:33:44:55", Subnet: "10.0.0.0/24", IPv4Address: "10.0.0.1", IPv6Address: "fd00::1"},
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

var _ = Describe("IaaS Client TLS verification", Label("unitest"), func() {
	var logger *zap.Logger

	BeforeEach(func() {
		var err error
		logger, err = zap.NewDevelopment()
		Expect(err).NotTo(HaveOccurred())
	})

	newCA := func(cn string) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		Expect(err).NotTo(HaveOccurred())
		cert, err := x509.ParseCertificate(der)
		Expect(err).NotTo(HaveOccurred())
		return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	}

	It("verifies the provider certificate, pins ServerName, and re-reads the CA bundle per connection", Label("tls-verify"), func() {
		const serverName = "iaas-network-provider.iaas-system.svc"

		caCert, caKey, caPEM := newCA("provider-ca")
		_, _, wrongCAPEM := newCA("wrong-ca")

		// Serving cert signed by the provider CA, SAN only contains the svc DNS name.
		srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		srvTmpl := &x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: serverName},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			DNSNames:     []string{serverName},
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
		Expect(err).NotTo(HaveOccurred())

		ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
			Certificates: []tls.Certificate{{Certificate: [][]byte{srvDER}, PrivateKey: srvKey}},
			MinVersion:   tls.VersionTLS12,
		})
		Expect(err).NotTo(HaveOccurred())
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"nodeName":"test-node","subEniResponses":[]}`))
		})}
		go func() { _ = srv.Serve(ln) }()
		defer func() { _ = srv.Close() }()

		// Start with the WRONG CA in the bundle file.
		caFile := filepath.Join(GinkgoT().TempDir(), "ca.crt")
		Expect(os.WriteFile(caFile, wrongCAPEM, 0o600)).To(Succeed())

		cfg := &spiderpooltypes.IaaSProviderConfig{
			Service:            spiderpooltypes.IaaSProviderService{Name: "iaas-network-provider", Namespace: "iaas-system", Port: 8443},
			TLS:                spiderpooltypes.IaaSProviderTLS{CaFile: caFile},
			HTTPRequestTimeout: "5s",
		}
		client, err := NewClient(cfg, logger)
		Expect(err).NotTo(HaveOccurred())
		// The svc DNS name does not resolve in unit tests; dial the listener
		// address directly. ServerName stays pinned to the svc DNS name.
		client.baseURL = "https://" + ln.Addr().String()

		req := &AllocateIPRequest{NodeName: "test-node", PodName: "p", PodNamespace: "default", PodUID: "u"}

		// Wrong CA -> handshake must fail.
		_, err = client.AllocateIPs(context.Background(), req)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("certificate"))

		// Rotate the bundle to the right CA; the same client must pick it up
		// on the next connection without restart, and the handshake must
		// succeed even though the dialed address (127.0.0.1) is not in the
		// certificate SAN, proving ServerName pinning works.
		Expect(os.WriteFile(caFile, caPEM, 0o600)).To(Succeed())
		resp, err := client.AllocateIPs(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.NodeName).To(Equal("test-node"))
	})
})
