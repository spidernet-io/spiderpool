// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spiderpoolv2beta1.AddToScheme(scheme))
}

const retryIntervalSec = 2

type CoreClient struct {
	client.Client
}

func NewCoreClient() (*CoreClient, error) {
	client, err := client.New(
		ctrl.GetConfigOrDie(),
		client.Options{Scheme: scheme},
	)
	if err != nil {
		return nil, err
	}

	return &CoreClient{Client: client}, nil
}

func (c *CoreClient) WaitForCoordinatorCreated(ctx context.Context, coord *spiderpoolv2beta1.SpiderCoordinator) error {
	logger := logutils.FromContext(ctx)

	for {
		err := c.Create(ctx, coord)
		if err == nil {
			logger.Sugar().Infof("Succeed to create default Coordinator: %+v", *coord)
			return nil
		}

		if apierrors.IsAlreadyExists(err) {
			logger.Sugar().Infof("Default Coordinator %s is already exists, ignore creating", coord.Name)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			interval := retryIntervalSec * time.Second
			logger.Sugar().Infof("Failed to create default Coordinator %s, recreate in %s: %v", coord.Name, interval, err)
			time.Sleep(interval)
		}
	}
}

func (c *CoreClient) WaitForSubnetCreated(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet) error {
	logger := logutils.FromContext(ctx)

	for {
		err := c.Create(ctx, subnet)
		if err == nil {
			logger.Sugar().Infof("Succeed to create default IPv%d Subnet: %+v", *subnet.Spec.IPVersion, *subnet)
			return nil
		}

		if apierrors.IsAlreadyExists(err) {
			logger.Sugar().Infof("Default IPv%d Subnet %s is already exists, ignore creating", *subnet.Spec.IPVersion, subnet.Name)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			interval := retryIntervalSec * time.Second
			logger.Sugar().Infof("Failed to create default IPv%d Subnet %s, recreate in %s: %v", *subnet.Spec.IPVersion, subnet.Name, interval, err)
			time.Sleep(interval)
		}
	}
}

func (c *CoreClient) WaitForIPPoolCreated(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) error {
	logger := logutils.FromContext(ctx)

	for {
		err := c.Create(ctx, ipPool)
		if err == nil {
			logger.Sugar().Infof("Succeed to create default IPv%d IPPool: %+v", *ipPool.Spec.IPVersion, *ipPool)
			return nil
		}

		if apierrors.IsAlreadyExists(err) {
			logger.Sugar().Infof("Default IPv%d IPPool %s is already exists, ignore creating", *ipPool.Spec.IPVersion, ipPool.Name)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			interval := retryIntervalSec * time.Second
			logger.Sugar().Infof("Failed to create default IPv%d IPPool %s, recreate in %s: %v", *ipPool.Spec.IPVersion, ipPool.Name, interval, err)
			time.Sleep(interval)
		}
	}
}

func (c *CoreClient) WaitForEndpointReady(ctx context.Context, namespace, name string) error {
	logger := logutils.FromContext(ctx)

	for {
		if c.CheckEndpointsAvailable(ctx, namespace, name) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			interval := retryIntervalSec * time.Second
			logger.Sugar().Infof("Spiderpool controller is not ready, recheck in %s", interval)
			time.Sleep(interval)
		}
	}
}

func (c *CoreClient) CheckEndpointsAvailable(ctx context.Context, namespace, name string) bool {
	var ep corev1.Endpoints
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &ep); err != nil {
		return false
	}

	if len(ep.Subsets) > 0 {
		return true
	}

	return false
}

func (c *CoreClient) WaitMultusCNIConfigCreated(ctx context.Context, multuscniconfig *spiderpoolv2beta1.SpiderMultusConfig) error {
	logger := logutils.FromContext(ctx)

	for {
		err := c.Create(ctx, multuscniconfig)
		if err == nil {
			logger.Sugar().Infof("Succeed to create multuscniconfig %s/%s: %+v", multuscniconfig.Namespace, multuscniconfig.Name, multuscniconfig)
			return nil
		}

		if apierrors.IsAlreadyExists(err) {
			logger.Sugar().Infof("multuscniconfig %s/%s is already exists, ignore creating", multuscniconfig.Namespace, multuscniconfig.Name)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			interval := retryIntervalSec * time.Second
			logger.Sugar().Infof("Failed to create multuscniconfig %s/%s, recreate in %s: %v", multuscniconfig.Namespace, multuscniconfig.Name, interval, err)
			time.Sleep(interval)
		}
	}
}

func (c *CoreClient) WaitPodListReady(ctx context.Context, namespace string, labels map[string]string) error {
	logger := logutils.FromContext(ctx)

	var podList corev1.PodList
	var err error
	noReady := true
	interval := retryIntervalSec * time.Second
	for noReady {
		if err = c.List(ctx, &podList, client.MatchingLabels(labels), client.InNamespace(namespace)); err != nil {
			logger.Sugar().Errorf("failed to get spiderAgent pods: %v, retrying...", err)
			time.Sleep(interval)
			continue
		}

		if podList.Items == nil {
			time.Sleep(interval)
			continue
		}

		noReady = false
		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodRunning {
				noReady = true
				break
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			logger.Sugar().Info("spiderpool-agent not ready, waiting...")
			time.Sleep(interval)
		}
	}

	return nil
}
