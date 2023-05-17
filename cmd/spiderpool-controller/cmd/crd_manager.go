// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"path"
	"strconv"

	calicov1 "github.com/tigera/operator/pkg/apis/crd.projectcalico.org/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv2beta1.AddToScheme(scheme))
	utilruntime.Must(calicov1.AddToScheme(scheme))
}

func newCRDManager() (ctrl.Manager, error) {
	port, err := strconv.Atoi(controllerContext.Cfg.WebhookPort)
	if err != nil {
		return nil, err
	}

	config := ctrl.GetConfigOrDie()
	config.Burst = 200
	config.QPS = 100

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Port:                   port,
		CertDir:                path.Dir(controllerContext.Cfg.TlsServerCertPath),
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(controllerContext.InnerCtx, &spiderpoolv2beta1.SpiderIPPool{}, "spec.default", func(raw client.Object) []string {
		ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
		return []string{strconv.FormatBool(*ipPool.Spec.Default)}
	}); err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(controllerContext.InnerCtx, &spiderpoolv2beta1.SpiderReservedIP{}, "spec.ipVersion", func(raw client.Object) []string {
		reservedIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
		return []string{strconv.FormatInt(*reservedIP.Spec.IPVersion, 10)}
	}); err != nil {
		return nil, err
	}

	// register a http handler for webhook health check
	mgr.GetWebhookServer().Register(webhookMutateRoute, &_webhookHealthCheck{})

	return mgr, nil
}

const webhookMutateRoute = "/webhook-health-check"

type _webhookHealthCheck struct{}

// ServeHTTP only serves for SpiderIPPool webhook health check, it will return http status code 200 for GET request
func (*_webhookHealthCheck) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		//logger.Debug("SpiderIPPool webhook health check ready")
		writer.WriteHeader(http.StatusOK)
	}
}

// WebhookHealthyCheck servers for spiderpool controller readiness and liveness probe.
// This is a Layer7 check.
func WebhookHealthyCheck(httpClient *http.Client, webhookPort string) error {
	webhookMutateURL := fmt.Sprintf("https://localhost:%s%s", webhookPort, webhookMutateRoute)

	req, err := http.NewRequest(http.MethodGet, webhookMutateURL, nil)
	if nil != err {
		return fmt.Errorf("failed to new webhook https request, error: %v", err)
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

// newWebhookHealthCheckClient creates one http client which serves for webhook health check
func newWebhookHealthCheckClient() *http.Client {
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
