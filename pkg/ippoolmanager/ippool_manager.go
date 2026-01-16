// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"net"
	"path/filepath"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	"github.com/spidernet-io/spiderpool/pkg/utils/retry"
)

type IPPoolManager interface {
	GetIPPoolByName(ctx context.Context, poolName string, cached bool) (*spiderpoolv2beta1.SpiderIPPool, error)
	ListIPPools(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderIPPoolList, error)
	AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error
	UpdateAllocatedIPs(ctx context.Context, poolName, namespacedName string, ipAndCIDs []types.IPAndUID) error
	ParseWildcardPoolNameList(ctx context.Context, PoolNames []string, ipVersion types.IPVersion) (newPoolNames []string, hasWildcard bool, err error)
}

type ipPoolManager struct {
	config     IPPoolManagerConfig
	client     client.Client
	apiReader  client.Reader
	rIPManager reservedipmanager.ReservedIPManager
}

func NewIPPoolManager(config IPPoolManagerConfig, client client.Client, apiReader client.Reader, rIPManager reservedipmanager.ReservedIPManager) (IPPoolManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}
	if rIPManager == nil {
		return nil, fmt.Errorf("reserved-IP manager %w", constant.ErrMissingRequiredParam)
	}

	return &ipPoolManager{
		config:     setDefaultsForIPPoolManagerConfig(config),
		client:     client,
		apiReader:  apiReader,
		rIPManager: rIPManager,
	}, nil
}

func (im *ipPoolManager) GetIPPoolByName(ctx context.Context, poolName string, cached bool) (*spiderpoolv2beta1.SpiderIPPool, error) {
	reader := im.apiReader
	if cached == constant.UseCache {
		reader = im.client
	}

	var ipPool spiderpoolv2beta1.SpiderIPPool
	if err := reader.Get(ctx, apitypes.NamespacedName{Name: poolName}, &ipPool); err != nil {
		return nil, err
	}

	return &ipPool, nil
}

func (im *ipPoolManager) ListIPPools(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderIPPoolList, error) {
	reader := im.apiReader
	if cached == constant.UseCache {
		reader = im.client
	}

	var ipPoolList spiderpoolv2beta1.SpiderIPPoolList
	if err := reader.List(ctx, &ipPoolList, opts...); err != nil {
		return nil, err
	}

	return &ipPoolList, nil
}

func (im *ipPoolManager) AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, error) {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	var ipConfig *models.IPConfig
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)
		logger.Debug("Re-get IPPool for IP allocation")
		ipPool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}

		logger.Debug("Generate a random IP address")
		allocatedIP, err := im.genRandomIP(ctx, ipPool, pod, podController)
		if err != nil {
			return err
		}

		resourceVersion := ipPool.ResourceVersion
		logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).
			Sugar().Debugf("Try to update the allocation status of IPPool using random IP %s", allocatedIP)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if apierrors.IsConflict(err) {
				metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
				logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).Warn("An conflict occurred when updating the status of IPPool")
			}
			return err
		}
		ipConfig = convert.GenIPConfigResult(allocatedIP, nic, ipPool)

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to allocate IP from IPPool %s", constant.ErrRetriesExhausted, steps, poolName)
		}

		return nil, err
	}
	// TODO(@cyclinder): set these values from ippool.spec
	ipConfig.EnableGatewayDetection = im.config.EnableGatewayDetection
	ipConfig.EnableIPConflictDetection = im.config.EnableIPConflictDetection

	return ipConfig, nil
}

