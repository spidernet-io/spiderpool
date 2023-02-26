// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"net"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type SubnetManager interface {
	GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error)
	AllocateEmptyIPPool(ctx context.Context, subnetMgrName string, podController types.PodTopController, podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error)
	CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetManagerName string, ipNum int) error
	AllocateIPPool(ctx context.Context, subnetName string, podController types.PodTopController, podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error)
	ReconcileAutoIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetName string, podController types.PodTopController, podSelector *metav1.LabelSelector, ipNum int, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error)
}

type subnetManager struct {
	config        SubnetManagerConfig
	client        client.Client
	ipPoolManager ippoolmanager.IPPoolManager
	reservedIPMgr reservedipmanager.ReservedIPManager
	Scheme        *runtime.Scheme
}

func NewSubnetManager(config SubnetManagerConfig, client client.Client, ipPoolManager ippoolmanager.IPPoolManager, reservedIPMgr reservedipmanager.ReservedIPManager, scheme *runtime.Scheme) (SubnetManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if ipPoolManager == nil {
		return nil, fmt.Errorf("ippool manager %w", constant.ErrMissingRequiredParam)
	}
	if scheme == nil {
		return nil, fmt.Errorf("scheme %w", constant.ErrMissingRequiredParam)
	}

	return &subnetManager{
		config:        setDefaultsForSubnetManagerConfig(config),
		client:        client,
		ipPoolManager: ipPoolManager,
		reservedIPMgr: reservedIPMgr,
		Scheme:        scheme,
	}, nil
}

func (sm *subnetManager) GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error) {
	var subnet spiderpoolv1.SpiderSubnet
	if err := sm.client.Get(ctx, apitypes.NamespacedName{Name: subnetName}, &subnet); err != nil {
		return nil, err
	}

	return &subnet, nil
}

func (sm *subnetManager) ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error) {
	var subnetList spiderpoolv1.SpiderSubnetList
	if err := sm.client.List(ctx, &subnetList, opts...); err != nil {
		return nil, err
	}

	return &subnetList, nil
}

// AllocateEmptyIPPool will create an empty IPPool and mark the status.AutoDesiredIPCount
// notice: this function only serves for auto-created IPPool
func (sm *subnetManager) AllocateEmptyIPPool(ctx context.Context, subnetName string, podController types.PodTopController,
	podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error) {
	if len(subnetName) == 0 {
		return nil, fmt.Errorf("%w: spider subnet name must be specified", constant.ErrWrongInput)
	}
	if ipNum < 0 {
		return nil, fmt.Errorf("%w: the required IP numbers '%d' is invalid", constant.ErrWrongInput, ipNum)
	}

	log := logutils.FromContext(ctx)
	subnet, err := sm.GetSubnetByName(ctx, subnetName)
	if nil != err {
		return nil, err
	}

	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't create a corresponding IPPool",
			constant.ErrWrongInput, subnet.Name)
	}

	sp := &spiderpoolv1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllers.SubnetPoolName(podController.Kind, podController.Namespace, podController.Name, ipVersion, ifName, podController.UID),
		},
		Spec: spiderpoolv1.IPPoolSpec{
			Subnet:      subnet.Spec.Subnet,
			Gateway:     subnet.Spec.Gateway,
			Vlan:        subnet.Spec.Vlan,
			Routes:      subnet.Spec.Routes,
			PodAffinity: podSelector,
		},
	}

	poolLabels := map[string]string{
		constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
		constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
		constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
		constant.LabelIPPoolInterface:           ifName,
	}

	if ipVersion == constant.IPv4 {
		sp.Spec.IPVersion = pointer.Int64(constant.IPv4)
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV4
	} else {
		sp.Spec.IPVersion = pointer.Int64(constant.IPv6)
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV6
	}

	cidrLabelValue, err := spiderpoolip.CIDRToLabelValue(*sp.Spec.IPVersion, sp.Spec.Subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to parse '%s' when allocating empty Auto-created IPPool '%v'", sp.Spec.Subnet, sp)
	}
	poolLabels[constant.LabelIPPoolCIDR] = cidrLabelValue

	if reclaimIPPool {
		poolLabels[constant.LabelIPPoolReclaimIPPool] = constant.True
	}
	sp.Labels = poolLabels

	err = ctrl.SetControllerReference(subnet, sp, sm.Scheme)
	if nil != err {
		return nil, fmt.Errorf("failed to set SpiderIPPool %s owner reference with SpiderSubnet %s: %v", sp.Name, subnetName, err)
	}

	timeRecorder := metric.NewTimeRecorder()
	defer func() {
		// Time taken for once Auto-created IPPool creation.
		creationDuration := timeRecorder.SinceInSeconds()
		metric.AutoPoolCreationDurationConstruct.RecordAutoPoolCreationDuration(ctx, creationDuration)
		log.Sugar().Infof("Auto-created IPPool '%s' creation duration: %v", sp.Name, creationDuration)
	}()
	log.Sugar().Infof("try to create IPPool '%v'", sp)
	err = sm.client.Create(ctx, sp)
	if nil != err {
		return nil, err
	}

	log.Sugar().Infof("try to update IPPool '%v' status DesiredIPNumber '%d'", sp, ipNum)
	err = sm.ipPoolManager.UpdateDesiredIPNumber(ctx, sp, ipNum)
	if nil != err {
		return nil, err
	}
	log.Sugar().Infof("create and mark IPPool '%v' successfully", sp)

	return sp, nil
}

