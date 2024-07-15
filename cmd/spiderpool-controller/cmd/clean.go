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
	"github.com/spidernet-io/spiderpool/pkg/k8s/utils"
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
	err := utils.DeleteWebhookConfiguration(ctx, c, validate, vObj)
	if err != nil {
		logger.Sugar().Errorf("failed to delete ValidatingWebhookConfiguration: %s , error: %v.", validate, err)
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up ValidatingWebhookConfiguration of sriov-network-operator
	var sriovOperatorWebhookValidate string = "sriov-operator-webhook-config"
	err = utils.DeleteWebhookConfiguration(ctx, c, sriovOperatorWebhookValidate, vObj)
	if err != nil {
		logger.Sugar().Errorf("failed to delete ValidatingWebhookConfiguration: %s, error: %v.", sriovOperatorWebhookValidate, err)
		jobResult = multierror.Append(jobResult, err)
	}

	mObj := &webhook.MutatingWebhookConfiguration{}
	err = utils.DeleteWebhookConfiguration(ctx, c, mutating, mObj)
	if err != nil {
		logger.Sugar().Errorf("failed to delete MutatingWebhookConfiguration: %s , error: %v.", mutating, err)
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up MutatingWebhookConfiguration of sriov-network-operator
	var sriovNetworkResourcesInjectorMutating string = "network-resources-injector-config"
	var sriovOperatorWebhookMutating string = "sriov-operator-webhook-config"
	err = utils.DeleteWebhookConfiguration(ctx, c, sriovNetworkResourcesInjectorMutating, mObj)
	if err != nil {
		logger.Sugar().Errorf("failed to delete MutatingWebhookConfiguration: %s, error: %v.", sriovNetworkResourcesInjectorMutating, err)
		jobResult = multierror.Append(jobResult, err)
	}
	err = utils.DeleteWebhookConfiguration(ctx, c, sriovOperatorWebhookMutating, mObj)
	if err != nil {
		logger.Sugar().Errorf("failed to delete MutatingWebhookConfiguration: %s, error: %v.", sriovOperatorWebhookMutating, err)
		jobResult = multierror.Append(jobResult, err)
	}

	ipPoolList := new(spiderpoolv2beta1.SpiderIPPoolList)
	err = c.List(ctx, ipPoolList)
	if err == nil {
		for _, item := range ipPoolList.Items {
			item.Finalizers = make([]string, 0)
			err := c.Update(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of ippool: %v, %v", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to clean the finalizers of ippool %v", item.Name)

			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete ippool: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete ippool: %v", item.Name)
		}
	}

	subnetList := new(spiderpoolv2beta1.SpiderSubnetList)
	err = c.List(ctx, subnetList)
	if err == nil {
		for _, item := range subnetList.Items {
			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete subnet: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete subnet: %v", item.Name)
		}
	}

	spiderEndpointList := new(spiderpoolv2beta1.SpiderEndpointList)
	err = c.List(ctx, spiderEndpointList)
	if err == nil {
		for _, item := range spiderEndpointList.Items {
			item.Finalizers = make([]string, 0)
			err := c.Update(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of spiderEndpoint: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to clean the finalizers of spiderEndpoint %v", item.Name)

			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderEndpoint: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderEndpoint: %v", item.Name)
		}
	}

	reservedIPList := new(spiderpoolv2beta1.SpiderReservedIPList)
	err = c.List(ctx, reservedIPList)
	if err == nil {
		for _, item := range reservedIPList.Items {
			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderReservedIP: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderReservedIP: %v", item.Name)
		}
	}

	spiderMultusConfigList := new(spiderpoolv2beta1.SpiderMultusConfigList)
	err = c.List(ctx, spiderMultusConfigList)
	if err == nil {
		for _, item := range spiderMultusConfigList.Items {
			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderMultusConfig: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderMultusConfig: %v", item.Name)
		}
	}

	spiderCoordinatorList := new(spiderpoolv2beta1.SpiderCoordinatorList)
	err = c.List(ctx, spiderCoordinatorList)
	if err == nil {
		for _, item := range spiderCoordinatorList.Items {
			item.Finalizers = make([]string, 0)
			err := c.Update(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of spiderCoordinator: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to clean the finalizers of spiderCoordinator %v", item.Name)

			err = c.Delete(ctx, &item)
			if err != nil {
				logger.Sugar().Errorf("failed to delete spiderCoordinator: %v, %v ", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete spiderCoordinator: %v", item.Name)
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
					logger.Sugar().Errorf("failed to delete customResourceDefinitionList: %v, %v ", item.Name, err)
					jobResult = multierror.Append(jobResult, err)
					continue
				}
				logger.Sugar().Infof("succeeded to delete customResourceDefinitionList: %v", item.Name)
			}
		}
	}

	// Delete all crds of sriov-network-operator
	// After sriov-network-operator was uninstalled, sriov-network-operator did not delete its own CRD,
	// and there were residual CRDs, which might bring some hidden dangers to the upgrade of sriov-network-operator;
	// we tried to uninstall it through spiderpool.
	sriovNetworkOperatorAPIGroup := "sriovnetwork.openshift.io"
	customResourceDefinitionList = new(apiextensionsv1.CustomResourceDefinitionList)
	err = c.List(ctx, customResourceDefinitionList)
	if err == nil {
		for _, item := range customResourceDefinitionList.Items {
			if item.Spec.Group == sriovNetworkOperatorAPIGroup {
				err = c.Delete(ctx, &item)
				if err != nil {
					logger.Sugar().Errorf("failed to delete customResourceDefinitionList: %v, %v ", item.Name, err)
					jobResult = multierror.Append(jobResult, err)
					continue
				}
				logger.Sugar().Infof("succeeded to delete customResourceDefinitionList: %v", item.Name)
			}
		}
	}

	return jobResult.ErrorOrNil()
}
