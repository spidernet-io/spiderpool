// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/k8s/utils"
	"github.com/spidernet-io/spiderpool/pkg/utils/retry"
	webhook "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

const (
	ENVNamespace          = "SPIDERPOOL_POD_NAMESPACE"
	ENVSpiderpoolInitName = "SPIDERPOOL_INIT_NAME"
)

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

	// Clean up MutatingWebhookConfiguration resources of spiderpool
	if err := c.cleanWebhookResources(ctx, constant.MutatingWebhookConfiguration, mutating, &webhook.MutatingWebhookConfiguration{}); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SriovNetworkResourcesInjectorMutating resources of sriov-network-operator
	if err := c.cleanWebhookResources(ctx, constant.MutatingWebhookConfiguration, constant.SriovNetworkResourcesInjectorMutating, &webhook.MutatingWebhookConfiguration{}); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up sriov-operator-webhook-config resources of sriov-network-operator
	if err := c.cleanWebhookResources(ctx, constant.MutatingWebhookConfiguration, constant.SriovOperatorWebhookConfigMutatingOrValidate, &webhook.MutatingWebhookConfiguration{}); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up ValidatingWebhookConfiguration resources of spiderpool
	if err := c.cleanWebhookResources(ctx, constant.ValidatingWebhookConfiguration, validate, &webhook.ValidatingWebhookConfiguration{}); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up sriov-operator-webhook-config resources of sriov-network-operator
	if err := c.cleanWebhookResources(ctx, constant.ValidatingWebhookConfiguration, constant.SriovOperatorWebhookConfigMutatingOrValidate, &webhook.ValidatingWebhookConfiguration{}); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderIPPool resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderIPPoolList{}, constant.KindSpiderIPPool); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderSubnet resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderSubnetList{}, constant.KindSpiderSubnet); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderEndpoint resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderEndpointList{}, constant.KindSpiderEndpoint); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderReservedIP resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderReservedIPList{}, constant.KindSpiderReservedIP); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderMultusConfig resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderMultusConfigList{}, constant.KindSpiderMultusConfig); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderCoordinator resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderCoordinatorList{}, constant.KindSpiderCoordinator); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Clean up SpiderClaimParameter resources of spiderpool
	if err := c.cleanSpiderpoolResources(ctx, &spiderpoolv2beta1.SpiderClaimParameterList{}, constant.KindSpiderClaimParameter); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Delete all crds of spiderpool or sriov-network-operator
	if err := c.cleanCRDs(ctx); err != nil {
		jobResult = multierror.Append(jobResult, err)
	}

	// Delete Job of spiderpool-Init
	spiderpoolInitNamespace := strings.ReplaceAll(os.Getenv(ENVNamespace), "\"", "")
	if len(spiderpoolInitNamespace) == 0 {
		logger.Sugar().Errorf("Tried to clean up spiderpool-init job, but ENV %s %w", ENVNamespace, constant.ErrMissingRequiredParam)
	}
	spiderpoolInitName := strings.ReplaceAll(os.Getenv(ENVSpiderpoolInitName), "\"", "")
	if len(spiderpoolInitName) == 0 {
		logger.Sugar().Errorf("Tried to clean up spiderpool-init job, but ENV %s %v", ENVSpiderpoolInitName, constant.ErrMissingRequiredParam)
	}

	if len(spiderpoolInitName) != 0 && len(spiderpoolInitNamespace) != 0 {
		if err := c.cleanSpiderpoolInitJob(ctx, spiderpoolInitNamespace, spiderpoolInitName); err != nil {
			jobResult = multierror.Append(jobResult, err)
		}
	} else {
		logger.Sugar().Error("skipping spiderpool-init job cleanup due to missing spiderpool-init environment variables")
	}

	return jobResult.ErrorOrNil()
}

// cleanWebhookResources deletes a specific webhook configuration based on the provided resource type and name.
func (c *CoreClient) cleanWebhookResources(ctx context.Context, resourceType, resourceName string, obj client.Object) error {
	err := utils.DeleteWebhookConfiguration(ctx, c, resourceName, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Infof("%s: %s does not exist, ignore it. error: %v", resourceType, resourceName, err)
			return nil
		}

		logger.Sugar().Errorf("failed to delete %s: %s, error: %v", resourceType, resourceName, err)
		return err
	}

	logger.Sugar().Infof("succeeded to delete %s: %s", resourceType, resourceName)
	return nil
}

