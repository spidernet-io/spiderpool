// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/event"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

type SubnetManager interface {
	GetSubnetByName(ctx context.Context, subnetName string, cached bool) (*spiderpoolv2beta1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderSubnetList, error)
	ReconcileAutoIPPool(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool, subnetName string, podController types.PodTopController, autoPoolProperty types.AutoPoolProperty) (*spiderpoolv2beta1.SpiderIPPool, error)
}

type subnetManager struct {
	client     client.Client
	apiReader  client.Reader
	rIPManager reservedipmanager.ReservedIPManager
}

func NewSubnetManager(client client.Client, apiReader client.Reader, rIPManager reservedipmanager.ReservedIPManager) (SubnetManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}
	if rIPManager == nil {
		return nil, fmt.Errorf("reserved-IP manager %w", constant.ErrMissingRequiredParam)
	}

	return &subnetManager{
		client:     client,
		apiReader:  apiReader,
		rIPManager: rIPManager,
	}, nil
}

func (sm *subnetManager) GetSubnetByName(ctx context.Context, subnetName string, cached bool) (*spiderpoolv2beta1.SpiderSubnet, error) {
	reader := sm.apiReader
	if cached == constant.UseCache {
		reader = sm.client
	}

	var subnet spiderpoolv2beta1.SpiderSubnet
	if err := reader.Get(ctx, apitypes.NamespacedName{Name: subnetName}, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

func (sm *subnetManager) ListSubnets(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderSubnetList, error) {
	reader := sm.apiReader
	if cached == constant.UseCache {
		reader = sm.client
	}

	var subnetList spiderpoolv2beta1.SpiderSubnetList
	if err := reader.List(ctx, &subnetList, opts...); err != nil {
		return nil, err
	}

	return &subnetList, nil
}

func (sm *subnetManager) ReconcileAutoIPPool(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool, subnetName string,
	podController types.PodTopController, autoPoolProperty types.AutoPoolProperty) (*spiderpoolv2beta1.SpiderIPPool, error) {
	if len(subnetName) == 0 {
		return nil, fmt.Errorf("%w: spider subnet name must be specified", constant.ErrWrongInput)
	}
	if autoPoolProperty.DesiredIPNumber < 0 {
		return nil, fmt.Errorf("%w: the required IP numbers '%d' is invalid", constant.ErrWrongInput, autoPoolProperty.DesiredIPNumber)
	}

	log := logutils.FromContext(ctx)

	subnet, err := sm.GetSubnetByName(ctx, subnetName, constant.IgnoreCache)
	if nil != err {
		return nil, fmt.Errorf("failed to get SpiderSubnet %s, error: %w", subnetName, err)
	}
	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't reconcile an auto-created IPPool from it", constant.ErrWrongInput, subnet.Name)
	}

	// check if the pool needs to be created
	operationCreate := pool == nil

	// check if the given pool's IPs numbers are equal with the desired IP number counts
	if !operationCreate {
		if pool.Spec.Subnet != subnet.Spec.Subnet {
			event.EventRecorder.Eventf(pool, corev1.EventTypeWarning, "ApplicationSubnetChanged",
				"the corresponding application specified SpiderSubnet changed from %s to %s", pool.Labels[constant.LabelIPPoolOwnerSpiderSubnet], subnetName)
			return nil, fmt.Errorf("%w: it's invalid to change recoincile auto-created IPPool %s with different subnet SpiderSubnet %s", constant.ErrWrongInput, pool.Name, subnetName)
		}

		poolIPs, err := spiderpoolip.ParseIPRanges(autoPoolProperty.IPVersion, pool.Spec.IPs)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse IPPool %s Spec IPs %s: %v", constant.ErrWrongInput, pool.Name, pool.Spec.IPs, err)
		}
		if len(poolIPs) == autoPoolProperty.DesiredIPNumber {
			oldAppUID := pool.Labels[constant.LabelIPPoolOwnerApplicationUID]
			oldReclaimIPPoolStr := pool.Labels[constant.LabelIPPoolReclaimIPPool]

			if oldAppUID == string(podController.UID) &&
				oldReclaimIPPoolStr == applicationinformers.IsReclaimAutoPoolLabelValue(autoPoolProperty.IsReclaimIPPool) {
				log.Sugar().Debugf("Auto-created IPPool %s matches the desired IP number %d, no need to reconcile", pool.Name, autoPoolProperty.DesiredIPNumber)
				return pool, nil
			}
		}

		// refresh the label "ipam.spidernet.io/owner-application-uid" and "ipam.spidernet.io/ippool-reclaim"
		labels := pool.GetLabels()
		labels[constant.LabelIPPoolOwnerApplicationUID] = string(podController.UID)
		labels[constant.LabelIPPoolReclaimIPPool] = applicationinformers.IsReclaimAutoPoolLabelValue(autoPoolProperty.IsReclaimIPPool)
		pool.SetLabels(labels)
	} else {
		pool = &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: applicationinformers.AutoPoolName(podController.Name, autoPoolProperty.IPVersion, autoPoolProperty.IfName, podController.UID),
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: ptr.To(autoPoolProperty.IPVersion),
				Subnet:    subnet.Spec.Subnet,
				Gateway:   subnet.Spec.Gateway,
				//Vlan:        subnet.Spec.Vlan,
				Routes:      subnet.Spec.Routes,
				PodAffinity: ippoolmanager.NewAutoPoolPodAffinity(podController),
			},
		}

		{
			poolLabels := map[string]string{
				constant.LabelIPPoolOwnerSpiderSubnet:         subnet.Name,
				constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(podController.APIVersion),
				constant.LabelIPPoolOwnerApplicationKind:      podController.Kind,
				constant.LabelIPPoolOwnerApplicationNamespace: podController.Namespace,
				constant.LabelIPPoolOwnerApplicationName:      podController.Name,
				constant.LabelIPPoolOwnerApplicationUID:       string(podController.UID),
				constant.LabelIPPoolInterface:                 autoPoolProperty.IfName,
				constant.LabelIPPoolReclaimIPPool:             applicationinformers.IsReclaimAutoPoolLabelValue(autoPoolProperty.IsReclaimIPPool),
				constant.LabelIPPoolIPVersion:                 applicationinformers.AutoPoolIPVersionLabelValue(autoPoolProperty.IPVersion),
			}
			// label IPPoolCIDR
			cidrLabelValue, err := spiderpoolip.CIDRToLabelValue(*pool.Spec.IPVersion, pool.Spec.Subnet)
			if nil != err {
				return nil, fmt.Errorf("failed to parse '%s' when allocating empty Auto-created IPPool '%v'", pool.Spec.Subnet, pool)
			}
			poolLabels[constant.LabelIPPoolCIDR] = cidrLabelValue
			pool.SetLabels(poolLabels)
		}

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

	// give the auto-created IPPool a symbol to explain whether it is IP number flexible or fixed.
	poolAnno := map[string]string{
		constant.AnnoSpiderSubnetPoolIPNumber: autoPoolProperty.AnnoPoolIPNumberVal,
	}
	pool.SetAnnotations(poolAnno)

	if operationCreate {
		log.Sugar().Infof("try to create IPPool '%v'", pool)
		err = sm.client.Create(ctx, pool)
		if nil != err {
			return nil, fmt.Errorf("failed to create auto-created IPPool %v: %w", pool, err)
		}
	} else {
		log.Sugar().Debugf("try to update IPPool %s with the new IPs %v from SpiderSubnet %s", pool.Name, ips, subnetName)
		err = sm.client.Update(ctx, pool)
		if nil != err {
			return nil, fmt.Errorf("failed to update auto-created IPPool %s with the new IPs %v from SpiderSubnet %s: %w", pool.Name, ips, subnetName, err)
		}
	}

	log.Sugar().Debugf("apply auto-created IPPool '%v' successfully", pool)
	return pool, nil
}

