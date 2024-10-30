// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"net/http"
	"path"
	"strconv"

	"github.com/go-logr/logr"
	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	calicov1 "github.com/tigera/operator/pkg/apis/crd.projectcalico.org/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	runtimeWebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv2beta1.AddToScheme(scheme))
	utilruntime.Must(calicov1.AddToScheme(scheme))
	utilruntime.Must(multusv1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(scheme))
}

func newCRDManager() (ctrl.Manager, error) {
	// set logger for controller-runtime framework
	// The controller-runtime would print debug stack if we do not init the log previously: https://github.com/kubernetes-sigs/controller-runtime/pull/2357
	ctrl.SetLogger(logr.New(controllerruntimelog.NullLogSink{}))

	port, err := strconv.Atoi(controllerContext.Cfg.WebhookPort)
	if err != nil {
		return nil, err
	}

	config := ctrl.GetConfigOrDie()
	config.Burst = 200
	config.QPS = 100
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
		WebhookServer: runtimeWebhook.NewServer(runtimeWebhook.Options{
			Port:    port,
			CertDir: path.Dir(controllerContext.Cfg.TlsServerCertPath),
		}),
	})
	if err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(controllerContext.InnerCtx, &spiderpoolv2beta1.SpiderIPPool{}, constant.SpecDefaultField, func(raw client.Object) []string {
		ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
		return []string{strconv.FormatBool(*ipPool.Spec.Default)}
	}); err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(controllerContext.InnerCtx, &spiderpoolv2beta1.SpiderReservedIP{}, constant.SpecIPVersionField, func(raw client.Object) []string {
		reservedIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
		return []string{strconv.FormatInt(*reservedIP.Spec.IPVersion, 10)}
	}); err != nil {
		return nil, err
	}

	// register a http handler for webhook health check
	mgr.GetWebhookServer().Register(constant.WebhookMutateRoute, &_webhookHealthCheck{})

	return mgr, nil
}

type _webhookHealthCheck struct{}

// ServeHTTP only serves for SpiderIPPool webhook health check, it will return http status code 200 for GET request
func (*_webhookHealthCheck) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		//logger.Debug("SpiderIPPool webhook health check ready")
		writer.WriteHeader(http.StatusOK)
	}
}
