// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var logger *zap.Logger

func Execute() {
	logger = logutils.Logger.Named("Spiderpool-Init")

	config := NewInitDefaultConfig()
	client, err := NewCoreClient()
	if err != nil {
		logger.Fatal(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.WaitForEndpointReady(ctx, config.Namespace, config.ControllerName); err != nil {
		logger.Fatal(err.Error())
	}

	if len(config.CoordinatorName) != 0 {
		logger.Sugar().Infof("Try to create default Coordinator %s", config.CoordinatorName)

		coord := &spiderpoolv1.SpiderCoordinator{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.CoordinatorName,
			},
			Spec: spiderpoolv1.CoordinatorSpec{
				Mode:               &config.CoordinatorMode,
				PodCIDRType:        &config.CoordinatorPodCIDRType,
				TunePodRoutes:      &config.CoordinatorTunePodRoutes,
				DetectIPConflict:   &config.CoordinatorDetectIPConflict,
				DetectGateway:      &config.CoordinatorDetectGateway,
				PodDefaultRouteNIC: &config.CoordinatorPodDefaultRouteNic,
				PodMACPrefix:       &config.CoordinatorPodMACPrefix,
				VethLinkAddress:    &config.CoordinatorVethLinkAddress,
				HijackCIDR:         config.CoordinatorHijackCIDR,
			},
		}
		if err := client.WaitForCoordinatorCreated(ctx, coord); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V4SubnetName) != 0 {
		logger.Sugar().Infof("Try to create default IPv4 Subnet %s", config.V4SubnetName)

		subnet := &spiderpoolv1.SpiderSubnet{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V4SubnetName,
			},
			Spec: spiderpoolv1.SubnetSpec{
				IPVersion: ptr.To(constant.IPv4),
				Subnet:    config.V4CIDR,
				IPs:       config.V4IPRanges,
			},
		}
		if len(config.V4Gateway) != 0 {
			subnet.Spec.Gateway = ptr.To(config.V4Gateway)
		}

		if err := client.WaitForSubnetCreated(ctx, subnet); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V6SubnetName) != 0 {
		logger.Sugar().Infof("Try to create default IPv6 Subnet %s", config.V6SubnetName)

		subnet := &spiderpoolv1.SpiderSubnet{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V6SubnetName,
			},
			Spec: spiderpoolv1.SubnetSpec{
				IPVersion: ptr.To(constant.IPv6),
				Subnet:    config.V6CIDR,
				IPs:       config.V6IPRanges,
			},
		}
		if len(config.V6Gateway) != 0 {
			subnet.Spec.Gateway = ptr.To(config.V6Gateway)
		}

		if err := client.WaitForSubnetCreated(ctx, subnet); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V4IPPoolName) != 0 {
		logger.Sugar().Infof("Try to create default IPv4 IPPool %s", config.V4IPPoolName)

		ipPool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V4IPPoolName,
			},
			Spec: spiderpoolv1.IPPoolSpec{
				IPVersion: ptr.To(constant.IPv4),
				Subnet:    config.V4CIDR,
				IPs:       config.V4IPRanges,
				Default:   ptr.To(true),
			},
		}
		if len(config.V4Gateway) != 0 {
			ipPool.Spec.Gateway = ptr.To(config.V4Gateway)
		}

		if err := client.WaitForIPPoolCreated(ctx, ipPool); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V6IPPoolName) != 0 {
		logger.Sugar().Infof("Try to create default IPv6 IPPool %s", config.V6IPPoolName)

		ipPool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V6IPPoolName,
			},
			Spec: spiderpoolv1.IPPoolSpec{
				IPVersion: ptr.To(constant.IPv6),
				Subnet:    config.V6CIDR,
				IPs:       config.V6IPRanges,
				Default:   ptr.To(true),
			},
		}
		if len(config.V6Gateway) != 0 {
			ipPool.Spec.Gateway = ptr.To(config.V6Gateway)
		}

		if err := client.WaitForIPPoolCreated(ctx, ipPool); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if config.enableMultusConfig {
		if err = InitMultusDefaultCR(ctx, &config, client); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if err = makeReadinessReady(&config); err != nil {
		logger.Fatal(err.Error())
	}

	logger.Info("Finish init")
}