// preAllocateIPsFromSubnet will calculate the auto-created IPPool required IPs from corresponding SpiderSubnet and return it.
func (sm *subnetManager) preAllocateIPsFromSubnet(ctx context.Context, subnet *spiderpoolv2beta1.SpiderSubnet, pool *spiderpoolv2beta1.SpiderIPPool, ipVersion types.IPVersion, desiredIPNum int, podController types.PodTopController) ([]string, error) {
	log := logutils.FromContext(ctx)

	var beforeAllocatedIPs []net.IP
	ipNum := desiredIPNum

	subnetControlledIPPools, err := convert.UnmarshalSubnetAllocatedIPPools(subnet.Status.ControlledIPPools)
	if nil != err {
		return nil, fmt.Errorf("%w: failed to parse SpiderSubnet %s Status allocations: %v", constant.ErrWrongInput, subnet.Name, err)
	}
	if subnetControlledIPPools == nil {
		subnetControlledIPPools = make(spiderpoolv2beta1.PoolIPPreAllocations)
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
			poolAllocatedIPs, err := func() ([]net.IP, error) {
				poolIPAllocations, err := convert.UnmarshalIPPoolAllocatedIPs(pool.Status.AllocatedIPs)
				if nil != err {
					return nil, fmt.Errorf("%w: failed to parse IPPool %s Status AllocatedIPs: %v", constant.ErrWrongInput, pool.Name, err)
				}

				var ips []string
				for tmpIP := range poolIPAllocations {
					ips = append(ips, tmpIP)
				}
				return spiderpoolip.ParseIPRanges(ipVersion, ips)
			}()
			if nil != err {
				return nil, err
			}

			// If we have difference sets, which means the subnet updated its status successfully in the last shrink operation but the next ippool update operation failed.
			// In the situation, the ippool may allocate or release one of ips that subnet updated. So, we should correct the subnet status.
			if spiderpoolip.IsDiffIPSet(poolAllocatedIPs, subnetPoolIPs) {
				log.Sugar().Warnf("the last whole auto-created pool scale operation interrupted, try to correct SpiderSubnet %s status %s IP allocations", subnet.Name, pool.Name)
				poolTotalIPs, err := spiderpoolip.ParseIPRanges(ipVersion, pool.Spec.IPs)
				if nil != err {
					return nil, fmt.Errorf("%w: failed to parse IPPool %s Spec TotalIPs: %v", constant.ErrWrongInput, pool.Name, err)
				}
				if len(poolTotalIPs)-len(poolAllocatedIPs) >= len(poolTotalIPs)-desiredIPNum {
					freeIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, poolAllocatedIPs, true)
					discardedIPs := freeIPs[:len(poolTotalIPs)-desiredIPNum]
					newIPs := spiderpoolip.IPsDiffSet(poolTotalIPs, discardedIPs, false)
					newPoolSpecIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, newIPs)
					if nil != err {
						return nil, fmt.Errorf("%w: failed to convert ips to ipranges: %v", constant.ErrWrongInput, err)
					}
					subnetPoolAllocation.IPs = newPoolSpecIPRange
					subnetControlledIPPools[pool.Name] = subnetPoolAllocation
					marshalSubnetAllocatedIPPools, err := convert.MarshalSubnetAllocatedIPPools(subnetControlledIPPools)
					if nil != err {
						return nil, fmt.Errorf("%w: failed to marshal Subnet controlled IPPools %v: %v", constant.ErrWrongInput, subnetControlledIPPools, err)
					}
					subnet.Status.ControlledIPPools = marshalSubnetAllocatedIPPools
					totalCount, allocatedCount := subnetStatusCount(subnet)
					subnet.Status.TotalIPCount = &totalCount
					subnet.Status.AllocatedIPCount = &allocatedCount
					log.Sugar().Infof("try to correct SpiderSubnet %s status %s IP allocations with %v", subnet.Name, pool.Name, subnetControlledIPPools)

					err = sm.client.Status().Update(ctx, subnet)
					if nil != err {
						return nil, fmt.Errorf("failed to correct SpiderSubnet %s status %s IP allocations: %w", subnet.Name, pool.Name, err)
					}
					return newPoolSpecIPRange, nil
				}
				return nil, fmt.Errorf("failed to scale down IPPool %s IPs: %w", pool.Name, constant.ErrFreeIPsNotEnough)
			}

			log.Sugar().Debugf("match the last IPPool %s last allocated %v IP number from SpiderSubnet %s, just reuse it",
				pool.Name, subnetPoolAllocation.IPs, subnet.Name)
			return subnetPoolAllocation.IPs, nil
		} else if desiredIPNum < len(subnetPoolIPs) {
			log.Sugar().Infof("IPPool %s decresed its desired IP number from %d to %d", pool.Name, len(subnetPoolIPs), desiredIPNum)
			poolIPAllocations, err := convert.UnmarshalIPPoolAllocatedIPs(pool.Status.AllocatedIPs)
			if nil != err {
				return nil, fmt.Errorf("%w: failed to parse IPPool %s Status AllocatedIPs: %v", constant.ErrWrongInput, pool.Name, err)
			}

			// shrink: free IP number >= return IP Num
			// when it needs to scale down IP, enough IP is released to make sure it scale down successfully
			if len(subnetPoolIPs)-len(poolIPAllocations) >= len(subnetPoolIPs)-desiredIPNum {
				// exist auto pool allocated IPs
				poolAllocatedIPs, err := func() ([]net.IP, error) {
					var ips []string
					for tmpIP := range poolIPAllocations {
						ips = append(ips, tmpIP)
					}
					return spiderpoolip.ParseIPRanges(ipVersion, ips)
				}()
				if nil != err {
					return nil, fmt.Errorf("%w: failed to parse IP ranges '%v', error: %v", constant.ErrWrongInput, poolAllocatedIPs, err)
				}

				// free IPs
				freeIPs := spiderpoolip.IPsDiffSet(subnetPoolIPs, poolAllocatedIPs, true)
				discardedIPs := freeIPs[:len(subnetPoolIPs)-desiredIPNum]
				newIPs := spiderpoolip.IPsDiffSet(subnetPoolIPs, discardedIPs, false)
				poolIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, newIPs)
				if nil != err {
					return nil, fmt.Errorf("%w: failed to convert ips to ipranges: %v", constant.ErrWrongInput, err)
				}
				subnetControlledIPPools[pool.Name] = spiderpoolv2beta1.PoolIPPreAllocation{
					IPs:         poolIPRange,
					Application: ptr.To(applicationinformers.ApplicationNamespacedName(podController.AppNamespacedName)),
				}
				marshalSubnetAllocatedIPPools, err := convert.MarshalSubnetAllocatedIPPools(subnetControlledIPPools)
				if nil != err {
					return nil, fmt.Errorf("%w: failed to marshal Subnet controlled IPPools %v: %v", constant.ErrWrongInput, subnetControlledIPPools, err)
				}
				subnet.Status.ControlledIPPools = marshalSubnetAllocatedIPPools
				totalCount, allocatedCount := subnetStatusCount(subnet)
				subnet.Status.TotalIPCount = &totalCount
				subnet.Status.AllocatedIPCount = &allocatedCount
				log.Sugar().Infof("try to allocate IPPool %s newly IPs %v from SpiderSubnet %s in shrink situation", pool.Name, poolIPRange, subnet.Name)
				err = sm.client.Status().Update(ctx, subnet)
				if nil != err {
					return nil, fmt.Errorf("failed to allocate IPPool %s newsly IPs %v from SpiderSubnet %s in shrink situation: %w", pool.Name, poolIPRange, subnet.Name, err)
				}
				return poolIPRange, nil
			}
			return nil, fmt.Errorf("failed to scale down IPPool %s IPs: %w", pool.Name, constant.ErrFreeIPsNotEnough)
		} else {
			log.Sugar().Infof("IPPool %s increased its desired IP number from %d to %d, and the last allocation is %v",
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
	reservedIPs, err := sm.rIPManager.AssembleReservedIPs(ctx, ipVersion)
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
		return nil, fmt.Errorf("%w: failed to convert ips to ipranges: %v", constant.ErrWrongInput, err)
	}

	subnetControlledIPPools[pool.Name] = spiderpoolv2beta1.PoolIPPreAllocation{
		IPs:         allocateIPRange,
		Application: ptr.To(applicationinformers.ApplicationNamespacedName(podController.AppNamespacedName)),
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
		return nil, fmt.Errorf("failed to generate %d IPs from SpiderSubnet %s for IPPool %s: %w", desiredIPNum, subnet.Name, pool.Name, err)
	}

	log.Sugar().Infof("generated '%d' IPs '%v' from SpiderSubnet '%s' for IPPool %s", desiredIPNum, allocateIPRange, subnet.Name, pool.Name)
	return allocateIPRange, nil
}

func subnetStatusCount(subnet *spiderpoolv2beta1.SpiderSubnet) (totalCount, allocatedCount int64) {
	// total IP Count
	subnetTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)

	if subnet.Status.ControlledIPPools == nil {
		return 0, 0
	}
	var controlledIPPools spiderpoolv2beta1.PoolIPPreAllocations
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
