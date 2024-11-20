// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (iw *IPPoolWebhook) mutateIPPool(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to mutate IPPool")

	if ipPool.DeletionTimestamp != nil {
		logger.Info("Terminating IPPool, noting to mutate")
		return nil
	}

	if !controllerutil.ContainsFinalizer(ipPool, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(ipPool, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	if ipPool.Spec.IPVersion == nil {
		var version types.IPVersion
		if spiderpoolip.IsIPv4CIDR(ipPool.Spec.Subnet) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6CIDR(ipPool.Spec.Subnet) {
			version = constant.IPv6
		} else {
			return fmt.Errorf("failed to generate 'spec.ipVersion' from 'spec.subnet' %s, nothing to mutate", ipPool.Spec.Subnet)
		}

		ipPool.Spec.IPVersion = new(types.IPVersion)
		*ipPool.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	cidr, err := spiderpoolip.CIDRToLabelValue(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet)
	if err != nil {
		return fmt.Errorf("failed to parse 'spec.subnet' %s as a valid label value: %v", ipPool.Spec.Subnet, err)
	}

	if v, ok := ipPool.Labels[constant.LabelIPPoolCIDR]; !ok || v != cidr {
		if ipPool.Labels == nil {
			ipPool.Labels = make(map[string]string)
		}
		ipPool.Labels[constant.LabelIPPoolCIDR] = cidr
		logger.Sugar().Infof("Set label %s: %s", constant.LabelIPPoolCIDR, cidr)
	}

	if iw.EnableSpiderSubnet {
		subnet, err := iw.setControllerSubnet(ctx, ipPool)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("failed to set the reference of the controller Subnet: %v", err))
		}

		// inherit gateway,vlan,routes from corresponding SpiderSubnet if not set
		if subnet != nil {
			InheritSubnetProperties(subnet, ipPool)
		}
	}

	if len(ipPool.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.IPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.ips': %v", err)
		}

		ips := ipPool.Spec.IPs
		ipPool.Spec.IPs = mergedIPs
		logger.Sugar().Debugf("Merge 'spec.ips' %v to %v", ips, mergedIPs)
	}

	if len(ipPool.Spec.ExcludeIPs) > 1 {
		mergedExcludeIPs, err := spiderpoolip.MergeIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.ExcludeIPs)
		if err != nil {
			return fmt.Errorf("failed to merge 'spec.excludeIPs': %v", err)
		}

		excludeIPs := ipPool.Spec.ExcludeIPs
		ipPool.Spec.ExcludeIPs = mergedExcludeIPs
		logger.Sugar().Debugf("Merge 'spec.excludeIPs' %v to %v", excludeIPs, mergedExcludeIPs)
	}

	return nil
}

func (iw *IPPoolWebhook) setControllerSubnet(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool) (*spiderpoolv2beta1.SpiderSubnet, error) {
	logger := logutils.FromContext(ctx)

	owner := metav1.GetControllerOf(ipPool)
	if v, ok := ipPool.Labels[constant.LabelIPPoolOwnerSpiderSubnet]; ok && owner != nil && v == owner.Name {
		return nil, nil
	}

	cidr, err := spiderpoolip.CIDRToLabelValue(*ipPool.Spec.IPVersion, ipPool.Spec.Subnet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CIDR %s as a valid label value: %v", ipPool.Spec.Subnet, err)
	}

	var subnetList spiderpoolv2beta1.SpiderSubnetList
	if err := iw.Client.List(
		ctx,
		&subnetList,
		client.MatchingLabels{constant.LabelSubnetCIDR: cidr},
	); err != nil {
		return nil, fmt.Errorf("failed to list Subnets: %v", err)
	}

	if len(subnetList.Items) == 0 {
		return nil, nil
	}

	subnet := subnetList.Items[0].DeepCopy()
	if !metav1.IsControlledBy(ipPool, subnet) {
		if err := ctrl.SetControllerReference(subnet, ipPool, iw.Client.Scheme()); err != nil {
			return nil, fmt.Errorf("failed to set owner reference: %v", err)
		}
		logger.Sugar().Infof("Set owner reference as Subnet %s", subnet.Name)
	}

	if v, ok := ipPool.Labels[constant.LabelIPPoolOwnerSpiderSubnet]; !ok || v != subnet.Name {
		ipPool.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnet.Name
		logger.Sugar().Infof("Set label %s: %s", constant.LabelIPPoolOwnerSpiderSubnet, subnet.Name)
	}

	return subnet, nil
}

func InheritSubnetProperties(subnet *spiderpoolv2beta1.SpiderSubnet, ipPool *spiderpoolv2beta1.SpiderIPPool) {
	if subnet.Spec.Gateway != nil && ipPool.Spec.Gateway == nil {
		ipPool.Spec.Gateway = ptr.To(*subnet.Spec.Gateway)
	}

	// if customer set empty route for this IPPool, it would not inherit the SpiderSubnet.Spec.Routes
	if len(subnet.Spec.Routes) != 0 && ipPool.Spec.Routes == nil {
		routes := make([]spiderpoolv2beta1.Route, len(subnet.Spec.Routes))
		copy(routes, subnet.Spec.Routes)
		ipPool.Spec.Routes = routes
	}
}
