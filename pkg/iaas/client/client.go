// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

const (
	allocateAPIPath = "/v1/apis/network.iaas.io/ipam/allocate-ips"
	releaseAPIPath  = "/v1/apis/network.iaas.io/ipam/release-ip"
)

// Client is the interface for IaaS provider API client
type Client interface {
	// AllocateIPs calls the IaaS provider to allocate IPs
	AllocateIPs(ctx context.Context, req *AllocateIPRequest) (*AllocateIPResponse, error)
	// ReleaseIPs calls the IaaS provider to release IPs
	ReleaseIP(ctx context.Context, req *ReleaseIPRequest) error
	// GetCachedParentNicMac returns the cached parent NIC MAC for the given key,
	// or empty string if not cached. Key is SpiderMultusConfig namespace/name.
	GetCachedParentNicMac(key string) (string, bool)
	// CacheParentNicMac stores a parent NIC MAC for the given key.
	CacheParentNicMac(key string, mac string)
}

// IaaSClient implements the Client interface
type IaaSClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger

	// parentNicMacCache caches key -> parent NIC MAC address.
	// Keys use SpiderMultusConfig namespace/name.
	parentNicMacCache sync.Map
}

// ValidateConfig validates the IaaS provider configuration.
// Returns nil if the configuration is valid or IaaS integration is disabled (URL is empty).
func ValidateConfig(cfg *spiderpooltypes.IaaSProviderConfig) error {
	if cfg.ServerURL == "" {
		return nil
	}
	u, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid iaasNetworkProvider.serverUrl %q: %w", cfg.ServerURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid iaasNetworkProvider.serverUrl %q: must start with http:// or https://", cfg.ServerURL)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid iaasNetworkProvider.serverUrl %q: host is empty", cfg.ServerURL)
	}
	return nil
}

// NewClient creates a new IaaS client with mTLS configuration
func NewClient(cfg *spiderpooltypes.IaaSProviderConfig, logger *zap.Logger) (*IaaSClient, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("IaaS provider URL is required")
	}
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	// TODO: enable mTLS certificate authentication
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}

	return &IaaSClient{
		baseURL:    cfg.ServerURL,
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

// AllocateIPs calls the IaaS provider to allocate IPs
func (c *IaaSClient) AllocateIPs(ctx context.Context, req *AllocateIPRequest) (*AllocateIPResponse, error) {
	c.logger.Debug("Calling IaaS allocate API",
		zap.String("url", c.baseURL),
		zap.String("nodeName", req.NodeName),
		zap.String("podName", req.PodName),
		zap.String("podNamespace", req.PodNamespace),
	)

	// Marshal request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal allocate request: %w", err)
	}

	// Create HTTP request
	reqURL, err := url.JoinPath(c.baseURL, allocateAPIPath)
	if err != nil {
		return nil, fmt.Errorf("failed to construct allocate URL: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create allocate request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("IaaS allocate API call failed",
			zap.Error(err),
			zap.String("url", reqURL),
		)
		return nil, fmt.Errorf("iaas allocate API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read allocate response: %w", err)
	}

	// Check status code - accept any 2xx success code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Error("IaaS allocate API returned non-success status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("response", string(respBody)),
		)
		return nil, fmt.Errorf("iaas allocate API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Unmarshal response
	var allocateResp AllocateIPResponse
	if err := json.Unmarshal(respBody, &allocateResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal allocate response: %w", err)
	}

	c.logger.Info("IaaS allocate API succeeded",
		zap.String("nodeName", allocateResp.NodeName),
		zap.Int("allocationCount", len(allocateResp.IaaSIPsAllocationResponse)),
	)

	return &allocateResp, nil
}

// ReleaseIP calls the IaaS provider to release an IP.
func (c *IaaSClient) ReleaseIP(ctx context.Context, req *ReleaseIPRequest) error {
	c.logger.Debug("Calling IaaS release API",
		zap.String("url", c.baseURL),
		zap.String("nodeName", req.NodeName),
		zap.String("ipAddress", req.IPAddress),
		zap.String("subnet", req.Subnet),
		zap.String("parentNicMac", req.ParentNicMac),
	)

	reqURL, err := url.JoinPath(c.baseURL, releaseAPIPath)
	if err != nil {
		return fmt.Errorf("failed to construct release URL: %w", err)
	}

	singleReq := &ReleaseIPRequest{
		PodName:      req.PodName,
		PodNamespace: req.PodNamespace,
		PodUID:       req.PodUID,
		NodeName:     req.NodeName,
		IPAddress:    req.IPAddress,
		Subnet:       req.Subnet,
		ParentNicMac: req.ParentNicMac,
	}

	if err := c.releaseSingleIP(ctx, reqURL, singleReq); err != nil {
		return fmt.Errorf("failed to release IP %s: %w", req.IPAddress, err)
	}

	c.logger.Info("IaaS release API succeeded",
		zap.String("nodeName", req.NodeName),
		zap.String("ipAddress", req.IPAddress),
		zap.String("subnet", req.Subnet),
		zap.String("parentNicMac", req.ParentNicMac),
	)

	return nil
}

// releaseSingleIP performs a single IP release API call
func (c *IaaSClient) releaseSingleIP(ctx context.Context, reqURL string, req *ReleaseIPRequest) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal release request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create release request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("IaaS release API call failed",
			zap.Error(err),
			zap.String("url", reqURL),
			zap.String("ipAddresses", req.IPAddress),
		)
		return fmt.Errorf("iaas release API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read release response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Error("IaaS release API returned non-success status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("response", string(respBody)),
			zap.String("ipAddresses", req.IPAddress),
		)
		return fmt.Errorf("iaas release API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetCachedParentNicMac returns the cached parent NIC MAC for the given key, or empty string if not cached.
func (c *IaaSClient) GetCachedParentNicMac(key string) (string, bool) {
	if v, ok := c.parentNicMacCache.Load(key); ok {
		return v.(string), true
	}
	return "", false
}

// CacheParentNicMac stores a parent NIC MAC for the given key.
func (c *IaaSClient) CacheParentNicMac(key string, mac string) {
	c.parentNicMacCache.Store(key, mac)
}

// Close closes the IaaS client
func (c *IaaSClient) Close() error {
	return nil
}