func (im *ipPoolManager) genRandomIP(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool, pod *corev1.Pod, podController types.PodTopController) (net.IP, error) {
	logger := logutils.FromContext(ctx)

	var tmpPod *corev1.Pod
	if im.config.EnableKubevirtStaticIP && podController.APIVersion == kubevirtv1.SchemeGroupVersion.String() && podController.Kind == constant.KindKubevirtVMI {
		tmpPod = pod.DeepCopy()
		tmpPod.SetName(podController.Name)
	} else {
		tmpPod = pod
	}
	key, err := cache.MetaNamespaceKeyFunc(tmpPod)
	if err != nil {
		return nil, err
	}

	reservedIPs, err := im.rIPManager.AssembleReservedIPs(ctx, *ipPool.Spec.IPVersion)
	if err != nil {
		return nil, err
	}

	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
	if err != nil {
		return nil, err
	}

	var used []string
	for ip, record := range allocatedRecords {
		// In a multi-NIC scenario, if one of the NIC pools does not have enough IPs, an allocation failure message will be displayed.
		// However, other IP pools still have IPs, which will cause IPs in other pools to be exhausted.
		// Check if there is a duplicate Pod UID in IPPool.allocatedRecords.
		// If so, we skip this allocation and assume that this Pod has already obtained an IP address in the pool.
		if record.PodUID == string(pod.UID) {
			logger.Sugar().Infof("The Pod %s/%s UID %s already exists in the assigned IP %s", pod.Namespace, pod.Name, ip, string(pod.UID))
			return net.ParseIP(ip), nil
		}
		used = append(used, ip)
	}

	usedIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, used)
	if err != nil {
		return nil, err
	}

	unAvailableIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return nil, err
	}

	availableIPs := spiderpoolip.FindAvailableIPs(ipPool.Spec.IPs, append(unAvailableIPs, append(reservedIPs, usedIPs...)...), 1)
	if len(availableIPs) == 0 {
		// traverse the usedIPs to find the previous allocated IPs if there be
		// reference issue: https://github.com/spidernet-io/spiderpool/issues/2517
		allocatedIPFromRecords, hasFound := findAllocatedIPFromRecords(allocatedRecords, key, string(pod.UID))
		if !hasFound {
			return nil, constant.ErrIPUsedOut
		}

		availableIPs, err = spiderpoolip.ParseIPRange(*ipPool.Spec.IPVersion, allocatedIPFromRecords)
		if nil != err {
			return nil, err
		}
		logger.Sugar().Warnf("find previous IP '%s' from IPPool '%s' recorded IP allocations", allocatedIPFromRecords, ipPool.Name)
	}
	resIP := availableIPs[0]

	if allocatedRecords == nil {
		allocatedRecords = spiderpoolv2beta1.PoolIPAllocations{}
	}
	allocatedRecords[resIP.String()] = spiderpoolv2beta1.PoolIPAllocation{
		NamespacedName: key,
		PodUID:         string(pod.UID),
	}

	data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
	if err != nil {
		return nil, err
	}
	ipPool.Status.AllocatedIPs = data

	if ipPool.Status.AllocatedIPCount == nil {
		ipPool.Status.AllocatedIPCount = new(int64)
	}

	// reference issue: https://github.com/spidernet-io/spiderpool/issues/3771
	if int64(len(usedIPs)) != *ipPool.Status.AllocatedIPCount {
		logger.Sugar().Errorf("Handling AllocatedIPCount while allocating IP from IPPool %s, but there is a data discrepancy. Expected %d, but got %d.", ipPool.Name, len(usedIPs), *ipPool.Status.AllocatedIPCount)
	}

	// Adding a newly assigned IP
	*ipPool.Status.AllocatedIPCount = int64(len(usedIPs)) + 1

	if *ipPool.Status.AllocatedIPCount > int64(*im.config.MaxAllocatedIPs) {
		return nil, fmt.Errorf("%w, threshold of IP records(<=%d) for IPPool %s exceeded", constant.ErrIPUsedOut, im.config.MaxAllocatedIPs, ipPool.Name)
	}

	return resIP, nil
}

