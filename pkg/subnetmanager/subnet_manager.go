// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"math/rand"
	"net"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var logger *zap.Logger

var defaultSpareIP int = 5

type SubnetManager interface {
	InitControllers(ctx context.Context, client kubernetes.Interface)
	GetSubnetByName(ctx context.Context, name string) (*spiderpoolv1.SpiderSubnet, error)
	GenerateIPsFromSubnet(ctx context.Context, subnetMgrName string, ipNum int) ([]string, error)
	AllocateIPPool(ctx context.Context, subnetMgrName string, appKind string, app metav1.Object, ipNum int, ipVersion types.IPVersion, reclaimIPPool bool) error
	RetrieveIPPools(ctx context.Context, appKind string, app metav1.Object) (v4Pool, v6Pool *spiderpoolv1.SpiderIPPool, err error)
	IPPoolExpansion(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetManagerName string, ipNum int) error
}

type subnetMgr struct {
	client client.Client
	scheme *runtime.Scheme

	stop                  chan struct{}
	maxConflictRetries    int
	conflictRetryUnitTime time.Duration

	poolMgr ippoolmanager.IPPoolManager
	leader  election.SpiderLeaseElector
}

func NewSubnetManager(c client.Client, scheme *runtime.Scheme, poolMgr ippoolmanager.IPPoolManager, spiderControllerLeader election.SpiderLeaseElector,
	maxConflictRetries int, conflictRetryUnitTime time.Duration) (SubnetManager, error) {
	if c == nil {
		return nil, fmt.Errorf("k8s client must be specified")
	}
	if scheme == nil {
		return nil, fmt.Errorf("object scheme must be specified")
	}
	if poolMgr == nil {
		return nil, fmt.Errorf("ippool manager must be specified")
	}

	logger = logutils.Logger.Named("Subnet-Manager")

	return &subnetMgr{
		client:                c,
		scheme:                scheme,
		maxConflictRetries:    maxConflictRetries,
		conflictRetryUnitTime: conflictRetryUnitTime,
		poolMgr:               poolMgr,
		leader:                spiderControllerLeader,
	}, nil
}

func (sm *subnetMgr) GetSubnetByName(ctx context.Context, name string) (*spiderpoolv1.SpiderSubnet, error) {
	var subnet spiderpoolv1.SpiderSubnet
	err := sm.client.Get(ctx, apitypes.NamespacedName{Name: name}, &subnet)
	if nil != err {
		return nil, err
	}

	return &subnet, nil
}

func (sm *subnetMgr) GenerateIPsFromSubnet(ctx context.Context, subnetMgrName string, ipNum int) ([]string, error) {
	var allocateIPRange []string

	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= sm.maxConflictRetries; i++ {
		subnet, err := sm.GetSubnetByName(ctx, subnetMgrName)
		if nil != err {
			return nil, err
		}

		var ipVersion types.IPVersion
		if subnet.Spec.IPVersion != nil {
			ipVersion = *subnet.Spec.IPVersion
		} else {
			return nil, fmt.Errorf("miss subnet '%v' spec IP version", subnet)
		}

		subnetIPs := subnet.Status.FreeIPs

		// reverse subnetIPs to decrease conflict
		if time.Now().UnixNano()%2 == 0 {
			length := len(subnetIPs)
			for j := 0; j < length/2; j++ {
				tmp := subnetIPs[length-j-1]
				subnetIPs[length-1-j] = subnetIPs[j]
				subnetIPs[j] = tmp
			}
			subnetIPs = subnet.Status.FreeIPs
		}

		// freeIPs
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
		freeIPs = freeIPs[ipNum:]

		leftIPRanges, err := spiderpoolip.ConvertIPsToIPRanges(ipVersion, freeIPs)
		if nil != err {
			return nil, err
		}

		allocateIPRange, err = spiderpoolip.ConvertIPsToIPRanges(ipVersion, allocateIPs)
		if nil != err {
			return nil, err
		}

		// fresh subnet FreeIPs
		subnet.Status.FreeIPs = leftIPRanges
		err = sm.client.Status().Update(ctx, subnet)
		if nil != err {
			if !apierrors.IsConflict(err) {
				return nil, err
			}

			if i == sm.maxConflictRetries {
				return nil, fmt.Errorf("insufficient retries(<=%d) to update subnet '%s' FreeIPs", sm.maxConflictRetries, subnet.Name)
			}

			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * sm.conflictRetryUnitTime)
			continue
		}
		break
	}

	return allocateIPRange, nil
}

