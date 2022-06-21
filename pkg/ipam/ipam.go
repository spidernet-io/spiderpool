// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

type IPAM interface {
	Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error)
	Release(ctx context.Context, delArgs *models.IpamDelArgs) error
}

type IPAMConfig struct {
	StatuflsetIPEnable       bool
	EnableIPv4               bool
	EnableIPv6               bool
	ClusterDefaultIPv4IPPool []string
	ClusterDefaultIPv6IPPool []string
}

type ipam struct {
	ipamConfig    *IPAMConfig
	ipPoolManager ippoolmanager.IPPoolManager
	weManager     workloadendpointmanager.WorkloadEndpointManager
	nsManager     namespacemanager.NamespaceManager
	podManager    podmanager.PodManager
}

func NewIPAM(c *IPAMConfig, ipPoolManager ippoolmanager.IPPoolManager, weManager workloadendpointmanager.WorkloadEndpointManager, nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager) (IPAM, error) {
	if c == nil {
		return nil, errors.New("ipam config must be specified")
	}

	if ipPoolManager == nil {
		return nil, errors.New("ip pool manager must be specified")
	}

	if weManager == nil {
		return nil, errors.New("workload endpoint manager must be specified")
	}

	if nsManager == nil {
		return nil, errors.New("namespace manager must be specified")
	}

	if podManager == nil {
		return nil, errors.New("pod manager must be specified")
	}

	return &ipam{
		ipamConfig:    c,
		ipPoolManager: ipPoolManager,
		weManager:     weManager,
		nsManager:     nsManager,
		podManager:    podManager,
	}, nil
}

type ToBeAllocated struct {
	NIC              string
	V4PoolCandidates []string
	V6PoolCandidates []string
}

func (i *ipam) Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to allocate IP")

	pod, err := i.podManager.GetPodByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %v", *addArgs.PodName, err)
	}

	podStatus, allocatable := i.podManager.IsIPAllocatable(ctx, pod)
	if !allocatable {
		return nil, fmt.Errorf("pod is %s: %w", podStatus, constant.ErrNotAllocatablePod)
	}

	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *addArgs.PodNamespace, *addArgs.PodName, *addArgs.ContainerID, *addArgs.IfName, false)
	if err != nil {
		return nil, err
	}

	addResp := &models.IpamAddResponse{}
	if allocation != nil && currently {
		logger.Sugar().Infof("Retrieve an existing IP allocation: %+v", allocation.IPs)
		addResp.Ips = convertToIPConfigs(allocation.IPs)
		// TODO(iiiceoo): Return routes or DNS
		// addResp.DNS = xxx
		// addResp.Routes = xxx

		return addResp, nil
	}

	preliminary, err := getPodIPPoolCandidates(ctx, i.nsManager, i.ipamConfig, *addArgs.IfName, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Preliminary IP pool candidates: %+v", preliminary)

	toBeAllocatedSet, err := filterPodIPPoolCandidates(ctx, i.ipPoolManager, preliminary, pod)
	if err != nil {
		return nil, err
	}
	logger.Sugar().Infof("Filtered IP pool candidates: %+v", toBeAllocatedSet)

	ownerType := i.podManager.GetOwnerType(ctx, pod)
	logger.Sugar().Infof("Pod owner type: %s", ownerType)

	var resIPs []*models.IPConfig
	defer func() {
		if err != nil {
			if err := i.release(ctx, *addArgs.ContainerID, convertToIPAllocationDetails(resIPs)); err != nil {
				logger.Sugar().Warnf("Failed to roll back allocated IP: %v", err)
			}
		}
	}()

	resIPs, podAnnotations, err := execAllocate(ctx, i.ipPoolManager, toBeAllocatedSet, ownerType, *addArgs.ContainerID, pod)
	if err != nil {
		return nil, err
	}

	err = i.podManager.MergeAnnotations(ctx, pod, podAnnotations)
	if err != nil {
		return nil, fmt.Errorf("failed to merge IP assignment annotation of pod: %v", err)
	}

	err = i.weManager.UpdateIPAllocation(ctx, *addArgs.PodNamespace, *addArgs.PodName, &spiderpoolv1.PodIPAllocation{
		ContainerID:  *addArgs.ContainerID,
		Node:         pod.Spec.NodeName,
		IPs:          convertToIPAllocationDetails(resIPs),
		CreationTime: metav1.Time{Time: time.Now()},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update IP allocation of workload endpoint: %v", err)
	}

	addResp.Ips = resIPs
	logger.Sugar().Infof("Succeed to allocate: %+v", addResp)

	return addResp, nil
}

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	logger := logutils.FromContext(ctx)
	logger.Info("Start to release IP")

	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *delArgs.PodNamespace, *delArgs.PodName, *delArgs.ContainerID, *delArgs.IfName, true)
	if err != nil {
		return err
	}

	if allocation == nil {
		logger.Info("Nothing retrieved for releasing")
		return nil
	}

	if !currently {
		logger.Warn("Request to release a non current IP allocation, there may be concurrency between the same pod")
	}

	if err = i.release(ctx, allocation.ContainerID, allocation.IPs); err != nil {
		return err
	}

	if err := i.weManager.ClearCurrentIPAllocation(ctx, *delArgs.PodNamespace, *delArgs.PodName, *delArgs.ContainerID, *delArgs.IfName); err != nil {
		return err
	}

	logger.Sugar().Infof("Succeed to release: %+v", allocation.IPs)

	return nil
}

