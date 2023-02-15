// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type SubnetManager interface {
	GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error)
	AllocateEmptyIPPool(ctx context.Context, subnetMgrName string, podController types.PodTopController, podSelector *metav1.LabelSelector, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool, ifName string) (*spiderpoolv1.SpiderIPPool, error)
	CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetManagerName string, ipNum int) (bool, error)
}

type subnetManager struct {
	config        SubnetManagerConfig
	client        client.Client
	ipPoolManager ippoolmanager.IPPoolManager
	Scheme        *runtime.Scheme
}

func NewSubnetManager(config SubnetManagerConfig, client client.Client, ipPoolManager ippoolmanager.IPPoolManager, scheme *runtime.Scheme) (SubnetManager, error) {
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
func (sm *subnetManager) CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetName string, ipNum int) (bool, error) {
	if pool == nil {
		return false, fmt.Errorf("%w: IPPool must be specified", constant.ErrWrongInput)
	}
	if ipNum <= 0 {
		return false, fmt.Errorf("%w: assign IP number '%d' is invalid", constant.ErrWrongInput, ipNum)
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
			return false, nil
		}

		// not equal
		needUpdate = true
	}

	if needUpdate {
		log.Sugar().Infof("try to update IPPool '%s' status DesiredIPNumber to '%d'", pool.Name, ipNum)
		err := sm.ipPoolManager.UpdateDesiredIPNumber(ctx, pool, ipNum)
		if nil != err {
			return true, err
		}
	}

	return false, nil
}
