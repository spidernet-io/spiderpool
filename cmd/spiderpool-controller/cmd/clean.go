// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	webhook "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean resources",
	Long:  "Clean resources with specified parameters.",
	Run: func(cmd *cobra.Command, args []string) {

		validate, err := cmd.Flags().GetString("validate")
		if err != nil {
			logger.Fatal(err.Error())
			os.Exit(1)
		}
		mutating, err := cmd.Flags().GetString("mutating")
		if err != nil {
			logger.Fatal(err.Error())
			os.Exit(1)
		}
		logger.Sugar().Infof("validate %s\nmutating %s\n", validate, mutating)

		client, err := NewCoreClient()
		if err != nil {
			logger.Fatal(err.Error())
			os.Exit(1)
		}
		err = client.clean(validate, mutating)
		if err != nil {
			logger.Fatal(err.Error())
			os.Exit(1)
		}
	},
}

type CoreClient struct {
	client.Client
}

func NewCoreClient() (*CoreClient, error) {
	c, err := client.New(
		ctrl.GetConfigOrDie(),
		client.Options{Scheme: scheme},
	)
	if err != nil {
		return nil, err
	}

	return &CoreClient{Client: c}, nil
}

func (c *CoreClient) clean(validate, mutating string) error {
	var jobResult *multierror.Error
	ctx := context.Background()
	vObj := &webhook.ValidatingWebhookConfiguration{}
	err := c.Get(ctx, client.ObjectKey{Name: validate}, vObj)
	if err == nil {
		err := c.Delete(ctx, vObj)
		if err != nil {
			logger.Sugar().Errorf("failed to delete ValidatingWebhookConfiguration: %v.", err)
			jobResult = multierror.Append(jobResult, err)
		}
		logger.Sugar().Infof("succeeded to delete ValidatingWebhookConfiguration")
	}

	mObj := &webhook.MutatingWebhookConfiguration{}
	err = c.Get(ctx, client.ObjectKey{Name: mutating}, mObj)
	if err == nil {
		err := c.Delete(ctx, mObj)
		if err != nil {
			logger.Sugar().Errorf("failed to delete MutatingWebhookConfiguration: %v.", err)
			jobResult = multierror.Append(jobResult, err)
		}
		logger.Sugar().Infof("succeeded to delete MutatingWebhookConfiguration")
	}

	ipPoolList := new(spiderpoolv2beta1.SpiderIPPoolList)
	err = c.List(ctx, ipPoolList)
	if err == nil {
		for _, item := range ipPoolList.Items {
			item.Finalizers = make([]string, 0)
			err := c.Update(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of ippool: %v, %v", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to clean the finalizers of ippool %v", &item.Name)

			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete ippool: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete ippool: %v", &item.Name)
		}
	}

	subnetList := new(spiderpoolv2beta1.SpiderSubnetList)
	err = c.List(ctx, subnetList)
	if err == nil {
		for _, item := range subnetList.Items {
			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete subnet: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete subnet: %v", &item.Name)
		}
	}

	spiderEndpointList := new(spiderpoolv2beta1.SpiderEndpointList)
	err = c.List(ctx, spiderEndpointList)
	if err == nil {
		for _, item := range spiderEndpointList.Items {
			item.Finalizers = make([]string, 0)
			err := c.Update(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of spiderEndpoint: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to clean the finalizers of spiderEndpoint %v", &item.Name)

			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderEndpoint: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderEndpoint: %v", &item.Name)
		}
	}

	reservedIPList := new(spiderpoolv2beta1.SpiderReservedIPList)
	err = c.List(ctx, reservedIPList)
	if err == nil {
		for _, item := range reservedIPList.Items {
			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderReservedIP: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderReservedIP: %v", &item.Name)
		}
	}

	spiderMultusConfigList := new(spiderpoolv2beta1.SpiderMultusConfigList)
	err = c.List(ctx, spiderMultusConfigList)
	if err == nil {
		for _, item := range spiderMultusConfigList.Items {
			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderMultusConfig: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderMultusConfig: %v", &item.Name)
		}
	}

	spiderCoordinatorList := new(spiderpoolv2beta1.SpiderCoordinatorList)
	err = c.List(ctx, spiderCoordinatorList)
	if err == nil {
		for _, item := range spiderCoordinatorList.Items {
			item.Finalizers = make([]string, 0)
			err := c.Update(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of spiderCoordinator: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to clean the finalizers of spiderCoordinator %v", &item.Name)

			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderCoordinator: %v, %v ", &item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderCoordinator: %v", &item.Name)
		}
	}

	// Delete all crds of spiderpool
	customResourceDefinitionList := new(apiextensionsv1.CustomResourceDefinitionList)
	err = c.List(ctx, customResourceDefinitionList)
	if err == nil {
		for _, item := range customResourceDefinitionList.Items {
			if item.Spec.Group == constant.SpiderpoolAPIGroup {
				err = c.Delete(ctx, &item)
				if err != nil {
					logger.Sugar().Errorf("failed to delete customResourceDefinitionList: %v, %v ", &item.Name, err)
					jobResult = multierror.Append(jobResult, err)
					continue
				}
				logger.Sugar().Infof("succeeded to delete customResourceDefinitionList: %v", &item.Name)
			}
		}
	}

	return jobResult.ErrorOrNil()
}
