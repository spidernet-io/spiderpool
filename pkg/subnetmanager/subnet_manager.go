// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

type SubnetManager interface {
	GetSubnetByName(ctx context.Context, subnetName string, cached bool) (*spiderpoolv1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error)
	ReconcileAutoIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetName string, podController types.PodTopController, autoPoolProperty types.AutoPoolProperty) (*spiderpoolv1.SpiderIPPool, error)
}

type subnetManager struct {
	client        client.Client
	apiReader     client.Reader
	reservedIPMgr reservedipmanager.ReservedIPManager
}

func NewSubnetManager(client client.Client, apiReader client.Reader, reservedIPMgr reservedipmanager.ReservedIPManager) (SubnetManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}
	if reservedIPMgr == nil {
		return nil, fmt.Errorf("reserved-IP manager %w", constant.ErrMissingRequiredParam)
	}

	return &subnetManager{
		client:        client,
		apiReader:     apiReader,
		reservedIPMgr: reservedIPMgr,
	}, nil
}

func (sm *subnetManager) GetSubnetByName(ctx context.Context, subnetName string, cached bool) (*spiderpoolv1.SpiderSubnet, error) {
	reader := sm.apiReader
	if cached == constant.UseCache {
		reader = sm.client
	}

	var subnet spiderpoolv1.SpiderSubnet
	if err := reader.Get(ctx, apitypes.NamespacedName{Name: subnetName}, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

func (sm *subnetManager) ListSubnets(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error) {
	reader := sm.apiReader
	if cached == constant.UseCache {
		reader = sm.client
	}

	var subnetList spiderpoolv1.SpiderSubnetList
	if err := reader.List(ctx, &subnetList, opts...); err != nil {
		return nil, err
	}

	return &subnetList, nil
}

func (sm *subnetManager) ReconcileAutoIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetName string,
	podController types.PodTopController, autoPoolProperty types.AutoPoolProperty) (*spiderpoolv1.SpiderIPPool, error) {
	if len(subnetName) == 0 {
		return nil, fmt.Errorf("%w: spider subnet name must be specified", constant.ErrWrongInput)
	}
	if autoPoolProperty.DesiredIPNumber < 0 {
		return nil, fmt.Errorf("%w: the required IP numbers '%d' is invalid", constant.ErrWrongInput, autoPoolProperty.DesiredIPNumber)
	}
	log := logutils.FromContext(ctx)

	// check if the pool needs to be created
	operationCreate := pool == nil

	// check if the given pool's IPs numbers are equal with the desired IP number counts
	if !operationCreate {
		poolIPs, err := spiderpoolip.ParseIPRanges(autoPoolProperty.IPVersion, pool.Spec.IPs)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse IPPool %s Spec IPs %s: %v", constant.ErrWrongInput, pool.Name, pool.Spec.IPs, err)
		}
		if len(poolIPs) == autoPoolProperty.DesiredIPNumber {
			log.Sugar().Debugf("Auto-created IPPool %s matches the desired IP number %d, no need to reconcile", pool.Name, autoPoolProperty.DesiredIPNumber)
			return pool, nil
		}
	}

	subnet, err := sm.GetSubnetByName(ctx, subnetName, constant.IgnoreCache)
	if nil != err {
		return nil, fmt.Errorf("failed to get SpiderSubnet %s, error: %w", subnetName, err)
	}
	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't reconcile an auto-created IPPool from it", constant.ErrWrongInput, subnet.Name)
	}

	if operationCreate {
		pool = &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: applicationinformers.SubnetPoolName(podController.Kind, podController.Namespace, podController.Name, autoPoolProperty.IPVersion, autoPoolProperty.IfName, podController.UID),
			},
			Spec: spiderpoolv1.IPPoolSpec{
				IPVersion:   pointer.Int64(autoPoolProperty.IPVersion),
				Subnet:      subnet.Spec.Subnet,
				Gateway:     subnet.Spec.Gateway,
				Vlan:        subnet.Spec.Vlan,
				Routes:      subnet.Spec.Routes,
				PodAffinity: autoPoolProperty.PodSelector,
			},
		}

		poolLabels := map[string]string{
			constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
			constant.LabelIPPoolOwnerApplication:    applicationinformers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
			constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
		}

		cidrLabelValue, err := spiderpoolip.CIDRToLabelValue(*pool.Spec.IPVersion, pool.Spec.Subnet)
		if nil != err {
			return nil, fmt.Errorf("failed to parse '%s' when allocating empty Auto-created IPPool '%v'", pool.Spec.Subnet, pool)
		}
		poolLabels[constant.LabelIPPoolCIDR] = cidrLabelValue
		if autoPoolProperty.IsReclaimIPPool {
			poolLabels[constant.LabelIPPoolReclaimIPPool] = constant.True
		}
		pool.Labels = poolLabels

		// set owner reference
		err = ctrl.SetControllerReference(subnet, pool, sm.client.Scheme())
		if nil != err {
			return nil, fmt.Errorf("failed to set SpiderIPPool %s owner reference with SpiderSubnet %s: %v", pool.Name, subnetName, err)
		}

		// set finalizer
		controllerutil.AddFinalizer(pool, constant.SpiderFinalizer)
	}

	log.Sugar().Infof("try to pre-allocate IPs from subnet %s for auto-create IPPool %s with %d IPs", subnetName, pool.Name, autoPoolProperty.DesiredIPNumber)
	ips, err := sm.preAllocateIPsFromSubnet(ctx, subnet, pool, autoPoolProperty.IPVersion, autoPoolProperty.DesiredIPNumber, podController)
	if nil != err {
		return nil, fmt.Errorf("failed to pre-allocate auto-create IPPool %s IPs from SpiderSubnet %s: %w", pool.Name, subnetName, err)
	}
	pool.Spec.IPs = ips

	if operationCreate {
		log.Sugar().Infof("try to create IPPool '%v'", pool)
		err = sm.client.Create(ctx, pool)
		if nil != err {
			return nil, fmt.Errorf("failed to create auto-created IPPool %v: %w", pool, err)
		}
	} else {
		err = sm.client.Update(ctx, pool)
		if nil != err {
			return nil, fmt.Errorf("failed to update already exist auto-created IPPool %s: %w", pool.Name, err)
		}
	}

	log.Sugar().Infof("apply auto-created IPPool '%v' successfully", pool)
	return pool, nil
}

