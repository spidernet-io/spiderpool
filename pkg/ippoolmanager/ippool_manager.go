// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type IPPoolManager interface {
	AllocateIP(ctx context.Context, ownerType types.OwnerType, poolName, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndCIDs []IPAndCID) error
	SelectByPod(ctx context.Context, version string, poolName string, pod *corev1.Pod) (bool, error)
	CheckVlanSame(ctx context.Context, poolList []string) (map[spiderpoolv1.Vlan][]string, bool, error)
	CheckPoolCIDROverlap(ctx context.Context, poolList1 []string, poolList2 []string) (bool, error)
}

type ipPoolManager struct {
	client            client.Client
	rIPManager        reservedipmanager.ReservedIPManager
	nodeManager       nodemanager.NodeManager
	nsManager         namespacemanager.NamespaceManager
	podManager        podmanager.PodManager
	maxConflictRetrys int
	maxAllocatedIPs   int
}

func NewIPPoolManager(c client.Client, rIPManager reservedipmanager.ReservedIPManager, nodeManager nodemanager.NodeManager, nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager, maxConflictRetrys, maxAllocatedIPs int) (IPPoolManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}
	if rIPManager == nil {
		return nil, errors.New("reserved IP manager must be specified")
	}
	if nodeManager == nil {
		return nil, errors.New("node manager must be specified")
	}
	if nsManager == nil {
		return nil, errors.New("namespace manager must be specified")
	}
	if podManager == nil {
		return nil, errors.New("pod manager must be specified")
	}

	return &ipPoolManager{
		client:            c,
		rIPManager:        rIPManager,
		nodeManager:       nodeManager,
		nsManager:         nsManager,
		podManager:        podManager,
		maxConflictRetrys: maxConflictRetrys,
		maxAllocatedIPs:   maxAllocatedIPs,
	}, nil
}

func (r *ipPoolManager) AllocateIP(ctx context.Context, ownerType types.OwnerType, poolName, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, error) {
	var ipConfig *models.IPConfig

	// TODO(iiiceoo): STS static ip, check "EnableStatuflsetIP"
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= r.maxConflictRetrys; i++ {
		var ipPool spiderpoolv1.IPPool
		if err := r.client.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool); err != nil {
			return nil, err
		}

		// TODO(iiiceoo): Check TotalIPCount - AllocatedIPCount

		reserved, err := r.rIPManager.GetReservedIPRanges(ctx, string(*ipPool.Spec.IPVersion))
		if err != nil {
			return nil, err
		}

		var used []string
		for ip := range ipPool.Status.AllocatedIPs {
			used = append(used, ip)
		}

		allocateIP, err := randomIP(ipPool.Spec.IPs, used, ipPool.Spec.ExcludeIPs, reserved)
		if err != nil {
			return nil, err
		}

		// TODO(iiiceoo): Remove when Defaulter webhook work
		if ipPool.Status.AllocatedIPs == nil {
			ipPool.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
		}
		ipPool.Status.AllocatedIPs[allocateIP.String()] = spiderpoolv1.PoolIPAllocation{
			ContainerID: containerID,
			NIC:         nic,
			Node:        pod.Spec.NodeName,
			Namespace:   pod.Namespace,
			Pod:         pod.Name,
		}

		// TODO(iiiceoo): Remove when Defaulter webhook work
		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int32)
		}

		*ipPool.Status.AllocatedIPCount++
		if *ipPool.Status.AllocatedIPCount > int32(r.maxAllocatedIPs) {
			return nil, fmt.Errorf("threshold of IP allocations(<=%d) for IP pool exceeded: %w", r.maxAllocatedIPs, constant.ErrIPUsedOut)
		}

		if err := r.client.Status().Update(ctx, &ipPool); err != nil {
			if apierrors.IsConflict(err) {
				if i == r.maxConflictRetrys {
					return nil, fmt.Errorf("insufficient retries(<=%d) to allocate IP from IP pool %s", r.maxConflictRetrys, poolName)
				}

				time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * time.Second)
				continue
			}
			return nil, err
		}

		ipConfig = genResIPConfig(allocateIP, &ipPool.Spec, nic, poolName)
		break
	}

	return ipConfig, nil
}

func randomIP(all []string, used []string, exclude []string, reserved []string) (net.IP, error) {
	// TODO(iiiceoo): Check nil
	reservedIPs := ip.ParseIPRanges(reserved)
	usedIPs := ip.ParseIPRanges(used)
	expectIPs := ip.ParseIPRanges(all)
	excludeIPs := ip.ParseIPRanges(exclude)
	availableIPs := ip.IPsDiffSet(expectIPs, append(reservedIPs, append(usedIPs, excludeIPs...)...))

	if len(availableIPs) == 0 {
		return nil, constant.ErrIPUsedOut
	}

	return availableIPs[rand.Int()%len(availableIPs)], nil
}

