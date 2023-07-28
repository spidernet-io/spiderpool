// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	if err := client.WaitForEndpointReady(ctx, config.Namespace, config.ControllerName); err != nil {
		logger.Fatal(err.Error())
	}

	if len(config.CoordinatorName) != 0 {
		logger.Sugar().Infof("Try to create default Coordinator %s", config.CoordinatorName)

		coord := &spiderpoolv2beta1.SpiderCoordinator{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.CoordinatorName,
			},
			Spec: spiderpoolv2beta1.CoordinatorSpec{
				Mode:               &config.CoordinatorMode,
				PodCIDRType:        &config.CoordinatorPodCIDRType,
				TunePodRoutes:      &config.CoordinatorTunePodRoutes,
				DetectIPConflict:   &config.CoordinatorDetectIPConflict,
				DetectGateway:      &config.CoordinatorDetectGateway,
				PodDefaultRouteNIC: &config.CoordinatorPodDefaultRouteNic,
				PodMACPrefix:       &config.CoordinatorPodMACPrefix,
				HijackCIDR:         config.CoordinatorHijackCIDR,
			},
		}
		if err := client.WaitForCoordinatorCreated(ctx, coord); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V4SubnetName) != 0 {
		logger.Sugar().Infof("Try to create default IPv4 Subnet %s", config.V4SubnetName)

		subnet := &spiderpoolv2beta1.SpiderSubnet{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V4SubnetName,
			},
			Spec: spiderpoolv2beta1.SubnetSpec{
				IPVersion: pointer.Int64(constant.IPv4),
				Subnet:    config.V4CIDR,
				IPs:       config.V4IPRanges,
			},
		}
		if len(config.V4Gateway) != 0 {
			subnet.Spec.Gateway = pointer.String(config.V4Gateway)
		}

		if err := client.WaitForSubnetCreated(ctx, subnet); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V6SubnetName) != 0 {
		logger.Sugar().Infof("Try to create default IPv6 Subnet %s", config.V6SubnetName)

		subnet := &spiderpoolv2beta1.SpiderSubnet{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V6SubnetName,
			},
			Spec: spiderpoolv2beta1.SubnetSpec{
				IPVersion: pointer.Int64(constant.IPv6),
				Subnet:    config.V6CIDR,
				IPs:       config.V6IPRanges,
			},
		}
		if len(config.V6Gateway) != 0 {
			subnet.Spec.Gateway = pointer.String(config.V6Gateway)
		}

		if err := client.WaitForSubnetCreated(ctx, subnet); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V4IPPoolName) != 0 {
		logger.Sugar().Infof("Try to create default IPv4 IPPool %s", config.V4IPPoolName)

		ipPool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V4IPPoolName,
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: pointer.Int64(constant.IPv4),
				Subnet:    config.V4CIDR,
				IPs:       config.V4IPRanges,
				Default:   pointer.Bool(true),
			},
		}
		if len(config.V4Gateway) != 0 {
			ipPool.Spec.Gateway = pointer.String(config.V4Gateway)
		}

		if err := client.WaitForIPPoolCreated(ctx, ipPool); err != nil {
			logger.Fatal(err.Error())
		}
	}

	if len(config.V6IPPoolName) != 0 {
		logger.Sugar().Infof("Try to create default IPv6 IPPool %s", config.V6IPPoolName)

		ipPool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.V6IPPoolName,
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: pointer.Int64(constant.IPv6),
				Subnet:    config.V6CIDR,
				IPs:       config.V6IPRanges,
				Default:   pointer.Bool(true),
			},
		}
		if len(config.V6Gateway) != 0 {
			ipPool.Spec.Gateway = pointer.String(config.V6Gateway)
		}

		if err := client.WaitForIPPoolCreated(ctx, ipPool); err != nil {
			logger.Fatal(err.Error())
		}
	}

	// create multuscniconfig for default network
	if config.DefaultCNIName == "" {
		logger.Sugar().Infof("Try to create MultusCniConfig default network in %s", config.DefaultCNIDir)
		if err = InitDefaultMultusCNIConfig(ctx, client, config.DefaultCNIDir); err != nil {
			logger.Fatal(err.Error())
		}
	}

	logger.Info("Finish init")

	// Wait for helm --wait.
	time.Sleep(300 * time.Second)
}

func InitDefaultMultusCNIConfig(ctx context.Context, client *CoreClient, cniDir string) error {
	defaultCNIConfPath, err := findDefaultCNIConf(cniDir)
	if err != nil {
		logger.Sugar().Errorf("failed to findDefaultCNIConf: %v", err)
		return fmt.Errorf("failed to findDefaultCNIConf: %v", err)
	}

	if defaultCNIConfPath == "" {
		// no networks in /etc/cni/net.d
		logger.Sugar().Warnf("No network found in %s, Skip create multuscniconfig", cniDir)
		return nil
	}

	// parse default cni config
	cniName, cniType, err := parseCNIFromConfig(defaultCNIConfPath)
	if err != nil {
		logger.Sugar().Errorf("failed to parseCNIFromConfig: %v", err)
		return fmt.Errorf("failed to parseCNIFromConfig: %v", err)
	}

	if err = client.WaitMultusCNIConfigCreated(ctx, getMultusCniConfig(cniName, cniType)); err != nil {
		return fmt.Errorf("failed to WaitMultusCNIConfigCreated: %v", err)
	}

	return nil
}