// preAllocateIPsFromSubnet will calculate the auto-created IPPool required IPs from corresponding SpiderSubnet and return it.
func (sm *subnetManager) preAllocateIPsFromSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet, pool *spiderpoolv1.SpiderIPPool, ipVersion types.IPVersion, desiredIPNum int, podController types.PodTopController) ([]string, error) {
	log := logutils.FromContext(ctx)

	var beforeAllocatedIPs []net.IP
	ipNum := desiredIPNum

	var subnetControlledIPPools spiderpoolv1.PoolIPPreAllocations
	if subnet.Status.ControlledIPPools != nil {
		err := json.Unmarshal([]byte(*subnet.Status.ControlledIPPools), &subnetControlledIPPools)
		if nil != err {
			return nil, err
		}
	}

	subnetPoolAllocation, ok := subnetControlledIPPools[pool.Name]
	if ok {
		log.Sugar().Infof("fetched the last IPPool %s last allocated %v from SpiderSubnet %s", pool.Name, subnetPoolAllocation, subnet.Name)
		subnetPoolIPs, err := spiderpoolip.ParseIPRanges(ipVersion, subnetPoolAllocation.IPs)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse SpiderSubnet '%s' Status ControlledIPPool '%s' IPs '%v', error: %v",
				constant.ErrWrongInput, subnet.Name, pool.Name, subnetPoolAllocation.IPs, err)
		}

		// In the last reconcile process, the SpiderSubnet allocated IPs successfully but the pool creation process failed.
		if desiredIPNum == len(subnetPoolIPs) {
			log.Sugar().Debugf("match the last IPPool %s last allocated %v IP number from SpiderSubnet %s, just reuse it",
				pool.Name, subnetPoolAllocation.IPs, subnet.Name)
			return subnetPoolAllocation.IPs, nil
		} else if desiredIPNum < len(subnetPoolIPs) {
			log.Sugar().Debugf("IPPool %s decresed its desired IP number from %d to %d", pool.Name, len(subnetPoolIPs), desiredIPNum)
			poolIPAllocations, err := convert.UnmarshalSubnetAllocatedIPPools(pool.Status.AllocatedIPs)
			if nil != err {
				return nil, fmt.Errorf("failed to parse IPPool %s Status AllocatedIPs: %v", pool.Name, err)
			}

			// shrink: free IP number >= return IP Num
			// when it needs to scale down IP, enough IP is released to make sure it scale down successfully
			if len(subnetPoolIPs)-len(poolIPAllocations) >= len(subnetPoolIPs)-desiredIPNum {
				// exist auto pool allocated IPs
				poolAllocatedIPRanges := make([]string, 0, len(poolIPAllocations))
				for tmpIP := range poolIPAllocations {
					poolAllocatedIPRanges = append(poolAllocatedIPRanges, tmpIP)
				}
				poolAllocatedIPs, err := spiderpoolip.ParseIPRanges(ipVersion, poolAllocatedIPRanges)
				if nil != err {
					return nil, fmt.Errorf("%w: failed to parse IP ranges '%v', error: %v", constant.ErrWrongInput, poolAllocatedIPs, err)
				}

				// free IPs
				freeIPs := spiderpoolip.IPsDiffSet(subnetPoolIPs, poolAllocatedIPs, true)
				discardedIPs := freeIPs[:len(subnetPoolIPs)-desiredIPNum]
				newlyIPs := spiderpoolip.IPsDiffSet(subnetPoolIPs, discardedIPs, false)
				poolIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, newlyIPs)
				if nil != err {
					return nil, err
				}
				subnetControlledIPPools[pool.Name] = spiderpoolv1.PoolIPPreAllocation{
					IPs:         poolIPRange,
					Application: pointer.String(ApplicationNamespacedName(podController.AppNamespacedName)),
				}
				marshalSubnetAllocatedIPPools, err := convert.MarshalSubnetAllocatedIPPools(subnetControlledIPPools)
				if nil != err {
					return nil, fmt.Errorf("%w: failed to marshal Subnet controlled IPPools %v: %v", constant.ErrWrongInput, subnetControlledIPPools, err)
				}
				subnet.Status.ControlledIPPools = marshalSubnetAllocatedIPPools
				totalCount, allocatedCount := subnetStatusCount(subnet)
				subnet.Status.TotalIPCount = &totalCount
				subnet.Status.AllocatedIPCount = &allocatedCount
				err = sm.client.Status().Update(ctx, subnet)
				if nil != err {
					return nil, fmt.Errorf("failed to allocate IPPool %s newsly IPs %v from SpiderSubnet %s: %w", pool.Name, poolIPRange, subnet.Name, err)
				}
				log.Sugar().Debugf("allocate IPPool %s newly IPs %v from SpiderSubnet %s", pool.Name, poolIPRange, subnet.Name)
				return poolIPRange, nil
			}
			return nil, fmt.Errorf("failed to scale down IPPool %s IPs: %w", pool.Name, constant.ErrFreeIPsNotEnough)
		} else {
			log.Sugar().Debugf("IPPool %s increased its desired IP number from %d to %d, and the last allocation is %v",
				pool.Name, len(subnetPoolIPs), desiredIPNum, subnetPoolAllocation.IPs)
			beforeAllocatedIPs = subnetPoolIPs
			ipNum = desiredIPNum - len(subnetPoolIPs)
		}
	}

	freeIPs, err := applicationinformers.GenSubnetFreeIPs(subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to generate SpiderSubnet '%s' free IPs, error: %v", subnet.Name, err)
	}

	// filter reserved IPs
	reservedIPs, err := sm.reservedIPMgr.AssembleReservedIPs(ctx, ipVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to filter reservedIPs '%v' by IP version '%d', error: %v",
			constant.ErrWrongInput, reservedIPs, ipVersion, err)
	}

	if len(reservedIPs) != 0 {
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, reservedIPs, true)
	}

	// check the filtered subnet free IP number is enough or not
	if len(freeIPs) < ipNum {
		return nil, fmt.Errorf("insufficient subnet FreeIPs, required '%d' but only left '%d'", ipNum, len(freeIPs))
	}

	allocateIPs := make([]net.IP, 0, ipNum)
	allocateIPs = append(allocateIPs, freeIPs[:ipNum]...)
	if len(beforeAllocatedIPs) != 0 {
		allocateIPs = append(allocateIPs, beforeAllocatedIPs...)
	}
	allocateIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, allocateIPs)
	if nil != err {
		return nil, err
	}

	newlyControlledIPPools := make(spiderpoolv1.PoolIPPreAllocations, len(subnetControlledIPPools))
	for tmpPoolName, poolAllocation := range subnetControlledIPPools {
		newlyControlledIPPools[tmpPoolName] = poolAllocation
	}
	newlyControlledIPPools[pool.Name] = spiderpoolv1.PoolIPPreAllocation{
		IPs:         allocateIPRange,
		Application: pointer.String(ApplicationNamespacedName(podController.AppNamespacedName)),
	}
	data, err := json.Marshal(newlyControlledIPPools)
	if nil != err {
		return nil, err
	}
	subnet.Status.ControlledIPPools = pointer.String(string(data))
	totalCount, allocatedCount := subnetStatusCount(subnet)
	subnet.Status.TotalIPCount = &totalCount
	subnet.Status.AllocatedIPCount = &allocatedCount

	err = sm.client.Status().Update(ctx, subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to generate %d IPs from SpiderSubnet %s for IPPool %s: %w", desiredIPNum, subnet.Name, pool.Name, err)
	}

	log.Sugar().Infof("generated '%d' IPs '%v' from SpiderSubnet '%s' for IPPool %s", desiredIPNum, allocateIPRange, subnet.Name, pool.Name)
	return allocateIPRange, nil
}