// CheckScaleIPPool will fetch some IPs from the specified subnet manager to expand the pool IPs
func (sm *subnetManager) CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetName string, ipNum int) error {
	if pool == nil {
		return fmt.Errorf("%w: IPPool must be specified", constant.ErrWrongInput)
	}
	if ipNum <= 0 {
		return fmt.Errorf("%w: assign IP number '%d' is invalid", constant.ErrWrongInput, ipNum)
	}

	log := logutils.FromContext(ctx)
	needUpdate := false
	if pool.Status.AutoDesiredIPCount == nil {
		// no desired IP number annotation
		needUpdate = true
	} else {
		// ignore it if they are equal
		if *pool.Status.AutoDesiredIPCount == int64(ipNum) {
			log.Sugar().Debugf("no need to scale subnet '%s' IPPool '%s'", subnetName, pool.Name)
			return nil
		}

		// not equal
		needUpdate = true
	}

	if needUpdate {
		log.Sugar().Infof("try to update IPPool '%s' status DesiredIPNumber to '%d'", pool.Name, ipNum)
		err := sm.ipPoolManager.UpdateDesiredIPNumber(ctx, pool, ipNum)
		if nil != err {
			return err
		}
	}

	return nil
}

func (sm *subnetManager) AllocateIPPool(ctx context.Context, subnetName string, podController types.PodTopController,
	podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error) {
	if len(subnetName) == 0 {
		return nil, fmt.Errorf("%w: spider subnet name must be specified", constant.ErrWrongInput)
	}
	if ipNum < 0 {
		return nil, fmt.Errorf("%w: the required IP numbers '%d' is invalid", constant.ErrWrongInput, ipNum)
	}

	log := logutils.FromContext(ctx)
	subnet, err := sm.GetSubnetByName(ctx, subnetName)
	if nil != err {
		return nil, fmt.Errorf("failed to get SpiderSubnet %s, error: %w", subnetName, err)
	}
	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't create an auto-created IPPool from it", constant.ErrWrongInput, subnet.Name)
	}

	sp := &spiderpoolv1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllers.SubnetPoolName(podController.Kind, podController.Namespace, podController.Name, ipVersion, ifName, podController.UID),
		},
		Spec: spiderpoolv1.IPPoolSpec{
			Subnet:      subnet.Spec.Subnet,
			Gateway:     subnet.Spec.Gateway,
			Vlan:        subnet.Spec.Vlan,
			Routes:      subnet.Spec.Routes,
			PodAffinity: podSelector,
		},
	}

	log.Sugar().Infof("try to pre-allocate IPs from subnet %s for auto-create IPPool %s with %d IPs", subnetName, sp.Name, ipNum)
	ips, err := sm.preAllocateIPsFromSubnet(ctx, subnet, sp.Name, ipNum)
	if nil != err {
		return nil, err
	}
	sp.Spec.IPs = ips

	poolLabels := map[string]string{
		constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
		constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
		constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
		constant.LabelIPPoolInterface:           ifName,
	}
	if ipVersion == constant.IPv4 {
		sp.Spec.IPVersion = pointer.Int64(constant.IPv4)
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV4
	} else {
		sp.Spec.IPVersion = pointer.Int64(constant.IPv6)
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV6
	}
	cidrLabelValue, err := spiderpoolip.CIDRToLabelValue(*sp.Spec.IPVersion, sp.Spec.Subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to parse '%s' when allocating empty Auto-created IPPool '%v'", sp.Spec.Subnet, sp)
	}
	poolLabels[constant.LabelIPPoolCIDR] = cidrLabelValue
	if reclaimIPPool {
		poolLabels[constant.LabelIPPoolReclaimIPPool] = constant.True
	}
	sp.Labels = poolLabels

	// set owner reference
	err = ctrl.SetControllerReference(subnet, sp, sm.Scheme)
	if nil != err {
		return nil, fmt.Errorf("failed to set SpiderIPPool %s owner reference with SpiderSubnet %s: %v", sp.Name, subnetName, err)
	}

	log.Sugar().Infof("try to create IPPool '%v'", sp)
	err = sm.client.Create(ctx, sp)
	if nil != err {
		if apierrors.IsAlreadyExists(err) {
			sp, err = sm.ipPoolManager.GetIPPoolByName(ctx, sp.Name)
			if nil != err {
				return nil, fmt.Errorf("failed to fetch the previous created IPPool %s: %w", sp.Name, err)
			}
			log.Sugar().Warnf("IPPool already exists, try get get the new updated one %v", sp)
		} else {
			return nil, fmt.Errorf("failed to create IPPool %s: %w", sp.Name, err)
		}
	}

	log.Sugar().Infof("try to update IPPool '%v' status DesiredIPNumber '%d'", sp, ipNum)
	err = sm.ipPoolManager.UpdateDesiredIPNumber(ctx, sp, ipNum)
	if nil != err {
		return nil, err
	}
	log.Sugar().Infof("create and mark IPPool '%v' successfully", sp)

	return sp, nil
}

