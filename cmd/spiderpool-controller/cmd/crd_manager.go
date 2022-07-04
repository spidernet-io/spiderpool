// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"path"
	"strconv"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/webhook"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
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
	})
	if nil != err {
		return nil, err
	}

	if err = (&webhook.IPPoolWebhook{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWebhookWithManager(mgr); err != nil {
		return nil, err
	}

	return mgr, nil
}