// TODO(Icarus9913): remove it
func subnetStatusCount(subnet *spiderpoolv1.SpiderSubnet) (totalCount, allocatedCount int64) {
	// total IP Count
	subnetTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)

	if subnet.Status.ControlledIPPools == nil {
		return 0, 0
	}
	var controlledIPPools spiderpoolv1.PoolIPPreAllocations
	err := json.Unmarshal([]byte(*subnet.Status.ControlledIPPools), &controlledIPPools)
	if nil != err {
		return 0, 0
	}

	// allocated IP Count
	var allocatedIPCount int64
	for _, poolAllocation := range controlledIPPools {
		tmpIPs, _ := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, poolAllocation.IPs)
		allocatedIPCount += int64(len(tmpIPs))
	}
	return int64(len(subnetTotalIPs)), allocatedIPCount
}

// TODO(Icarus9913): move it
func ApplicationNamespacedName(appNamespacedName types.AppNamespacedName) string {
	return fmt.Sprintf("%s_%s_%s_%s", appNamespacedName.APIVersion, appNamespacedName.Kind, appNamespacedName.Namespace, appNamespacedName.Name)
}

// TODO(Icarus9913): move it
func ParseApplicationNamespacedName(appNamespacedNameKey string) (appNamespacedName types.AppNamespacedName, isMatch bool) {
	split := strings.Split(appNamespacedNameKey, "_")
	if len(split) == 4 {
		return types.AppNamespacedName{
			APIVersion: split[0],
			Kind:       split[1],
			Namespace:  split[2],
			Name:       split[3],
		}, true
	}

	return
}