func genResIPConfig(allocateIP net.IP, poolSpec *spiderpoolv1.IPPoolSpec, nic, poolName string) *models.IPConfig {
	ipNet := ip.ParseIP(poolSpec.Subnet)
	ipNet.IP = allocateIP
	address := ipNet.String()

	var version int64
	if *poolSpec.IPVersion == spiderpoolv1.IPv4 {
		version = 4
	} else {
		version = 6
	}

	// TODO(iiiceoo): Remove when Defaulter webhook work
	var gateway string
	if poolSpec.Gateway != nil {
		gateway = *poolSpec.Gateway
	}

	return &models.IPConfig{
		Address: &address,
		Gateway: gateway,
		IPPool:  poolName,
		Nic:     &nic,
		Version: &version,
		Vlan:    int64(*poolSpec.Vlan),
	}
}

type IPAndCID struct {
	IP          string
	ContainerID string
}

func (r *ipPoolManager) ReleaseIP(ctx context.Context, poolName string, ipAndCIDs []IPAndCID) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= r.maxConflictRetrys; i++ {
		var ipPool spiderpoolv1.IPPool
		if err := r.client.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool); err != nil {
			return err
		}

		// TODO(iiiceoo): Remove when Defaulter webhook work
		if ipPool.Status.AllocatedIPs == nil {
			ipPool.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
		}
		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int32)
		}

		needRelease := false
		for _, e := range ipAndCIDs {
			if a, ok := ipPool.Status.AllocatedIPs[e.IP]; ok {
				if a.ContainerID == e.ContainerID {
					delete(ipPool.Status.AllocatedIPs, e.IP)
					*ipPool.Status.AllocatedIPCount--
					needRelease = true
				}
			}
		}

		if !needRelease {
			return nil
		}

		if err := r.client.Status().Update(ctx, &ipPool); err != nil {
			if apierrors.IsConflict(err) {
				if i == r.maxConflictRetrys {
					return fmt.Errorf("insufficient retries(<=%d) to release IP %+v from IP pool %s", r.maxConflictRetrys, ipAndCIDs, poolName)
				}

				time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * time.Second)
				continue
			}
			return err
		}
		break
	}

	return nil
}

func (r *ipPoolManager) SelectByPod(ctx context.Context, version string, poolName string, pod *corev1.Pod) (bool, error) {
	logger := logutils.FromContext(ctx).Named("IPPoolManager")

	var ipPool spiderpoolv1.IPPool
	if err := r.client.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool); err != nil {
		logger.Sugar().Warnf("IP pool %s is not found", poolName)
		return false, client.IgnoreNotFound(err)
	}

	if ipPool.DeletionTimestamp != nil {
		logger.Sugar().Warnf("IP pool %s is terminating", poolName)
		return false, nil
	}

	if *ipPool.Spec.Disable {
		logger.Sugar().Warnf("IP pool %s is disable", poolName)
		return false, nil
	}

	if string(*ipPool.Spec.IPVersion) != version {
		logger.Sugar().Warnf("IP pool %s has different version with specified via input", poolName)
		return false, nil
	}

	if ipPool.Spec.NodeSelector != nil {
		nodeMatched, err := r.nodeManager.MatchLabelSelector(ctx, pod.Spec.NodeName, ipPool.Spec.NodeSelector)
		if err != nil {
			return false, err
		}
		if !nodeMatched {
			logger.Sugar().Infof("Unmatched Node selector, IP pool %s is filtered", poolName)
			return false, nil
		}
	}

	if ipPool.Spec.NamesapceSelector != nil {
		nsMatched, err := r.nsManager.MatchLabelSelector(ctx, pod.Namespace, ipPool.Spec.NamesapceSelector)
		if err != nil {
			return false, err
		}
		if !nsMatched {
			logger.Sugar().Infof("Unmatched Namespace selector, IP pool %s is filtered", poolName)
			return false, nil
		}
	}

	if ipPool.Spec.PodSelector != nil {
		podMatched, err := r.podManager.MatchLabelSelector(ctx, pod.Namespace, pod.Name, ipPool.Spec.PodSelector)
		if err != nil {
			return false, err
		}
		if !podMatched {
			logger.Sugar().Infof("Unmatched Pod selector, IP pool %s is filtered", poolName)
			return false, nil
		}
	}

	return true, nil
}

func (r *ipPoolManager) CheckVlanSame(ctx context.Context, poolList []string) (map[spiderpoolv1.Vlan][]string, bool, error) {
	vlanToPools := map[spiderpoolv1.Vlan][]string{}
	for _, p := range poolList {
		var ipPool spiderpoolv1.IPPool
		if err := r.client.Get(ctx, apitypes.NamespacedName{Name: p}, &ipPool); err != nil {
			return nil, false, err
		}

		vlanToPools[*ipPool.Spec.Vlan] = append(vlanToPools[*ipPool.Spec.Vlan], p)
	}

	if len(vlanToPools) > 1 {
		return vlanToPools, false, nil
	}

	return vlanToPools, true, nil
}

func (r *ipPoolManager) CheckPoolCIDROverlap(ctx context.Context, poolList1 []string, poolList2 []string) (bool, error) {
	return false, nil
}