// preAllocateIPsFromSubnet will calculate the auto-created IPPool required IPs from corresponding SpiderSubnet and return it.
func (sm *subnetManager) preAllocateIPsFromSubnet(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet, poolName string, desiredIPNum int) ([]string, error) {
	log := logutils.FromContext(ctx)

	var ipVersion types.IPVersion
	if subnet.Spec.IPVersion != nil {
		ipVersion = *subnet.Spec.IPVersion
	} else {
		return nil, fmt.Errorf("%w: SpiderSubnet '%v' misses spec IP version", constant.ErrWrongInput, subnet)
	}

	var beforeAllocatedIPs []net.IP
	ipNum := desiredIPNum
	subnetPoolAllocation, ok := subnet.Status.ControlledIPPools[poolName]
	if ok {
		subnetPoolAllocatedIPs, err := spiderpoolip.ParseIPRanges(ipVersion, subnetPoolAllocation.IPs)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse SpiderSubnet '%s' Status ControlledIPPool '%s' IPs '%v', error: %v",
				constant.ErrWrongInput, subnet.Name, poolName, subnetPoolAllocation.IPs, err)
		}

		if len(subnetPoolAllocatedIPs) == desiredIPNum {
			log.Sugar().Warnf("============fetch the ippool %s last allocated IPs %v from subnet %s", poolName, subnetPoolAllocation.IPs, subnet.Name)
			return subnetPoolAllocation.IPs, nil
		} else if len(subnetPoolAllocatedIPs) > desiredIPNum {
			// 上次已经记录，结果后续操作失败，接着立刻扩缩容，要的ip少了些
			subnetPoolAllocatedIPs = subnetPoolAllocatedIPs[:desiredIPNum:desiredIPNum]
			allocateIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, subnetPoolAllocatedIPs)
			if nil != err {
				return nil, err
			}
			subnet.Status.ControlledIPPools[poolName] = spiderpoolv1.PoolIPPreAllocation{IPs: allocateIPRange}
			return allocateIPRange, sm.client.Status().Update(ctx, subnet)
		} else {
			// 上次已经记录，结果后续操作失败，接着立刻扩缩容，要的ip更多了
			beforeAllocatedIPs = subnetPoolAllocatedIPs
			ipNum = len(subnetPoolAllocatedIPs) - desiredIPNum
		}
	}

	freeIPs, err := controllers.GenSubnetFreeIPs(subnet)
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

	// TODO(Icarus9913): 更新totalIPCount
	controlledIPPools := spiderpoolv1.PoolIPPreAllocations{}
	for tmpPoolName, poolAllocation := range subnet.Status.ControlledIPPools {
		controlledIPPools[tmpPoolName] = poolAllocation
	}
	controlledIPPools[poolName] = spiderpoolv1.PoolIPPreAllocation{IPs: allocateIPRange}
	subnet.Status.ControlledIPPools = controlledIPPools

	totalCount, allocatedCount := totalIPCountAndAllocatedIPCount(subnet)
	subnet.Status.TotalIPCount = &totalCount
	subnet.Status.AllocatedIPCount = &allocatedCount

	err = sm.client.Status().Update(ctx, subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to generate %d IPs from SpiderSubnet %s for IPPool %s: %w", desiredIPNum, subnet.Name, poolName, err)
	}

	log.Sugar().Infof("generated '%d' IPs '%v' from SpiderSubnet '%s' for IPPool %s", desiredIPNum, allocateIPRange, subnet.Name, poolName)
	return allocateIPRange, nil
}

