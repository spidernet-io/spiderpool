// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	ippoolmanagertypes "github.com/spidernet-io/spiderpool/pkg/ippoolmanager/types"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var logger *zap.Logger
var ErrMaxRetries = fmt.Errorf("over max retries")

type subnetManager struct {
	config        *SubnetConfig
	client        client.Client
	runtimeMgr    ctrl.Manager
	ipPoolManager ippoolmanagertypes.IPPoolManager
	leader        election.SpiderLeaseElector

	innerCtx context.Context
}

func NewSubnetManager(c *SubnetConfig, mgr ctrl.Manager, ipPoolManager ippoolmanagertypes.IPPoolManager) (subnetmanagertypes.SubnetManager, error) {
	if c == nil {
		return nil, errors.New("subnet manager config must be specified")
	}
	if mgr == nil {
		return nil, errors.New("k8s manager must be specified")
	}
	if ipPoolManager == nil {
		return nil, errors.New("ippool manager must be specified")
	}

	logger = logutils.Logger.Named("Subnet-Manager")

	return &subnetManager{
		config:        c,
		client:        mgr.GetClient(),
		runtimeMgr:    mgr,
		ipPoolManager: ipPoolManager,
	}, nil
}

func (sm *subnetManager) GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error) {
	var subnet spiderpoolv1.SpiderSubnet
	if err := sm.client.Get(ctx, apitypes.NamespacedName{Name: subnetName}, &subnet); err != nil {
		return nil, fmt.Errorf("failed to get subnet '%s', error: %v", subnetName, err)
	}

	return &subnet, nil
}

func (sm *subnetManager) ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error) {
	subnetList := &spiderpoolv1.SpiderSubnetList{}
	if err := sm.client.List(ctx, subnetList, opts...); err != nil {
		return nil, fmt.Errorf("failed to list subnet with options '%v', error: %v", opts, err)
	}

	return subnetList, nil
}

