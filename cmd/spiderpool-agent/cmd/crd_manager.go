// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strconv"

	"github.com/go-logr/logr"
	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv2beta1.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
	utilruntime.Must(multusv1.AddToScheme(scheme))
}

func newCRDManager(cfg Config) (ctrl.Manager, error) {
	// set logger for controller-runtime framework
	// The controller-runtime would print debug stack if we do not init the log previously: https://github.com/kubernetes-sigs/controller-runtime/pull/2357
	ctrl.SetLogger(logr.New(controllerruntimelog.NullLogSink{}))

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

	if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &spiderpoolv2beta1.SpiderIPPool{}, constant.SpecDefaultField, func(raw client.Object) []string {
		ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
		if ipPool.Spec.Default != nil {
			return []string{strconv.FormatBool(*ipPool.Spec.Default)}
		}
		return []string{}
	}); err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &spiderpoolv2beta1.SpiderIPPool{}, constant.SpecIPVersionField, func(raw client.Object) []string {
		ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
		if ipPool.Spec.IPVersion != nil {
			return []string{strconv.FormatInt(*ipPool.Spec.IPVersion, 10)}
		}
		return []string{}
	}); err != nil {
		return nil, err
	}

	if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &spiderpoolv2beta1.SpiderReservedIP{}, constant.SpecIPVersionField, func(raw client.Object) []string {
		reservedIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
		if reservedIP.Spec.IPVersion != nil {
			return []string{strconv.FormatInt(*reservedIP.Spec.IPVersion, 10)}
		}
		return []string{}
	}); err != nil {
		return nil, err
	}

	if cfg.DRAConfig.Enabled {
		if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &resourcev1.ResourceSlice{},
			resourcev1.ResourceSliceSelectorNodeName, func(raw client.Object) []string {
				rs := raw.(*resourcev1.ResourceSlice)
				return []string{*rs.Spec.NodeName}
			}); err != nil {
			return nil, err
		}

		if err := mgr.GetFieldIndexer().IndexField(agentContext.InnerCtx, &resourcev1.ResourceSlice{},
			resourcev1.ResourceSliceSelectorDriver, func(raw client.Object) []string {
				rs := raw.(*resourcev1.ResourceSlice)
				return []string{rs.Spec.Driver}
			}); err != nil {
			return nil, err
		}

	}
	return mgr, nil
}