func (i *ipam) release(ctx context.Context, containerID string, details []spiderpoolv1.IPAllocationDetail) error {
	logger := logutils.FromContext(ctx)

	if len(details) == 0 {
		return nil
	}

	poolToIPAndCIDs := map[string][]ippoolmanager.IPAndContainerID{}
	for _, d := range details {
		if d.IPv4 != nil {
			poolToIPAndCIDs[*d.IPv4Pool] = append(poolToIPAndCIDs[*d.IPv4Pool], ippoolmanager.IPAndContainerID{
				IP:          strings.Split(*d.IPv4, "/")[0],
				ContainerID: containerID,
			})
		}
		if d.IPv6 != nil {
			poolToIPAndCIDs[*d.IPv6Pool] = append(poolToIPAndCIDs[*d.IPv6Pool], ippoolmanager.IPAndContainerID{
				IP:          strings.Split(*d.IPv6, "/")[0],
				ContainerID: containerID,
			})
		}
	}

	errCh := make(chan error, len(poolToIPAndCIDs))
	wg := sync.WaitGroup{}
	wg.Add(len(poolToIPAndCIDs))

	for pool, ipAndCIDs := range poolToIPAndCIDs {
		go func(pool string, ipAndCIDs []ippoolmanager.IPAndContainerID) {
			defer wg.Done()
			if err := i.ipPoolManager.ReleaseIP(ctx, pool, ipAndCIDs); err != nil {
				errCh <- err
			}
			logger.Sugar().Infof("Succeed to release IP %+v from IP pool %s", ipAndCIDs, pool)
		}(pool, ipAndCIDs)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to release all allocated IP %+v: %v", poolToIPAndCIDs, utilerrors.NewAggregate(errs).Error())
	}

	return nil
}

func convertToIPConfigs(details []spiderpoolv1.IPAllocationDetail) []*models.IPConfig {
	var ipConfigs []*models.IPConfig
	for _, d := range details {
		var version int64
		if d.IPv4 != nil {
			version = 4
			ipConfigs = append(ipConfigs, &models.IPConfig{
				Address: d.IPv4,
				Nic:     &d.NIC,
				Version: &version,
				Vlan:    int64(*d.Vlan),
				Gateway: *d.IPv4Gateway,
			})
		}

		if d.IPv6 != nil {
			version = 6
			ipConfigs = append(ipConfigs, &models.IPConfig{
				Address: d.IPv6,
				Nic:     &d.NIC,
				Version: &version,
				Vlan:    int64(*d.Vlan),
				Gateway: *d.IPv6Gateway,
			})
		}
	}

	return ipConfigs
}

func convertToIPAllocationDetails(ipConfigs []*models.IPConfig) []spiderpoolv1.IPAllocationDetail {
	nicToDetail := map[string]*spiderpoolv1.IPAllocationDetail{}
	for _, c := range ipConfigs {
		if d, ok := nicToDetail[*c.Nic]; ok {
			if *c.Version == 4 {
				d.IPv4 = c.Address
				d.IPv4Pool = &c.IPPool
				d.IPv4Gateway = &c.Gateway
			} else {
				d.IPv6 = c.Address
				d.IPv6Pool = &c.IPPool
				d.IPv6Gateway = &c.Gateway
			}
		} else {
			vlan := spiderpoolv1.Vlan(c.Vlan)
			if *c.Version == 4 {
				nicToDetail[*c.Nic] = &spiderpoolv1.IPAllocationDetail{
					NIC:         *c.Nic,
					IPv4:        c.Address,
					IPv4Pool:    &c.IPPool,
					Vlan:        &vlan,
					IPv4Gateway: &c.Gateway,
				}
			} else {
				nicToDetail[*c.Nic] = &spiderpoolv1.IPAllocationDetail{
					NIC:         *c.Nic,
					IPv6:        c.Address,
					IPv6Pool:    &c.IPPool,
					Vlan:        &vlan,
					IPv6Gateway: &c.Gateway,
				}
			}
		}
	}

	details := []spiderpoolv1.IPAllocationDetail{}
	for _, d := range nicToDetail {
		details = append(details, *d)
	}

	return details
}

