// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var logger = logutils.Logger.Named("IPAM")

type IPAM interface {
	Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error)
	Release(ctx context.Context, delArgs *models.IpamDelArgs) error
	release(ctx context.Context, delArgs *models.IpamDelArgs) error
}

type IPAMConfig struct {
	StatuflsetIPEnable       bool
	ClusterDefaultIPv4IPPool []string
	ClusterDefaultIPv6IPPool []string
}

type ipam struct {
	ipamConfig    IPAMConfig
	ipPoolManager ippoolmanager.IPPoolManager
	weManager     workloadendpointmanager.WorkloadEndpointManager
	nsManager     namespacemanager.NamespaceManager
	podManager    podmanager.PodManager
}

func NewIPAM(c IPAMConfig, ipPoolManager ippoolmanager.IPPoolManager, weManager workloadendpointmanager.WorkloadEndpointManager, nsManager namespacemanager.NamespaceManager, podManager podmanager.PodManager) IPAM {
	return &ipam{
		ipamConfig:    c,
		ipPoolManager: ipPoolManager,
		weManager:     weManager,
		nsManager:     nsManager,
		podManager:    podManager,
	}
}

type ToBeAllocated struct {
	NIC              string
	V4PoolCandidates []string
	V6PoolCandidates []string
}

func (i *ipam) Allocate(ctx context.Context, addArgs *models.IpamAddArgs) (*models.IpamAddResponse, error) {
	logger := logger.With(zap.String("ContainerID", *addArgs.ContainerID),
		zap.String("IfName", *addArgs.IfName),
		zap.String("NetNamespace", *addArgs.NetNamespace),
		zap.String("PodNamespace", *addArgs.PodNamespace),
		zap.String("PodName", *addArgs.PodName))
	logger.Info("Start to allocate IP")

	allocation, currently, err := i.weManager.RetriveIPAllocation(ctx, *addArgs.PodNamespace, *addArgs.PodName, *addArgs.ContainerID, true)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	addResp := &models.IpamAddResponse{}
	if allocation != nil && currently {
		// TODO(iiiceoo): Return routes or DNS
		logger.Sugar().Infof("Retrieve an existing IP allocation: %+v", allocation.IPs)
		addResp.Ips = convertToIPConfigs(allocation.IPs)
		return addResp, nil
	}

	pod, err := i.podManager.GetPodByName(ctx, *addArgs.PodNamespace, *addArgs.PodName)
	if err != nil {
		logger.Sugar().Errorf("Failed to get pod: %v", err)
		return nil, err
	}

	podStatus, allocatable := i.podManager.IsIPAllocatable(ctx, pod)
	if !allocatable {
		logger.Sugar().Errorf("Pod is %s, cannot allocate IP", podStatus)
		return nil, errors.New("unable to allocate IP to an invalid pod")
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

	poolToIPConfigs := make(map[string][]*models.IPConfig)
	podAnnotations := make(map[string]string, len(toBeAllocatedSet))
	for _, e := range toBeAllocatedSet {
		var v4Succeed, v6Succeed bool
		annoPodAssigned := &constant.AnnoPodAssignedEthxValue{
			NIC: e.NIC,
		}

		if len(e.V4PoolCandidates) != 0 {
			for _, pool := range e.V4PoolCandidates {
				ipv4, err := i.ipPoolManager.AllocateIP(ctx, ownerType, pool, *addArgs.ContainerID, *addArgs.IfName, pod)
				if err != nil {
					logger.Sugar().Warnf("Failed to allocate IP(v4) to %s from IP pool %s: %v", e.NIC, pool, err)
					continue
				}
				logger.Sugar().Infof("Allocate IP(v4) %s to %s from IP pool %s", *ipv4.Address, e.NIC, pool)
				poolToIPConfigs[pool] = append(poolToIPConfigs[pool], ipv4)

				annoPodAssigned.IPv4Pool = pool
				annoPodAssigned.IPv4 = *ipv4.Address
				annoPodAssigned.Vlan = strconv.FormatInt(ipv4.Vlan, 10)

				v4Succeed = true
				break
			}

			if !v4Succeed {
				logger.Sugar().Errorf("Failed to allocate any IP(v4) to %s from IP pools: %v", e.NIC, e.V4PoolCandidates)
				return nil, errors.New("failed to allocate any IP(v4) successfully")
			}
		}

		if len(e.V6PoolCandidates) != 0 {
			for _, pool := range e.V6PoolCandidates {
				ipv6, err := i.ipPoolManager.AllocateIP(ctx, ownerType, pool, *addArgs.ContainerID, *addArgs.IfName, pod)
				if err != nil {
					logger.Sugar().Warnf("Failed to allocate IP(v6) to %s from IP pool %s: %v", e.NIC, pool, err)
					continue
				}
				logger.Sugar().Infof("Allocate IP(v6) %s to %s from IP pool %s", *ipv6.Address, e.NIC, pool)
				poolToIPConfigs[pool] = append(poolToIPConfigs[pool], ipv6)

				annoPodAssigned.IPv6Pool = pool
				annoPodAssigned.IPv6 = *ipv6.Address
				annoPodAssigned.Vlan = strconv.FormatInt(ipv6.Vlan, 10)

				v6Succeed = true
				break
			}

			if !v6Succeed {
				logger.Sugar().Errorf("Failed to allocate any IP(v6) to %s from IP pools: %v", e.NIC, e.V6PoolCandidates)
				return nil, errors.New("failed to allocate any IP(v6) successfully")
			}
		}

		v, err := json.Marshal(annoPodAssigned)
		if err != nil {
			return nil, err
		}
		podAnnotations[constant.AnnotationPre+"/assigned-"+e.NIC] = string(v)
	}

	for _, v := range poolToIPConfigs {
		addResp.Ips = append(addResp.Ips, v...)
	}

	if err := i.podManager.MergeAnnotations(ctx, pod, podAnnotations); err != nil {
		logger.Sugar().Errorf("Failed to merge the IP assignment annotation of pod: %v", err)
		return nil, err
	}

	if err := i.weManager.UpdateIPAllocation(ctx, *addArgs.PodNamespace, *addArgs.PodName, &spiderpoolv1.PodIPAllocation{
		ContainerID:  *addArgs.ContainerID,
		Node:         pod.Spec.NodeName,
		IPs:          convertToIPAllocationDetails(poolToIPConfigs),
		CreationTime: metav1.Time{Time: time.Now()},
	}); err != nil {
		logger.Sugar().Errorf("Failed to update the IP allocation of workload endpoint: %v", err)
		return nil, err
	}

	logger.Sugar().Infof("Succeed to allocate: %+v", addResp)

	return addResp, nil
}

func (i *ipam) Release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	return i.release(ctx, delArgs)
}