func (sm *subnetMgr) AllocateIPPool(ctx context.Context, subnetMgrName string, appKind string, app metav1.Object,
	ipNum int, ipVersion types.IPVersion, reclaimIPPool bool) error {
	if len(subnetMgrName) == 0 {
		return fmt.Errorf("subnet manager name must be specified")
	}
	if ipNum <= 0 {
		return fmt.Errorf("the required IP numbers '%d' is invalid", ipNum)
	}

	subnet, err := sm.GetSubnetByName(ctx, subnetMgrName)
	if nil != err {
		return fmt.Errorf("failed to get subnet '%s', error: %v", subnetMgrName, err)
	}

	poolIPs, err := sm.GenerateIPsFromSubnet(ctx, subnetMgrName, ipNum+defaultSpareIP)
	if nil != err {
		return err
	}

	poolName := fmt.Sprintf("auto-%s-%s-%s-v%d", appKind, app.GetNamespace(), app.GetName(), ipVersion)
	poolLabels := map[string]string{
		OwnedSubnetManager: subnet.Name,
		OwnedApplication:   AppName(appKind, app.GetNamespace(), app.GetName()),
	}

	sp := &spiderpoolv1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      poolName,
			Namespace: app.GetNamespace(),
			Labels:    poolLabels,
		},
		Spec: spiderpoolv1.IPPoolSpec{
			Subnet:  subnet.Spec.Subnet,
			IPs:     poolIPs,
			Gateway: subnet.Spec.Gateway,
			Vlan:    subnet.Spec.Vlan,
			Routes:  subnet.Spec.Routes,
		},
	}

	// IPPool lifecycle is same with APP
	if reclaimIPPool {
		err = controllerutil.SetOwnerReference(app, sp, sm.scheme)
		if nil != err {
			return err
		}
	}

	err = sm.client.Create(ctx, sp)
	if nil != err {
		return err
	}

	return nil
}

// RetrieveIPPools try to retrieve IPPools by app label
func (sm *subnetMgr) RetrieveIPPools(ctx context.Context, appKind string, app metav1.Object) (v4Pool, v6Pool *spiderpoolv1.SpiderIPPool, err error) {
	poolList, err := sm.poolMgr.ListIPPools(ctx,
		client.MatchingLabels{OwnedApplication: AppName(appKind, app.GetNamespace(), app.GetName())},
	)
	if nil != err {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil
		}

		return nil, nil, err
	}

	if len(poolList.Items) == 0 {
		return nil, nil, nil
	}

	for _, pool := range poolList.Items {
		// v4 pool
		if pool.Name == SubnetPoolName(appKind, app.GetNamespace(), app.GetName(), constant.IPv4) {
			v4Pool = pool.DeepCopy()
		}

		// v6 pool
		if pool.Name == SubnetPoolName(appKind, app.GetNamespace(), app.GetName(), constant.IPv6) {
			v6Pool = pool.DeepCopy()
		}
	}

	return
}

func (sm *subnetMgr) IPPoolExpansion(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetManagerName string, ipNum int) error {
	if pool == nil {
		return fmt.Errorf("%w: IPPool must be specified", ErrorAnnoInput)
	}
	if ipNum <= 0 {
		return fmt.Errorf("%w: Assign IP number '%d' is invalid", ErrorAnnoInput, ipNum)
	}

	// no need to expand
	if len(pool.Spec.IPs)-defaultSpareIP == ipNum {
		return nil
	}

	ipsFromSubnet, err := sm.GenerateIPsFromSubnet(ctx, subnetManagerName, ipNum+defaultSpareIP-len(pool.Spec.IPs))
	if nil != err {
		return err
	}

	// update IPPool
	pool.Spec.IPs = append(pool.Spec.IPs, ipsFromSubnet...)

	sortedIPRanges, err := spiderpoolip.SortIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
	if nil != err {
		return err
	}
	if !reflect.DeepEqual(pool.Spec.IPs, sortedIPRanges) {
		pool.Spec.IPs = sortedIPRanges
	}

	for i := 0; i < sm.maxConflictRetries; i++ {
		err = sm.client.Update(ctx, pool)
		if nil != err {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == sm.maxConflictRetries {
				return fmt.Errorf("insufficient retries(<=%d) to update IPPool '%s'", sm.maxConflictRetries, pool.Name)
			}
			continue
		}
		break
	}

	return nil
}