func getPodIPPoolCandidates(ctx context.Context, nsManager namespacemanager.NamespaceManager, ipamConfig *IPAMConfig, nic string, pod *corev1.Pod) ([]ToBeAllocated, error) {
	// TODO(iiiceoo): Validate annotations
	logger := logutils.FromContext(ctx)

	var preliminary []ToBeAllocated
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		logger.Sugar().Infof("Use IP pools from Pod annotation '%s'", constant.AnnoPodIPPools)
		var annoPodIPPools types.AnnoPodIPPoolsValue
		err := json.Unmarshal([]byte(anno), &annoPodIPPools)
		if err != nil {
			return nil, err
		}

		for _, i := range annoPodIPPools {
			preliminary = append(preliminary, ToBeAllocated{
				NIC:              i.NIC,
				V4PoolCandidates: i.IPv4Pools,
				V6PoolCandidates: i.IPv6Pools,
			})
		}
	} else if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		logger.Sugar().Infof("Use IP pools from Pod annotation '%s'", constant.AnnoPodIPPool)
		var annoPodIPPool types.AnnoPodIPPoolValue
		if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
			return nil, err
		}

		if annoPodIPPool.NIC != nil && *annoPodIPPool.NIC != nic {
			return nil, fmt.Errorf("interface of pod annotaiton is not same with runtime request: %w", constant.ErrWrongInput)
		}

		preliminary = append(preliminary, ToBeAllocated{
			NIC:              nic,
			V4PoolCandidates: annoPodIPPool.IPv4Pools,
			V6PoolCandidates: annoPodIPPool.IPv6Pools,
		})
	} else {
		nsDefautlV4Pools, nsDefautlV6Pools, err := nsManager.GetNSDefaultPools(ctx, pod.Namespace)
		if err != nil {
			return nil, err
		}

		if !(nsDefautlV4Pools == nil && nsDefautlV6Pools == nil) {
			logger.Sugar().Infof("Use IP pools from Namespace annotation '%s'", constant.AnnotationPre+"/defaultv(4/6)ippool")
			preliminary = append(preliminary, ToBeAllocated{
				NIC:              nic,
				V4PoolCandidates: nsDefautlV4Pools,
				V6PoolCandidates: nsDefautlV6Pools,
			})
		} else if !(ipamConfig.ClusterDefaultIPv4IPPool == nil && ipamConfig.ClusterDefaultIPv6IPPool == nil) {
			logger.Info("Use IP pools from cluster default pools")
			preliminary = append(preliminary, ToBeAllocated{
				NIC:              nic,
				V4PoolCandidates: ipamConfig.ClusterDefaultIPv4IPPool,
				V6PoolCandidates: ipamConfig.ClusterDefaultIPv6IPPool,
			})
		} else {
			return nil, fmt.Errorf("no default IP pool is specified: %w", constant.ErrNoAvailablePool)
		}
	}

	for _, e := range preliminary {
		if !ipamConfig.EnableIPv4 && len(e.V4PoolCandidates) != 0 {
			return nil, fmt.Errorf("specify default IPv4 pool when IPv4 is disabled: %w", constant.ErrWrongInput)
		}

		if !ipamConfig.EnableIPv6 && len(e.V6PoolCandidates) != 0 {
			return nil, fmt.Errorf("specify default IPv6 pool when IPv6 is disabled: %w", constant.ErrWrongInput)
		}
	}

	return preliminary, nil
}

