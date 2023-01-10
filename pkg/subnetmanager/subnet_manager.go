// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"errors"
	"fmt"
	"net"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var logger *zap.Logger

type subnetManager struct {
	config        *SubnetManagerConfig
	client        client.Client
	runtimeMgr    ctrl.Manager
	ipPoolManager ippoolmanagertypes.IPPoolManager
	reservedMgr   reservedipmanager.ReservedIPManager

	leader election.SpiderLeaseElector

	workQueue workqueue.RateLimitingInterface
}

func NewSubnetManager(c *SubnetManagerConfig, mgr ctrl.Manager, ipPoolManager ippoolmanagertypes.IPPoolManager, reservedIPMgr reservedipmanager.ReservedIPManager) (subnetmanagertypes.SubnetManager, error) {
	if c == nil {
		return nil, errors.New("subnet manager config must be specified")
	}
	if mgr == nil {
		return nil, errors.New("k8s manager must be specified")
	}
	if ipPoolManager == nil {
		return nil, errors.New("ippool manager must be specified")
	}
	if reservedIPMgr == nil {
		return nil, errors.New("reserved IP manager must be specified")
	}

	logger = logutils.Logger.Named("Subnet-Manager")

	return &subnetManager{
		config:        c,
		client:        mgr.GetClient(),
		runtimeMgr:    mgr,
		ipPoolManager: ipPoolManager,
		reservedMgr:   reservedIPMgr,
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

func (sm *subnetManager) GenerateIPsFromSubnetWhenScaleUpIP(ctx context.Context, subnetName string, pool *spiderpoolv1.SpiderIPPool, cursor bool) ([]string, error) {
	if pool.Status.AutoDesiredIPCount == nil {
		return nil, fmt.Errorf("%w: we can't generate IPs for the IPPool '%s' who doesn't have Status AutoDesiredIPCount", constant.ErrWrongInput, pool.Name)
	}

	subnet, err := sm.GetSubnetByName(ctx, subnetName)
	if nil != err {
		return nil, err
	}

	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't generate IPs from it", constant.ErrWrongInput, subnet.Name)
	}

	var ipVersion types.IPVersion
	if subnet.Spec.IPVersion != nil {
		ipVersion = *subnet.Spec.IPVersion
	} else {
		return nil, fmt.Errorf("%w: SpiderSubnet '%v' misses spec IP version", constant.ErrWrongInput, subnet)
	}

	log := logutils.FromContext(ctx)

	var beforeAllocatedIPs []net.IP

	desiredIPNum := int(*pool.Status.AutoDesiredIPCount)
	poolTotalIPs, err := spiderpoolip.AssembleTotalIPs(ipVersion, pool.Spec.IPs, pool.Spec.ExcludeIPs)
	if nil != err {
		return nil, fmt.Errorf("%w: failed to assemble IPPool '%s' total IPs, error: %v", constant.ErrWrongInput, pool.Name, err)
	}
	ipNum := desiredIPNum - len(poolTotalIPs)
	if ipNum <= 0 {
		return nil, fmt.Errorf("%w: IPPool '%s' status desiredIPNum is '%d' and total IP counts is '%d', we can't generate IPs for it",
			constant.ErrWrongInput, pool.Name, desiredIPNum, len(poolTotalIPs))
	}

	subnetPoolAllocation, ok := subnet.Status.ControlledIPPools[pool.Name]
	if ok {
		subnetPoolAllocatedIPs, err := spiderpoolip.ParseIPRanges(ipVersion, subnetPoolAllocation.IPs)
		if nil != err {
			return nil, fmt.Errorf("%w: failed to parse SpiderSubnet '%s' Status ControlledIPPool '%s' IPs '%v', error: %v",
				constant.ErrWrongInput, subnet.Name, pool.Name, subnetPoolAllocation.IPs, err)
		}

		// the subnetPoolAllocatedIPs is greater than pool total IP counts indicates that
		// the SpiderSubnet updated successfully but the IPPool failed to update in the last procession
		if len(subnetPoolAllocatedIPs) > len(poolTotalIPs) {
			lastAllocatedIPs := spiderpoolip.IPsDiffSet(subnetPoolAllocatedIPs, poolTotalIPs)
			log.Sugar().Warnf("SpiderSubnet '%s' Status ControlledIPPool '%s' has the allocated IPs '%v', try to re-use it!", subnetName, pool.Name, lastAllocatedIPs)
			if len(lastAllocatedIPs) == desiredIPNum-len(poolTotalIPs) {
				// last allocated IPs is same with the current allocation request
				return spiderpoolip.ConvertIPsToIPRanges(ipVersion, lastAllocatedIPs)
			} else if len(lastAllocatedIPs) > desiredIPNum-len(poolTotalIPs) {
				// last allocated IPs is greater than the current allocation request,
				// we will update the SpiderSubnet status correctly in next IPPool webhook SpiderSubnet update procession
				return spiderpoolip.ConvertIPsToIPRanges(ipVersion, lastAllocatedIPs[:desiredIPNum-len(poolTotalIPs)])
			} else {
				// last allocated IPs less than the current allocation request,
				// we can re-use the allocated IPs and generate some another IPs
				beforeAllocatedIPs = lastAllocatedIPs
				ipNum = desiredIPNum - len(poolTotalIPs) - len(lastAllocatedIPs)
			}
		}
	}

	freeIPs, err := controllers.GenSubnetFreeIPs(subnet)
	if nil != err {
		return nil, fmt.Errorf("failed to generate SpiderSubnet '%s' free IPs, error: %v", subnetName, err)
	}

	// filter reserved IPs
	reservedIPList, err := sm.reservedMgr.ListReservedIPs(ctx)
	if nil != err {
		return nil, fmt.Errorf("failed to list reservedIPs, error: %v", err)
	}

	reservedIPs, err := reservedipmanager.AssembleReservedIPs(ipVersion, reservedIPList)
	if nil != err {
		return nil, fmt.Errorf("%w: failed to filter reservedIPs '%v' by IP version '%d', error: %v",
			constant.ErrWrongInput, reservedIPs, ipVersion, err)
	}

	if len(reservedIPs) != 0 {
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, reservedIPs)
	}

	if len(pool.Spec.ExcludeIPs) != 0 {
		excludeIPs, err := spiderpoolip.ParseIPRanges(ipVersion, pool.Spec.ExcludeIPs)
		if nil != err {
			return nil, fmt.Errorf("failed to parse exclude IP ranges '%v', error: %v", pool.Spec.ExcludeIPs, err)
		}
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, excludeIPs)
	}

	// check the filtered subnet free IP number is enough or not
	if len(freeIPs) < ipNum {
		return nil, fmt.Errorf("insufficient subnet FreeIPs, required '%d' but only left '%d'", ipNum, len(freeIPs))
	}

	allocateIPs := make([]net.IP, 0, ipNum)
	if cursor {
		allocateIPs = append(allocateIPs, freeIPs[:ipNum]...)
	} else {
		allocateIPs = append(allocateIPs, freeIPs[len(freeIPs)-ipNum:]...)
	}

	// re-use the last allocated IPs
	if len(beforeAllocatedIPs) != 0 {
		allocateIPs = append(allocateIPs, beforeAllocatedIPs...)
	}

	allocateIPRange, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, allocateIPs)
	if nil != err {
		return nil, err
	}

	logger.Sugar().Infof("generated '%d' IPs '%v' from SpiderSubnet '%s'", ipNum, allocateIPRange, subnet.Name)

	return allocateIPRange, nil

}

