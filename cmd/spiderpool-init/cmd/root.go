// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var (
	logger = logutils.Logger

	runtimeClient client.Client

	SpiderControllerEndpointName      = ""
	SpiderControllerEndpointNamespace = ""

	RetryIntervalForApi = time.Second * 2
)

func WaitForSpiderControllerEndpoint(ctx context.Context) {
	logger.Sugar().Infof("begin to check endpoint %s/%s ", SpiderControllerEndpointNamespace, SpiderControllerEndpointName)
	for {
		existed, e := k8sCheckEndpointAvailable(runtimeClient, SpiderControllerEndpointName, SpiderControllerEndpointNamespace)
		if e != nil {
			logger.Sugar().Warnf("failed to check spider controller endpoint : %v ", e)
		} else {
			if existed {
				logger.Info("spider controller is ready")
				return
			} else {
				logger.Info("waiting for spider controller")
			}
		}

		select {
		case <-ctx.Done():
			logger.Fatal("time out , failed  ")

		default:
			time.Sleep(RetryIntervalForApi)
		}
	}
}

func CreateIppool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) {

	for {

		if v, e := k8sCheckIppoolExisted(runtimeClient, pool.Name); e == nil && v != nil {
			logger.Sugar().Errorf(" ippool %v is already existed, ignore creating , detail=%v ", pool.Name, *v)
			return
		}

		e := k8sCreateIppool(runtimeClient, pool)
		if e != nil {
			if apierrors.IsAlreadyExists(e) {
				logger.Sugar().Errorf(" ippool %v is already existed, ignore creating ", pool.Name)
				return
			}
			logger.Sugar().Warnf("failed to create ippool %s , reason=%v ", pool.Name, e)
		} else {
			logger.Sugar().Infof("succeeded to create ippool %v ", pool.Name)
			return
		}

		select {
		case <-ctx.Done():
			logger.Fatal("time out , failed  ")

		default:
			time.Sleep(RetryIntervalForApi)
		}
	}
}

func Execute() {
	// init k8s client
	runtimeClient = InitK8sClient()

	// global context
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*20)
	defer cancel()

	// wait for spider controller endpoint and the webhook is ready
	WaitForSpiderControllerEndpoint(ctx)

	// create ipv4 ippool
	if len(Config.PoolV4Name) > 0 {
		logger.Sugar().Infof("Ipv4 ippool will be created ")

		pool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: Config.PoolV4Name},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet: Config.PoolV4Subnet,
				IPs:    Config.PoolV4IPRanges,
			},
		}
		if len(Config.PoolV4Gateway) > 0 {
			pool.Spec.Gateway = &Config.PoolV4Gateway
		}
		logger.Sugar().Infof("try to create ippool: %+v ", pool)

		CreateIppool(ctx, pool)

	} else {
		logger.Info("Ipv4 ippool will not be created")
	}

	// create ipv6 ippool
	if len(Config.PoolV6Name) > 0 {
		logger.Sugar().Infof("Ipv6 ippool will be created ")

		pool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: Config.PoolV6Name},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet: Config.PoolV6Subnet,
				IPs:    Config.PoolV6IPRanges,
			},
		}
		if len(Config.PoolV6Gateway) > 0 {
			pool.Spec.Gateway = &Config.PoolV6Gateway
		}
		logger.Sugar().Infof("try to create ippool: %+v ", pool)

		CreateIppool(ctx, pool)

	} else {
		logger.Info("Ipv6 ippool will not be created")
	}

	logger.Info("finish initialization")

	// wait for helm --wait
	time.Sleep(time.Second * 300)
}
