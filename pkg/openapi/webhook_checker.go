// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

// NewWebhookHealthCheckClient creates one http client which serves for webhook health check
func NewWebhookHealthCheckClient() *http.Client {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DisableKeepAlives: true,
		},
	}

	return httpClient
}

// WebhookHealthyCheck servers for spiderpool controller readiness and liveness probe.
// This is a Layer7 check.
func WebhookHealthyCheck(httpClient *http.Client, webhookPort string, url *string) error {
	var webhookMutateURL string
	if url != nil {
		webhookMutateURL = fmt.Sprintf("https://%s:%s%s", *url, webhookPort, constant.WebhookMutateRoute)
	} else {
		webhookMutateURL = fmt.Sprintf("https://localhost:%s%s", webhookPort, constant.WebhookMutateRoute)
	}

	req, err := http.NewRequest(http.MethodGet, webhookMutateURL, nil)
	if nil != err {
		return fmt.Errorf("failed to new webhook https request, error: %w", err)
	}

	resp, err := httpClient.Do(req)
	if nil != err {
		return fmt.Errorf("webhook server is not reachable: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook health check status code: %d", resp.StatusCode)
	}

	return nil
}