// AllocateEmptyIPPool will create an empty IPPool and mark the status.AutoDesiredIPCount
// notice: this function only serves for auto-created IPPool
func (sm *subnetManager) AllocateEmptyIPPool(ctx context.Context, subnetName string, appKind string, app metav1.Object,
	podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) error {
	if len(subnetName) == 0 {
		return fmt.Errorf("%w: spider subnet name must be specified", constant.ErrWrongInput)
	}
	if ipNum < 0 {
		return fmt.Errorf("%w: the required IP numbers '%d' is invalid", constant.ErrWrongInput, ipNum)
	}

	subnet, err := sm.GetSubnetByName(ctx, subnetName)
	if nil != err {
		return err
	}

	if subnet.DeletionTimestamp != nil {
		return fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't create a corresponding IPPool", constant.ErrWrongInput, subnet.Name)
	}

	poolLabels := map[string]string{
		constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
		constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
		constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
		constant.LabelIPPoolInterface:           ifName,
	}

	if ipVersion == constant.IPv4 {
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV4
	} else {
		poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV6
	}

	if reclaimIPPool {
		poolLabels[constant.LabelIPPoolReclaimIPPool] = constant.True
	}

	sp := &spiderpoolv1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:   controllers.SubnetPoolName(appKind, app.GetNamespace(), app.GetName(), ipVersion, ifName, app.GetUID()),
			Labels: poolLabels,
		},
		Spec: spiderpoolv1.IPPoolSpec{
			Subnet:      subnet.Spec.Subnet,
			Gateway:     subnet.Spec.Gateway,
			Vlan:        subnet.Spec.Vlan,
			Routes:      subnet.Spec.Routes,
			PodAffinity: podSelector,
		},
	}

	logger.Sugar().Infof("try to create IPPool '%v'", sp)
	err = sm.ipPoolManager.CreateIPPool(ctx, sp)
	if nil != err {
		return err
	}

	logger.Sugar().Infof("try to update IPPool '%v' status DesiredIPNumber '%d'", sp, ipNum)
	err = sm.ipPoolManager.UpdateDesiredIPNumber(ctx, sp, ipNum)
	if nil != err {
		return err
	}

	return nil
}

// CheckScaleIPPool will fetch some IPs from the specified subnet manager to expand the pool IPs
func (sm *subnetManager) CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetName string, ipNum int) error {
	if pool == nil {
		return fmt.Errorf("%w: IPPool must be specified", constant.ErrWrongInput)
	}
	if ipNum <= 0 {
		return fmt.Errorf("%w: assign IP number '%d' is invalid", constant.ErrWrongInput, ipNum)
	}

	needUpdate := false
	if pool.Status.AutoDesiredIPCount == nil {
		// no desired IP number annotation
		needUpdate = true
	} else {
		// ignore it if they are equal
		if *pool.Status.AutoDesiredIPCount == int64(ipNum) {
			logger.Sugar().Debugf("no need to scale subnet '%s' IPPool '%s'", subnetName, pool.Name)
			return nil
		}

		// not equal
		needUpdate = true
	}

	if needUpdate {
		logger.Sugar().Infof("try to update IPPool '%s' status DesiredIPNumber to '%d'", pool.Name, ipNum)
		err := sm.ipPoolManager.UpdateDesiredIPNumber(ctx, pool, ipNum)
		if nil != err {
			return err
		}
	}

	return nil
}
