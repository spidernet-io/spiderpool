// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
)

type IPPoolManager interface {
	AllocateIP(ctx context.Context, ownerType constant.OwnerType, poolName, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName, containerID string, pod *corev1.Pod) error
	ListPoolAll(ctx context.Context) (*spiderpoolv1.IPPoolList, error)
	SelectByPod(ctx context.Context, version spiderpoolv1.IPVersion, poolName string, pod *corev1.Pod) (bool, error)
	CheckVlanSame(ctx context.Context, poolList []string) (bool, error)
	CheckPoolCIDROverlap(ctx context.Context, poolList1 []string, poolList2 []string) (bool, error)
}

type ipPoolManager struct {
	client      client.Client
	rIPManager  reservedipmanager.ReservedIPManager
	nodeManager nodemanager.NodeManager
	nsManager   namespacemanager.NamespaceManager
	podManager  podmanager.PodManager
}

func NewIPPoolManager(c client.Client, rIPManager reservedipmanager.ReservedIPManager, nodeManager nodemanager.NodeManager, nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager) IPPoolManager {
	return &ipPoolManager{
		client:      c,
		rIPManager:  rIPManager,
		nodeManager: nodeManager,
		nsManager:   nsManager,
		podManager:  podManager,
	}
}

func (r *ipPoolManager) AllocateIP(ctx context.Context, ownerType constant.OwnerType, poolName, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, error) {
	var ipConfig *models.IPConfig

	// TODO(iiiceoo): STS static ip, check "EnableStatuflsetIP"
	if ownerType == constant.OwnerStatefuleSet {
		fmt.Println(ownerType)
	}

	retryCount := 3
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= retryCount; i++ {
		var ipPool spiderpoolv1.IPPool
		if err := r.client.Get(ctx, types.NamespacedName{Name: poolName}, &ipPool); err != nil {
			return nil, err
		}

		// TODO(iiiceoo): Check TotalIPCount - AllocatedIPCount

		reservedIPRanges, err := r.rIPManager.GetReservedIPRanges(ctx, ipPool.Spec.IPVersion)
		if err != nil {
			return nil, err
		}
		reservedIPs := ip.ParseIPRanges(reservedIPRanges)

		var usedIPRanges []string
		for k := range ipPool.Status.AllocatedIPs {
			usedIPRanges = append(usedIPRanges, k)
		}
		usedIPs := ip.ParseIPRanges(usedIPRanges)

		expectIPs := ip.ParseIPRanges(ipPool.Spec.IPs)
		excludeIPs := ip.ParseIPRanges(ipPool.Spec.ExcludeIPs)
		disableIPs := append(reservedIPs, append(usedIPs, excludeIPs...)...)
		availableIPs := ip.IPsDiffSet(expectIPs, disableIPs)

		if len(availableIPs) == 0 {
			return nil, errors.New("all IP used out")
		}
		allocateIP := availableIPs[rand.Int()%len(availableIPs)]

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
		if ipPool.Status.AllocateCount == nil {
			ipPool.Status.AllocateCount = new(int64)
		}
		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int32)
		}
		*ipPool.Status.AllocateCount++
		*ipPool.Status.AllocatedIPCount++
		if err := r.client.Status().Update(ctx, &ipPool); err != nil {
			if apierrors.IsConflict(err) {
				if i == retryCount {
					return nil, err
				}

				time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * time.Second)
				continue
			}
			return nil, err
		}

		// TODO(iiiceoo): Check nil? Refactor ip.ParseIP?
		var address string
		ipNet := ip.ParseIP(ipPool.Spec.Subnet)
		ipNet.IP = allocateIP
		address = ipNet.String()

		var version int64
		if ipPool.Spec.IPVersion == spiderpoolv1.IPv4 {
			version = 4
		} else {
			version = 6
		}

		// TODO(iiiceoo): Remove when Defaulter webhook work
		var vlan int64
		if ipPool.Spec.Vlan != nil {
			vlan = int64(*ipPool.Spec.Vlan)
		}
		var gateway string
		if ipPool.Spec.Gateway != nil {
			gateway = *ipPool.Spec.Gateway
		}

		ipConfig = &models.IPConfig{
			Address: &address,
			Gateway: gateway,
			Nic:     &nic,
			Version: &version,
			Vlan:    vlan,
		}
		break
	}

	return ipConfig, nil
}

func (r *ipPoolManager) ReleaseIP(ctx context.Context, poolName, containerID string, pod *corev1.Pod) error {
	return nil
}

func (r *ipPoolManager) ListPoolAll(ctx context.Context) (*spiderpoolv1.IPPoolList, error) {
	return nil, nil
}

func (r *ipPoolManager) SelectByPod(ctx context.Context, version spiderpoolv1.IPVersion, poolName string, pod *corev1.Pod) (bool, error) {
	var ipPool spiderpoolv1.IPPool
	if err := r.client.Get(ctx, types.NamespacedName{Name: poolName}, &ipPool); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	if ipPool.DeletionTimestamp != nil {
		return false, nil
	}

	if ipPool.Spec.Disable {
		return false, nil
	}

	if ipPool.Spec.IPVersion != version {
		return false, nil
	}

	if ipPool.Spec.NodeSelector != nil {
		nodeMatched, err := r.nodeManager.MatchLabelSelector(ctx, pod.Spec.NodeName, ipPool.Spec.NodeSelector)
		if err != nil {
			return false, err
		}
		if !nodeMatched {
			return false, nil
		}
	}

	if ipPool.Spec.NodeSelector != nil {
		nsMatched, err := r.nsManager.MatchLabelSelector(ctx, pod.Namespace, ipPool.Spec.NodeSelector)
		if err != nil {
			return false, err
		}
		if !nsMatched {
			return false, nil
		}
	}

	if ipPool.Spec.NodeSelector != nil {
		podMatched, err := r.podManager.MatchLabelSelector(ctx, pod.Namespace, pod.Name, ipPool.Spec.NodeSelector)
		if err != nil {
			return false, err
		}
		if !podMatched {
			return false, nil
		}
	}

	return true, nil
}

func (r *ipPoolManager) CheckVlanSame(ctx context.Context, poolList []string) (bool, error) {
	marks := make(map[spiderpoolv1.Vlan]bool)
	for _, p := range poolList {
		var ipPool spiderpoolv1.IPPool
		if err := r.client.Get(ctx, types.NamespacedName{Name: p}, &ipPool); err != nil {
			return false, err
		}

		if ipPool.Spec.Vlan == nil {
			continue
		}

		marks[*ipPool.Spec.Vlan] = true
	}

	if len(marks) != 0 && len(marks) != 1 {
		return false, nil
	}

	return true, nil
}

func (r *ipPoolManager) CheckPoolCIDROverlap(ctx context.Context, poolList1 []string, poolList2 []string) (bool, error) {
	return false, nil
}
