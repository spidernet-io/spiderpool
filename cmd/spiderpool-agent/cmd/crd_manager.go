// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv2beta1.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
}

func newCRDManager() (ctrl.Manager, error) {
	config := ctrl.GetConfigOrDie()
	config.Burst = 100
	config.QPS = 50

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &spiderpoolv2beta1.SpiderIPPool{}, "spec.default", func(raw client.Object) []string {
		ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
		return []string{strconv.FormatBool(*ipPool.Spec.Default)}
	}); err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &spiderpoolv2beta1.SpiderReservedIP{}, "spec.ipVersion", func(raw client.Object) []string {
		reservedIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
		return []string{strconv.FormatInt(*reservedIP.Spec.IPVersion, 10)}
	}); err != nil {
		return nil, err
	}

	return mgr, nil
}