func (i *ipam) release(ctx context.Context, delArgs *models.IpamDelArgs) error {
	return nil
}

func convertToIPConfigs(details []spiderpoolv1.IPAllocationDetail) []*models.IPConfig {
	var ipConfigs []*models.IPConfig
	for _, d := range details {
		version := new(int64)
		var address *string
		if d.IPv4 != nil {
			*version = 4
			address = d.IPv4
		} else {
			*version = 6
			address = d.IPv6
		}

		ipConfigs = append(ipConfigs, &models.IPConfig{
			Address: address,
			Gateway: *d.Gateway,
			Nic:     &d.NIC,
			Version: version,
			Vlan:    int64(*d.Vlan),
		})
	}

	return ipConfigs
}

func convertToIPAllocationDetails(poolToIPConfigs map[string][]*models.IPConfig) []spiderpoolv1.IPAllocationDetail {
	var details []spiderpoolv1.IPAllocationDetail
	for pool, ipConfigs := range poolToIPConfigs {
		for _, c := range ipConfigs {
			var ipv4Pool, ipv6Pool *string
			var ipv4, ipv6 *string
			if *c.Version == 4 {
				ipv4Pool = &pool
				ipv4 = c.Address
			} else {
				ipv4Pool = &pool
				ipv6 = c.Address
			}

			var vlan *spiderpoolv1.Vlan
			if c.Vlan != 0 {
				vlan := new(spiderpoolv1.Vlan)
				*vlan = spiderpoolv1.Vlan(c.Vlan)
			}

			details = append(details, spiderpoolv1.IPAllocationDetail{
				NIC:      *c.Nic,
				IPv4:     ipv4,
				IPv6:     ipv6,
				IPv4Pool: ipv4Pool,
				IPv6Pool: ipv6Pool,
				Vlan:     vlan,
				Gateway:  &c.Gateway,
				// TODO(iiiceoo): Routes
			})
		}
	}

	return details
}

