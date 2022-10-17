// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sort"
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
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var logger *zap.Logger
var ErrMaxRetries = fmt.Errorf("over max retries")

type subnetManager struct {
	config        *SubnetManagerConfig
	client        client.Client
	runtimeMgr    ctrl.Manager
	ipPoolManager ippoolmanagertypes.IPPoolManager
	reservedMgr   reservedipmanager.ReservedIPManager

	leader election.SpiderLeaseElector

	innerCtx context.Context
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
	subnetList := &spiderpoolv1.SpiderSubnetList{}
	if err := sm.client.List(ctx, subnetList, opts...); err != nil {
		return nil, err
	}

	return subnetList, nil
}

func (sm *subnetManager) GenerateIPsFromSubnet(ctx context.Context, subnetMgrName string, ipNum int, excludeIPRanges []string) ([]string, error) {
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

	freeIPs, err := controllers.GenSubnetFreeIPs(subnet)
	if nil != err {
		return nil, err
	}

	// filter reserved IPs
	reservedIPList, err := sm.reservedMgr.ListReservedIPs(ctx)
	if nil != err {
		return nil, fmt.Errorf("failed to list reservedIPs, error: %v", err)
	}

	reservedIPs, err := sm.reservedMgr.GetReservedIPsByIPVersion(ctx, ipVersion, reservedIPList)
	if nil != err {
		return nil, fmt.Errorf("%w: failed to filter reservedIPs '%v' by IP version '%d', error: %v",
			constant.ErrWrongInput, reservedIPs, ipVersion, err)
	}

	if len(reservedIPs) != 0 {
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, reservedIPs)
	}

	if len(excludeIPRanges) != 0 {
		excludeIPs, err := spiderpoolip.ParseIPRanges(ipVersion, excludeIPRanges)
		if nil != err {
			return nil, fmt.Errorf("failed to parse exclude IP ranges '%v', error: %v", excludeIPRanges, err)
		}
		freeIPs = spiderpoolip.IPsDiffSet(freeIPs, excludeIPs)
	}

	// check the filtered subnet free IP number is enough or not
	if len(freeIPs) < ipNum {
		return nil, fmt.Errorf("insufficient subnet FreeIPs, required '%d' but only left '%d'", ipNum, len(freeIPs))
	}

	// sort freeIPs
	sort.Slice(freeIPs, func(i, j int) bool {
		return bytes.Compare(freeIPs[i].To16(), freeIPs[j].To16()) < 0
	})

	allocateIPs := make([]net.IP, ipNum)
	for j := 0; j < ipNum; j++ {
		allocateIPs[j] = freeIPs[j]
	}

	allocateIPRange, err = spiderpoolip.ConvertIPsToIPRanges(ipVersion, allocateIPs)
	if nil != err {
		return nil, err
	}

	logger.Sugar().Infof("generated '%d' IPs '%v' from SpiderSubnet '%s'", ipNum, allocateIPRange, subnetMgrName)

	return allocateIPRange, nil
}

func (sm *subnetManager) AllocateEmptyIPPool(ctx context.Context, subnetName string, appKind string, app metav1.Object, podSelector map[string]string,
	ipNum int, ipVersion types.IPVersion, reclaimIPPool bool) error {
	if len(subnetName) == 0 {
		return fmt.Errorf("spider subnet name must be specified")
	}
	if ipNum < 0 {
		return fmt.Errorf("the required IP numbers '%d' is invalid", ipNum)
	}

	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= sm.config.MaxConflictRetries; i++ {
		subnet, err := sm.GetSubnetByName(ctx, subnetName)
		if nil != err {
			if i == sm.config.MaxConflictRetries {
				return fmt.Errorf("%w: %v", ErrMaxRetries, err)
			}

			logger.Error(err.Error())
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		poolLabels := map[string]string{
			constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
			constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(appKind, app.GetNamespace(), app.GetName()),
			constant.LabelIPPoolOwnerApplicationUID: string(app.GetUID()),
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
				Name:   controllers.SubnetPoolName(appKind, app.GetNamespace(), app.GetName(), ipVersion),
				Labels: poolLabels,
			},
			Spec: spiderpoolv1.IPPoolSpec{
				Subnet:  subnet.Spec.Subnet,
				Gateway: subnet.Spec.Gateway,
				Vlan:    subnet.Spec.Vlan,
				Routes:  subnet.Spec.Routes,
				PodAffinity: &metav1.LabelSelector{
					MatchLabels: podSelector,
				},
			},
		}

		logger.Sugar().Infof("try to create IPPool '%v'", sp)
		err = sm.ipPoolManager.CreateIPPool(ctx, sp)
		if nil != err {
			if i == sm.config.MaxConflictRetries {
				return fmt.Errorf("%w, failed to create IPPool, error: %v", ErrMaxRetries, err)
			}

			logger.Sugar().Errorf("failed to create IPPool, error: %v", err)
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		logger.Sugar().Infof("try to update IPPool '%v' status DesiredIPNumber '%d'", sp, ipNum)
		err = sm.ipPoolManager.UpdateDesiredIPNumber(ctx, sp, ipNum)
		if nil != err {
			if i == sm.config.MaxConflictRetries {
				return fmt.Errorf("%w, failed to update IPPool '%s' status DesiredIPNumber '%d', error: %v", ErrMaxRetries, sp.Name, ipNum, err)
			}

			logger.Sugar().Errorf("failed to update IPPool '%s' status DesiredIPNumber '%d', error: %v", sp.Name, ipNum, err)
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.config.ConflictRetryUnitTime)
			continue
		}

		break
	}

	return nil
}

// CheckScaleIPPool will fetch some IPs from the specified subnet manager to expand the pool IPs
func (sm *subnetManager) CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetMgrName string, ipNum int) error {
	if pool == nil {
		return fmt.Errorf("IPPool must be specified")
	}
	if ipNum <= 0 {
		return fmt.Errorf("assign IP number '%d' is invalid", ipNum)
	}

	needUpdate := false
	if pool.Status.AutoDesiredIPCount == nil {
		// no desired IP number annotation
		needUpdate = true
	} else {
		// ignore it if they are equal
		if *pool.Status.AutoDesiredIPCount == int64(ipNum) {
			logger.Sugar().Debugf("no need to scale subnet '%s' IPPool '%s'", subnetMgrName, pool.Name)
			return nil
		}

		// not equal
		needUpdate = true
	}

	if needUpdate {
		logger.Sugar().Infof("try to update IPPool '%s' status DesiredIPNumber to '%d'", pool.Name, ipNum)
		err := sm.ipPoolManager.UpdateDesiredIPNumber(ctx, pool, ipNum)
		if nil != err {
			return fmt.Errorf("failed to update IPPool '%s' status DesiredIPNumber to '%d', error: %v", pool.Name, ipNum, err)
		}
	}

	return nil
}
