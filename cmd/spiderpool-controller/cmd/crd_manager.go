// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"path"
	"strconv"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spidernet-io/spiderpool/pkg/webhook"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv1.AddToScheme(scheme))
}

func newCRDManager() (ctrl.Manager, error) {
	port, err := strconv.Atoi(controllerContext.WebhookPort)
	if err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		LeaderElection:                true,
		LeaderElectionResourceLock:    "",    // 若不指定，则默认使用lease锁。还可以指定endpointLock或configmapLock
		LeaderElectionNamespace:       "",    // TODO, 如果不填，则会使用Default the namespace (if running in cluster)
		LeaderElectionID:              "",    // TODO, MUST
		LeaderElectionReleaseOnCancel: false, // 若设置true，则新领导不需要首先等待LeaseDuration时间
		LeaseDuration:                 nil,   // TODO
		RenewDeadline:                 nil,   // TODO
		RetryPeriod:                   nil,   // TODO

		Scheme:                 scheme,
		Port:                   port,
		CertDir:                path.Dir(controllerContext.TlsServerCertPath),
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