func (sm *subnetManager) UpdateSubnetStatusOnce(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error {
	if err := sm.client.Status().Update(ctx, subnet); err != nil {
		return fmt.Errorf("failed to update subnet '%s', error: %v", subnet.Name, err)
	}

	return nil
}

func (sm *subnetManager) GenerateIPsFromSubnet(ctx context.Context, subnetMgrName string, ipNum int) ([]string, error) {
	var allocateIPRange []string

	subnet, err := sm.GetSubnetByName(ctx, subnetMgrName)
	if nil != err {
		return nil, err
	}

	var ipVersion types.IPVersion
	if subnet.Spec.IPVersion != nil {
		ipVersion = *subnet.Spec.IPVersion
	} else {
		return nil, fmt.Errorf("subnet '%v' misses spec IP version", subnet)
	}

	freeIPs, err := spiderpoolip.ParseIPRanges(ipVersion, subnet.Status.FreeIPs)
	if nil != err {
		return nil, err
	}

	if len(freeIPs) < ipNum {
		return nil, fmt.Errorf("insufficient subnet FreeIPs, required '%d' but only left '%d'", ipNum, len(freeIPs))
	}

	allocateIPs := make([]net.IP, ipNum)
	for j := 0; j < ipNum; j++ {
		allocateIPs[j] = freeIPs[j]
	}

	allocateIPRange, err = spiderpoolip.ConvertIPsToIPRanges(ipVersion, allocateIPs)
	if nil != err {
		return nil, err
	}

	logger.Sugar().Infof("generated '%d' IPs '%v' from subnet manager '%s'", ipNum, allocateIPRange, subnetMgrName)

	return allocateIPRange, nil
}

func (sm *subnetManager) AllocateIPPool(ctx context.Context, subnetMgrName string, appKind string, app metav1.Object, controllerLabels map[string]string,
	ipNum int, ipVersion types.IPVersion) error {
	if len(subnetMgrName) == 0 {
		return fmt.Errorf("subnet manager name must be specified")
	}
	if ipNum < 0 {
		return fmt.Errorf("the required IP numbers '%d' is invalid", ipNum)
	}

	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= sm.config.MaxConflictRetrys; i++ {
		subnet, err := sm.GetSubnetByName(ctx, subnetMgrName)
		if nil != err {
			if i == sm.config.MaxConflictRetrys {
				return fmt.Errorf("%w: %v", ErrMaxRetries, err)
			}

			logger.Error(err.Error())
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		poolIPs, err := sm.GenerateIPsFromSubnet(ctx, subnetMgrName, ipNum)
		if nil != err {
			if i == sm.config.MaxConflictRetrys {
				return fmt.Errorf("%w: failed to generate IPs from subnet '%s', error: %v", ErrMaxRetries, subnetMgrName, err)
			}

			logger.Sugar().Errorf("failed to generate IPs from subnet '%s', error: %v", subnetMgrName, err)
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		poolLabels := map[string]string{
			constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
			constant.LabelIPPoolOwnerApplication:    AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
			constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
		}
		if ipVersion == constant.IPv4 {
			poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV4
		} else {
			poolLabels[constant.LabelIPPoolVersion] = constant.LabelIPPoolVersionV6
		}

		sp := &spiderpoolv1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:   SubnetPoolName(appKind, app.GetNamespace(), app.GetName(), ipVersion),
				Labels: poolLabels,
			},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet:  subnet.Spec.Subnet,
				IPs:     poolIPs,
				Gateway: subnet.Spec.Gateway,
				Vlan:    subnet.Spec.Vlan,
				Routes:  subnet.Spec.Routes,
				PodAffinity: &metav1.LabelSelector{
					MatchLabels: controllerLabels,
				},
			},
		}

		logger.Sugar().Infof("try to create IPPool '%v'", sp)
		err = sm.ipPoolManager.CreateIPPool(ctx, sp)
		if nil != err {
			if i == sm.config.MaxConflictRetrys {
				return fmt.Errorf("%w, failed to create IPPool, error: %v", ErrMaxRetries, err)
			}

			logger.Error(err.Error())
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		break
	}

	return nil
}

// RetrieveIPPool try to retrieve IPPools by app label
func (sm *subnetManager) RetrieveIPPool(ctx context.Context, appKind string, app metav1.Object, subnetMgrName string, ipVersion types.IPVersion) (pool *spiderpoolv1.SpiderIPPool, err error) {
	matchLabel := client.MatchingLabels{
		constant.LabelIPPoolOwnerSpiderSubnet:   subnetMgrName,
		constant.LabelIPPoolOwnerApplication:    AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
		constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
	}

	retrievePool := func(labelIPPoolVersion string) (*spiderpoolv1.SpiderIPPool, error) {
		matchLabel[constant.LabelIPPoolVersion] = labelIPPoolVersion
		poolList, err := sm.ipPoolManager.ListIPPools(ctx, matchLabel)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		if len(poolList.Items) == 0 {
			return nil, nil
		}
		if len(poolList.Items) != 1 {
			return nil, fmt.Errorf("unmatch IPPool list items number '%d' with labels '%v'", len(poolList.Items), matchLabel)
		}

		pool := poolList.Items[0]
		return &pool, nil
	}

	if ipVersion == constant.IPv4 {
		pool, err = retrievePool(constant.LabelIPPoolVersionV4)
	} else {
		pool, err = retrievePool(constant.LabelIPPoolVersionV6)
	}
	if nil != err {
		return nil, err
	}

	if pool != nil && pool.DeletionTimestamp != nil {
		return nil, fmt.Errorf("IPPool '%s' is deleting", pool.Name)
	}
	return
}

// CheckScaleIPPool will fetch some IPs from the specified subnet managger to expand the pool IPs
func (sm *subnetManager) CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetMgrName string, ipNum int) error {
	if pool == nil {
		return fmt.Errorf("IPPool must be specified")
	}
	if ipNum <= 0 {
		return fmt.Errorf("assign IP number '%d' is invalid", ipNum)
	}

	// TODO (Icarus9913): check no pointer here?
	ips, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
	if nil != err {
		return fmt.Errorf("failed to parse IPPool '%s' IPs, error: %v", pool.Name, err)
	}

	// no need to expand
	if len(ips) == ipNum {
		logger.Sugar().Debugf("no need to scale subnet '%s' IPPool '%s'", subnetMgrName, pool.Name)
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	for i := 0; i < sm.config.MaxConflictRetrys; i++ {
		ipsFromSubnet, err := sm.GenerateIPsFromSubnet(ctx, subnetMgrName, ipNum-len(ips))
		if nil != err {
			if i == sm.config.MaxConflictRetrys {
				return fmt.Errorf("%w: failed to generate IPs from subnet '%s', error: %v", ErrMaxRetries, subnetMgrName, err)
			}

			logger.Sugar().Errorf("failed to generate IPs from subnet '%s', error: %v", subnetMgrName, err)
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		logger.Sugar().Infof("try to scale IPPool '%s' IPs '%v'", pool.Name, ipsFromSubnet)
		// update IPPool
		err = sm.ipPoolManager.ScaleIPPoolIPs(logutils.IntoContext(ctx, logger), pool.Name, ipsFromSubnet)
		if nil != err {
			if i == sm.config.MaxConflictRetrys {
				return fmt.Errorf("%w: %v", ErrMaxRetries, err)
			}

			logger.Error(err.Error())
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		break
	}

	return nil
}
