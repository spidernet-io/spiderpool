// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type SubnetManager interface {
	GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error)
	SetupControllers(client kubernetes.Interface, leader election.SpiderLeaseElector) error
	AllocateEmptyIPPool(ctx context.Context, subnetMgrName string, podController types.PodTopController, podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error)
	CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetManagerName string, ipNum int) error
}

var logger *zap.Logger

type subnetManager struct {
	config        *SubnetManagerConfig
	client        client.Client
	runtimeMgr    ctrl.Manager
	ipPoolManager ippoolmanager.IPPoolManager
	reservedMgr   reservedipmanager.ReservedIPManager

	workQueue workqueue.RateLimitingInterface
}

func NewSubnetManager(c *SubnetManagerConfig, mgr ctrl.Manager, ipPoolManager ippoolmanager.IPPoolManager, reservedIPMgr reservedipmanager.ReservedIPManager) (SubnetManager, error) {
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

	subnet, err := sm.GetSubnetByName(ctx, subnetName)
	if nil != err {
		return nil, err
	}

	if subnet.DeletionTimestamp != nil {
		return nil, fmt.Errorf("%w: SpiderSubnet '%s' is terminating, we can't create a corresponding IPPool", constant.ErrWrongInput, subnet.Name)
	}

	poolLabels := map[string]string{
		constant.LabelIPPoolOwnerSpiderSubnet:   subnet.Name,
		constant.LabelIPPoolOwnerApplication:    controllers.AppLabelValue(podController.Kind, podController.Namespace, podController.Name),
		constant.LabelIPPoolOwnerApplicationUID: string(podController.Uid),
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
			Name:   controllers.SubnetPoolName(podController.Kind, podController.Namespace, podController.Name, ipVersion, ifName, podController.Uid),
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
		return nil, err
	}

	logger.Sugar().Infof("try to update IPPool '%v' status DesiredIPNumber '%d'", sp, ipNum)
	err = sm.ipPoolManager.UpdateDesiredIPNumber(ctx, sp, ipNum)
	if nil != err {
		return nil, err
	}

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