func filterPodIPPoolCandidates(ctx context.Context, ipPoolManager ippoolmanager.IPPoolManager, toBeAllocatedSet []ToBeAllocated, pod *corev1.Pod) ([]ToBeAllocated, error) {
	for idx, e := range toBeAllocatedSet {
		for idx, pool := range e.V4PoolCandidates {
			eligible, err := ipPoolManager.SelectByPod(ctx, string(spiderpoolv1.IPv4), pool, pod)
			if err != nil {
				return nil, err
			}
			if !eligible {
				e.V4PoolCandidates = append(e.V4PoolCandidates[:idx], e.V4PoolCandidates[idx+1:]...)
			}
		}

		for _, pool := range e.V6PoolCandidates {
			eligible, err := ipPoolManager.SelectByPod(ctx, string(spiderpoolv1.IPv6), pool, pod)
			if err != nil {
				return nil, err
			}
			if !eligible {
				e.V4PoolCandidates = append(e.V6PoolCandidates[:idx], e.V6PoolCandidates[idx+1:]...)
			}
		}

		if len(e.V4PoolCandidates) == 0 && len(e.V6PoolCandidates) == 0 {
			toBeAllocatedSet = append(toBeAllocatedSet[:idx], toBeAllocatedSet[idx+1:]...)
		}
	}

	if len(toBeAllocatedSet) == 0 {
		return nil, fmt.Errorf("all IP pools filtered out: %w", constant.ErrNoAvailablePool)
	}

	for _, e := range toBeAllocatedSet {
		allPools := append(e.V4PoolCandidates, e.V6PoolCandidates...)
		vlanToPools, same, err := ipPoolManager.CheckVlanSame(ctx, allPools)
		if err != nil {
			return nil, err
		}
		if !same {
			return nil, fmt.Errorf("vlans in each IP pools are not same: %w, details: %v", constant.ErrWrongInput, vlanToPools)
		}

		// TODO(iiiceoo): Check CIDR overlap
	}

	return toBeAllocatedSet, nil
}

// TODO(iiiceoo): refactor
func execAllocate(ctx context.Context, ipPoolManager ippoolmanager.IPPoolManager, toBeAllocatedSet []ToBeAllocated, ownerType types.OwnerType, containerID string, pod *corev1.Pod) ([]*models.IPConfig, map[string]string, error) {
	logger := logutils.FromContext(ctx)

	resIPs := []*models.IPConfig{}
	podAnnotations := map[string]string{}
	for _, e := range toBeAllocatedSet {
		var v4Succeed, v6Succeed bool
		annoPodAssigned := &types.AnnoPodAssignedEthxValue{
			NIC: e.NIC,
		}

		if len(e.V4PoolCandidates) != 0 {
			var errs []error
			for _, pool := range e.V4PoolCandidates {
				ipv4, err := ipPoolManager.AllocateIP(ctx, ownerType, pool, containerID, e.NIC, pod)
				if err != nil {
					errs = append(errs, err)
					logger.Sugar().Warnf("Failed to allocate IP(v4) to %s from IP pool %s: %v", e.NIC, pool, err)
					continue
				}
				logger.Sugar().Infof("Allocate IP(v4) %s to %s from IP pool %s", *ipv4.Address, e.NIC, pool)
				resIPs = append(resIPs, ipv4)

				annoPodAssigned.IPv4Pool = ipv4.IPPool
				annoPodAssigned.IPv4 = *ipv4.Address
				annoPodAssigned.Vlan = strconv.FormatInt(ipv4.Vlan, 10)

				v4Succeed = true
				break
			}

			if !v4Succeed {
				return resIPs, nil, fmt.Errorf("failed to allocate any IP(v4) to %s from IP pools %v: %v", e.NIC, e.V4PoolCandidates, utilerrors.NewAggregate(errs).Error())
			}
		}

		if len(e.V6PoolCandidates) != 0 {
			var errs []error
			for _, pool := range e.V6PoolCandidates {
				ipv6, err := ipPoolManager.AllocateIP(ctx, ownerType, pool, containerID, e.NIC, pod)
				if err != nil {
					errs = append(errs, err)
					logger.Sugar().Warnf("Failed to allocate IP(v6) to %s from IP pool %s: %v", e.NIC, pool, err)
					continue
				}
				logger.Sugar().Infof("Allocate IP(v6) %s to %s from IP pool %s", *ipv6.Address, e.NIC, pool)
				resIPs = append(resIPs, ipv6)

				annoPodAssigned.IPv6Pool = ipv6.IPPool
				annoPodAssigned.IPv6 = *ipv6.Address
				annoPodAssigned.Vlan = strconv.FormatInt(ipv6.Vlan, 10)

				v6Succeed = true
				break
			}

			if !v6Succeed {
				return resIPs, nil, fmt.Errorf("failed to allocate any IP(v6) to %s from IP pools %v: %v", e.NIC, e.V6PoolCandidates, utilerrors.NewAggregate(errs).Error())
			}
		}

		v, err := json.Marshal(annoPodAssigned)
		if err != nil {
			return nil, nil, err
		}
		podAnnotations[constant.AnnotationPre+"/assigned-"+e.NIC] = string(v)
	}

	return resIPs, podAnnotations, nil
}