func totalIPCountAndAllocatedIPCount(subnet *spiderpoolv1.SpiderSubnet) (totalCount, allocatedCount int64) {
	// total IP Count
	subnetTotalIPs, _ := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)

	// allocated IP Count
	var allocatedIPCount int64
	for _, poolAllocation := range subnet.Status.ControlledIPPools {
		tmpIPs, _ := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, poolAllocation.IPs)
		allocatedIPCount += int64(len(tmpIPs))
	}
	return int64(len(subnetTotalIPs)), allocatedIPCount
}

/*
1. 找subnet要ip，且更新subnet的status
2. 更新池
  - 池不在，则带上ip去创建池
  - 池在，则更新其ip
*/
func (sm *subnetManager) ReconcileAutoIPPool(ctx context.Context,
	pool *spiderpoolv1.SpiderIPPool,
	subnetName string,
	podController types.PodTopController,
	podSelector *metav1.LabelSelector,
	ipNum int,
	reclaimIPPool bool,
	ifName string) (*spiderpoolv1.SpiderIPPool, error) {
	if len(subnetName) == 0 {
		return nil, fmt.Errorf("%w: spider subnet name must be specified", constant.ErrWrongInput)
	}
	if ipNum < 0 {
		return nil, fmt.Errorf("%w: the required IP numbers '%d' is invalid", constant.ErrWrongInput, ipNum)
	}
	log := logutils.FromContext(ctx)

	subnet, err := sm.GetSubnetByName(ctx, subnetName)
	if nil != err {
		return nil, fmt.Errorf("failed to get SpiderSubnet %s, error: %w", subnetName, err)
	}
	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't create an auto-created IPPool from it", constant.ErrWrongInput, subnet.Name)
	}

	ipVersion := constant.IPv4
	if subnet.Spec.IPVersion != nil {
		ipVersion = *subnet.Spec.IPVersion
	}

	if pool == nil {
		pool = &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: controllers.SubnetPoolName(podController.Kind, podController.Namespace, podController.Name, ipVersion, ifName, podController.UID),
			},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet:      subnet.Spec.Subnet,
				Gateway:     subnet.Spec.Gateway,
				Vlan:        subnet.Spec.Vlan,
				Routes:      subnet.Spec.Routes,
				PodAffinity: podSelector,
			},
		}
	}

	log.Sugar().Infof("try to pre-allocate IPs from subnet %s for auto-create IPPool %s with %d IPs", subnetName, pool.Name, ipNum)
	ips, err := sm.preAllocateIPsFromSubnet(ctx, subnet, pool.Name, ipNum)
	if nil != err {
		return nil, err
	}
	pool.Spec.IPs = ips

	poolLabels := map[string]string{
		constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
		constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
		constant.LabelIPPoolOwnerApplicationUID: string(podController.UID),
		constant.LabelIPPoolInterface:           ifName,
	}
	if ipVersion == constant.IPv4 {
		pool.Spec.IPVersion = pointer.Int64(constant.IPv4)
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV4
	} else {
		pool.Spec.IPVersion = pointer.Int64(constant.IPv6)
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV6
	}
	cidrLabelValue, err := spiderpoolip.CIDRToLabelValue(*pool.Spec.IPVersion, pool.Spec.Subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to parse '%s' when allocating empty Auto-created IPPool '%v'", pool.Spec.Subnet, pool)
	}
	poolLabels[constant.LabelIPPoolCIDR] = cidrLabelValue
	if reclaimIPPool {
		poolLabels[constant.LabelIPPoolReclaimIPPool] = constant.True
	}
	pool.Labels = poolLabels

	// set owner reference
	err = ctrl.SetControllerReference(subnet, pool, sm.Scheme)
	if nil != err {
		return nil, fmt.Errorf("failed to set SpiderIPPool %s owner reference with SpiderSubnet %s: %v", pool.Name, subnetName, err)
	}

	log.Sugar().Infof("try to create IPPool '%v'", pool)
	err = sm.client.Create(ctx, pool)
	if nil != err {
		if apierrors.IsAlreadyExists(err) {
			pool, err = sm.ipPoolManager.GetIPPoolByName(ctx, pool.Name)
			if nil != err {
				return nil, fmt.Errorf("failed to fetch the previous created IPPool %s: %w", pool.Name, err)
			}
			log.Sugar().Warnf("IPPool already exists, try get get the new updated one %v", pool)
		} else {
			return nil, fmt.Errorf("failed to create IPPool %s: %w", pool.Name, err)
		}
	}

	log.Sugar().Infof("try to update IPPool '%v' status DesiredIPNumber '%d'", pool, ipNum)
	err = sm.ipPoolManager.UpdateDesiredIPNumber(ctx, pool, ipNum)
	if nil != err {
		return nil, err
	}
	log.Sugar().Infof("create and mark IPPool '%v' successfully", pool)

	return pool, nil
}