func getPodIPPoolCandidates(ctx context.Context, nsManager namespacemanager.NamespaceManager, ipamConfig IPAMConfig, nic string, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	// TODO(iiiceoo): Validate annotations
	var preliminary []*ToBeAllocated
	if anno, ok := pod.Annotations[constant.AnnoPodIPPools]; ok {
		var annoPodIPPools constant.AnnoPodIPPoolsValue
		err := json.Unmarshal([]byte(anno), &annoPodIPPools)
		if err != nil {
			return nil, err
		}

		for _, i := range annoPodIPPools {
			preliminary = append(preliminary, &ToBeAllocated{
				NIC:              i.NIC,
				V4PoolCandidates: i.IPv4Pools,
				V6PoolCandidates: i.IPv6Pools,
			})
		}
	} else if anno, ok := pod.Annotations[constant.AnnoPodIPPool]; ok {
		var annoPodIPPool constant.AnnoPodIPPoolValue
		if err := json.Unmarshal([]byte(anno), &annoPodIPPool); err != nil {
			return nil, err
		}

		if annoPodIPPool.NIC != nil && *annoPodIPPool.NIC != nic {
			return nil, errors.New("the interface of pod annotaiton is not same with the runtime request")
		}

		preliminary = append(preliminary, &ToBeAllocated{
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
			preliminary = append(preliminary, &ToBeAllocated{
				NIC:              nic,
				V4PoolCandidates: nsDefautlV4Pools,
				V6PoolCandidates: nsDefautlV4Pools,
			})
		} else if !(ipamConfig.ClusterDefaultIPv4IPPool == nil && ipamConfig.ClusterDefaultIPv6IPPool == nil) {
			preliminary = append(preliminary, &ToBeAllocated{
				NIC:              nic,
				V4PoolCandidates: ipamConfig.ClusterDefaultIPv4IPPool,
				V6PoolCandidates: ipamConfig.ClusterDefaultIPv6IPPool,
			})
		} else {
			return nil, errors.New("no default IP pools are specified")
		}
	}

	return preliminary, nil
}

func filterPodIPPoolCandidates(ctx context.Context, ipPoolManager ippoolmanager.IPPoolManager, toBeAllocatedSet []*ToBeAllocated, pod *corev1.Pod) ([]*ToBeAllocated, error) {
	for _, e := range toBeAllocatedSet {
		allPools := append(e.V4PoolCandidates, e.V6PoolCandidates...)
		same, err := ipPoolManager.CheckVlanSame(ctx, allPools)
		if err != nil {
			return nil, err
		}
		if !same {
			logger.Sugar().Errorf("VLANs of IP pools %v are not same", allPools)
			return nil, errors.New("vlans are not same")
		}

		// TODO(iiiceoo): Check v4 v6 enable
		// TODO(iiiceoo): Check CIDR overlap
	}

	for idx, e := range toBeAllocatedSet {
		for idx, pool := range e.V4PoolCandidates {
			eligible, err := ipPoolManager.SelectByPod(ctx, spiderpoolv1.IPv4, pool, pod)
			if err != nil {
				return nil, err
			}
			if !eligible {
				e.V4PoolCandidates = append(e.V4PoolCandidates[:idx], e.V4PoolCandidates[idx+1:]...)
			}
		}

		for _, pool := range e.V6PoolCandidates {
			eligible, err := ipPoolManager.SelectByPod(ctx, spiderpoolv1.IPv6, pool, pod)
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
		return nil, errors.New("all IP pools filtered out")
	}

	return toBeAllocatedSet, nil
}
