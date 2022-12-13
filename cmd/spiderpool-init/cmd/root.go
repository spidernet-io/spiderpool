// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func CreateSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) {

	for {

		if v, e := k8sCheckSubnetExisted(runtimeClient, subnet.Name); e == nil && v != nil {
			logger.Sugar().Errorf(" subnet %v is already existed, ignore creating , detail=%v ", subnet.Name, *v)
			return
		}

		e := k8sCreateSubnet(runtimeClient, subnet)
		if e != nil {
			if apierrors.IsAlreadyExists(e) {
				logger.Sugar().Errorf(" subnet %v is already existed, ignore creating ", subnet.Name)
				return
			}
			logger.Sugar().Warnf("failed to create subnet %s , reason=%v ", subnet.Name, e)
		} else {
			logger.Sugar().Infof("succeeded to create subnet %v ", subnet.Name)
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

	// create ipv4 subnet
	if len(Config.SubnetV4Name) > 0 {
		logger.Sugar().Infof("Ipv4 subnet will be created ")

		obj := &spiderpoolv1.SpiderSubnet{
			ObjectMeta: metav1.ObjectMeta{Name: Config.SubnetV4Name},
			Spec: spiderpoolv1.SubnetSpec{
				Subnet: Config.PoolV4Subnet,
				IPs:    Config.PoolV4IPRanges,
			},
		}
		if len(Config.PoolV4Gateway) > 0 {
			obj.Spec.Gateway = &Config.PoolV4Gateway
		}
		logger.Sugar().Infof("try to create subnet: %+v ", obj)

		CreateSubnet(ctx, obj)

	} else {
		logger.Info("Ipv4 subnet will not be created")
	}

	// create ipv4 ippool
	if len(Config.PoolV4Name) > 0 {
		logger.Sugar().Infof("Ipv4 ippool will be created ")

		pool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: Config.PoolV4Name},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet: Config.PoolV4Subnet,
			},
		}

		// if we create SpiderSubnet CR object, we'll create an empty default IPPool.
		// Otherwise, we'll create a truly useful default IPPool
		if len(Config.SubnetV4Name) == 0 {
			pool.Spec.IPs = Config.PoolV4IPRanges
		}

		if len(Config.PoolV4Gateway) > 0 {
			pool.Spec.Gateway = &Config.PoolV4Gateway
		}
		logger.Sugar().Infof("try to create ippool: %+v ", pool)

		CreateIppool(ctx, pool)

	} else {
		logger.Info("Ipv4 ippool will not be created")
	}

	// create ipv6 subnet
	if len(Config.SubnetV6Name) > 0 {
		logger.Sugar().Infof("Ipv6 subnet will be created ")

		obj := &spiderpoolv1.SpiderSubnet{
			ObjectMeta: metav1.ObjectMeta{Name: Config.SubnetV6Name},
			Spec: spiderpoolv1.SubnetSpec{
				Subnet: Config.PoolV6Subnet,
				IPs:    Config.PoolV6IPRanges,
			},
		}
		if len(Config.PoolV6Gateway) > 0 {
			obj.Spec.Gateway = &Config.PoolV6Gateway
		}
		logger.Sugar().Infof("try to create subnet: %+v ", obj)

		CreateSubnet(ctx, obj)

	} else {
		logger.Info("Ipv6 subnet will not be created")
	}

	// create ipv6 ippool
	if len(Config.PoolV6Name) > 0 {
		logger.Sugar().Infof("Ipv6 ippool will be created ")

		pool := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: Config.PoolV6Name},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet: Config.PoolV6Subnet,
			},
		}

		// if we create SpiderSubnet CR object, we'll create an empty default IPPool.
		// Otherwise, we'll create a truly useful default IPPool
		if len(Config.SubnetV6Name) == 0 {
			pool.Spec.IPs = Config.PoolV6IPRanges
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