// cleanSpiderpoolResources lists and deletes specific Spiderpool resources, with an optional finalizer cleanup step.
func (c *CoreClient) cleanSpiderpoolResources(ctx context.Context, list client.ObjectList, resourceName string) error {
	var jobResult *multierror.Error
	err := c.List(ctx, list)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Infof("%s does not exist, ignore it. error: %v", resourceName, err)
			return nil
		}
		logger.Sugar().Errorf("failed to list %s, error: %v", resourceName, err)
		return err
	}

	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	for i := 0; i < items.Len(); i++ {
		item := items.Index(i).Addr().Interface().(client.Object)

		cleanFinalizers := false
		switch resourceName {
		case constant.KindSpiderIPPool:
			cleanFinalizers = true
		case constant.KindSpiderEndpoint:
			cleanFinalizers = true
		case constant.KindSpiderCoordinator:
			cleanFinalizers = true
		default:
			cleanFinalizers = false
		}

		if cleanFinalizers {
			err = retry.RetryOnConflictWithContext(ctx, retry.DefaultBackoff, func(ctx context.Context) error {
				copyItem := item.DeepCopyObject().(client.Object)
				err = c.Get(ctx, client.ObjectKeyFromObject(item), copyItem)
				if err != nil {
					if apierrors.IsNotFound(err) {
						logger.Sugar().Infof("%s: %v does not exist, skip finalizer removal, error: %v", resourceName, item.GetName(), err)
						return nil
					}
					return err
				}

				if len(copyItem.GetFinalizers()) == 0 {
					logger.Sugar().Infof("%s: %v has no finalizers, skipping finalizer removal", resourceName, item.GetName())
					return nil
				}

				copyItem.SetFinalizers(nil)
				if err := c.Update(ctx, copyItem); err != nil {
					if apierrors.IsConflict(err) {
						logger.Sugar().Warnf("A conflict occurred when updating the status of Spiderpool resource %s: %s, %v", resourceName, copyItem.GetName(), err)
					}
					return err
				}
				logger.Sugar().Infof("succeeded to clean the finalizers of %s %v", resourceName, item.GetName())
				return nil
			})

			if err != nil {
				logger.Sugar().Errorf("failed to clean the finalizers of %s: %v, %v", resourceName, item.GetName(), err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
		}

		err = c.Delete(ctx, item)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Sugar().Infof("%s: %v does not exist, ignore it. error: %v", resourceName, item.GetName(), err)
				continue
			}
			logger.Sugar().Errorf("failed to delete %s: %v, %v", resourceName, item.GetName(), err)
			jobResult = multierror.Append(jobResult, err)
			continue
		}
		logger.Sugar().Infof("succeeded to delete %s: %v", resourceName, item.GetName())
	}

	return jobResult.ErrorOrNil()
}

// cleanCRDs lists and deletes CustomResourceDefinitions (CRDs) related to Spiderpool and sriov-network-operator.
func (c *CoreClient) cleanCRDs(ctx context.Context) error {
	var jobResult *multierror.Error
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	err := c.List(ctx, crdList)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Infof("CustomResourceDefinitionList does not exist, ignore it. error: %v", err)
			return nil
		}
		logger.Sugar().Errorf("failed to list CustomResourceDefinitionList, error: %v", err)
		return err
	}

	for _, item := range crdList.Items {
		cleanCRD := false
		switch item.Spec.Group {
		case constant.SpiderpoolAPIGroup:
			cleanCRD = true
		// Delete all crds of sriov-network-operator
		// After sriov-network-operator was uninstalled, sriov-network-operator did not delete its own CRD,
		// and there were residual CRDs, which might bring some hidden dangers to the upgrade of sriov-network-operator;
		// we tried to uninstall it through spiderpool.
		case constant.SriovNetworkOperatorAPIGroup:
			// After helm uninstall, sriov-operator will delete the resources under sriovoperatorconfigs.sriovnetwork.openshift.io.
			// If we delete this CRD resource in advance, helm uninstall will report an error.
			// We will skip it for now to allow other resources to be deleted.
			if item.Name == constant.SriovNetworkOperatorConfigs {
				cleanCRD = false
			} else {
				cleanCRD = true
			}
		default:
			cleanCRD = false
		}

		if cleanCRD {
			err = c.Delete(ctx, &item)
			if err != nil {
				if apierrors.IsNotFound(err) {
					logger.Sugar().Infof("CustomResourceDefinition: %v does not exist, ignore it. error: %v", item.Name, err)
					continue
				}
				logger.Sugar().Errorf("failed to delete CustomResourceDefinition: %v, error: %v", item.Name, err)
				jobResult = multierror.Append(jobResult, err)
				continue
			}
			logger.Sugar().Infof("succeeded to delete CustomResourceDefinition: %v", item.Name)
		}
	}

	return jobResult.ErrorOrNil()
}

// cleanSpiderpoolInitJob deletes the spiderpool-init Job, logs any errors or success.
func (c *CoreClient) cleanSpiderpoolInitJob(ctx context.Context, spiderpoolInitNamespace, spiderpoolInitName string) error {
	spiderpoolInitJob := &batchv1.Job{}
	err := c.Get(ctx, types.NamespacedName{Namespace: spiderpoolInitNamespace, Name: spiderpoolInitName}, spiderpoolInitJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Infof("spiderpool-init Job %s/%s does not exist, ignore it. error: %v", spiderpoolInitNamespace, spiderpoolInitName, err)
			return nil
		}
		logger.Sugar().Errorf("failed to get spiderpool-init Job %s/%s, error: %v", spiderpoolInitNamespace, spiderpoolInitName, err)
		return err
	}

	propagationPolicy := metav1.DeletePropagationBackground
	err = c.Delete(ctx, spiderpoolInitJob, &client.DeleteOptions{PropagationPolicy: &propagationPolicy})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Infof("spiderpool-init Job %s/%s does not exist, ignore it. error: %v", spiderpoolInitNamespace, spiderpoolInitName, err)
			return nil
		}
		logger.Sugar().Errorf("failed to delete spiderpool-init Job: %v/%v, error: %v", spiderpoolInitJob.Namespace, spiderpoolInitJob.Name, err)
		return err
	}
	logger.Sugar().Infof("succeeded to delete spiderpool-init Job: %v/%v", spiderpoolInitJob.Namespace, spiderpoolInitJob.Name)

	return nil
}
