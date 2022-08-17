// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv1.AddToScheme(scheme))
}

func newCRDManager() (ctrl.Manager, error) {
	port, err := strconv.Atoi(controllerContext.Cfg.WebhookPort)
	if err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Port:                   port,
		CertDir:                path.Dir(controllerContext.Cfg.TlsServerCertPath),
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
		ClientDisableCacheFor: []client.Object{
			&corev1.Node{},
			&corev1.Namespace{},
			&corev1.Pod{},
			&appsv1.Deployment{},
			&appsv1.StatefulSet{},
			&spiderpoolv1.SpiderIPPool{},
			&spiderpoolv1.SpiderEndpoint{},
			&spiderpoolv1.SpiderReservedIP{}},
	})
	if nil != err {
		return nil, err
	}

	// register a http handler for webhook health check
	mgr.GetWebhookServer().Register(webhookMutateRoute, &_webhookHealthyCheck{})

	return mgr, nil
}

const webhookMutateRoute = "/webhook-health-check"

type _webhookHealthyCheck struct{}

// ServeHTTP only serves for SpiderIPPool webhook health check, it will return http status code 200 for GET request
func (*_webhookHealthyCheck) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		logger.Debug("SpiderIPPool webhook health check ready")
		writer.WriteHeader(http.StatusOK)
	}
}

// WebhookHealthyCheck servers for spiderpool controller readiness and liveness probe.
// This is a Layer7 check.
func WebhookHealthyCheck(webhookPort string) error {
	webhookMutateURL := fmt.Sprintf("https://localhost:%s%s", webhookPort, webhookMutateRoute)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, webhookMutateURL, nil)
	if nil != err {
		return fmt.Errorf("failed to new webhook https request, error: %v", err)
	}

	resp, err := httpClient.Do(req)
	if nil != err {
		return fmt.Errorf("webhook server is not reachable: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook health check status code: %d", resp.StatusCode)
	}

	_ = resp.Body.Close()

	return nil
}