func (im *ipPoolManager) ReleaseIP(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)
		logger.Debug("Re-get IPPool for IP release")
		ipPool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}

		allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
		if err != nil {
			return err
		}

		if ipPool.Status.AllocatedIPCount == nil {
			ipPool.Status.AllocatedIPCount = new(int64)
		}

		// reference issue: https://github.com/spidernet-io/spiderpool/issues/3771
		if int64(len(allocatedRecords)) != *ipPool.Status.AllocatedIPCount {
			logger.Sugar().Errorf("Handling AllocatedIPCount while releasing IP from IPPool %s, but there is a data discrepancy. Expected %d, but got %d.", ipPool.Name, len(allocatedRecords), *ipPool.Status.AllocatedIPCount)
		}

		release := false
		for _, iu := range ipAndUIDs {
			if record, ok := allocatedRecords[iu.IP]; ok {
				if record.PodUID == iu.UID {
					delete(allocatedRecords, iu.IP)
					*ipPool.Status.AllocatedIPCount = int64(len(allocatedRecords))
					release = true
				}
			}
		}

		if !release {
			return nil
		}

		data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
		if err != nil {
			return err
		}
		ipPool.Status.AllocatedIPs = data

		resourceVersion := ipPool.ResourceVersion
		logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).
			Sugar().Debugf("Try to clean the IP allocation records of IPPool with IP addresses %+v", ipAndUIDs)
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if apierrors.IsConflict(err) {
				metric.IpamReleaseUpdateIPPoolConflictCounts.Add(ctx, 1)
				logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).Warn("An conflict occurred when cleaning the IP allocation records of IPPool")
			}
			return err
		}

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to release IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, steps, ipAndUIDs, poolName)
		}
		return err
	}

	return nil
}

func (im *ipPoolManager) UpdateAllocatedIPs(ctx context.Context, poolName, namespacedName string, ipAndUIDs []types.IPAndUID) error {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)

		ipPool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}

		allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
		if err != nil {
			return err
		}

		recreate := false
		for _, iu := range ipAndUIDs {
			if record, ok := allocatedRecords[iu.IP]; ok {
				if record.NamespacedName != namespacedName {
					return fmt.Errorf("failed to update allocated IP because of data broken: IPPool %s IP %s allocation detail %v mistach namespacedName %s",
						poolName, iu.IP, record, namespacedName)
				}
				if record.PodUID != iu.UID {
					record.PodUID = iu.UID
					allocatedRecords[iu.IP] = record
					recreate = true
				}
			}
		}

		if !recreate {
			return nil
		}

		data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
		if err != nil {
			return err
		}
		ipPool.Status.AllocatedIPs = data

		resourceVersion := ipPool.ResourceVersion
		if err := im.client.Status().Update(ctx, ipPool); err != nil {
			if apierrors.IsConflict(err) {
				metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
				logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).Warn("An conflict occurred when updating the status of IPPool")
			}
			return err
		}

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to re-allocate the IP addresses %+v from IPPool %s", constant.ErrRetriesExhausted, steps, ipAndUIDs, poolName)
		}
		return err
	}

	return nil
}

func (im *ipPoolManager) ParseWildcardPoolNameList(ctx context.Context, poolNamesArr []string, ipVersion types.IPVersion) (newPoolNames []string, hasWildcard bool, err error) {
	if HasWildcardInSlice(poolNamesArr) {
		var ipVersionStr string
		if ipVersion == constant.IPv4 {
			ipVersionStr = constant.Str4
		} else {
			ipVersionStr = constant.Str6
		}

		poolList, err := im.ListIPPools(ctx, constant.UseCache, client.MatchingFields{constant.SpecIPVersionField: ipVersionStr})
		if nil != err {
			return nil, false, err
		}

		newPoolNamesArr := []string{}
		for _, tmpStr := range poolNamesArr {
			if HasWildcardInStr(tmpStr) {
				for _, tmpPool := range poolList.Items {
					isMatch, err := filepath.Match(tmpStr, tmpPool.Name)
					if nil != err {
						return nil, false, fmt.Errorf("failed to match wildcard: IPv%d PoolName pattern '%s', character '%s', error: %w", ipVersion, tmpStr, tmpPool.Name, err)
					}
					// wildcard matches
					if isMatch {
						newPoolNamesArr = append(newPoolNamesArr, tmpPool.Name)
					}
				}
			} else {
				// original IPPool name
				newPoolNamesArr = append(newPoolNamesArr, tmpStr)
			}
		}

		return newPoolNamesArr, true, nil
	}

	return poolNamesArr, false, nil
}
