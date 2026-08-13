// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"math"
	"net"
	"path/filepath"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
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
	// AllocateIP returns the allocated IPConfig and a bool that is true when
	// the IP was selected via the pool's provider-written prewarm metadata
	// (status.ipMetaData.metadata). Callers use this to skip the redundant
	// synchronous IaaS provider allocation call for prewarmed IPs (FR-015).
	AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, bool, error)
	// AllocateIPPair allocates one IPv4/IPv6 address pair for the Pod from a
	// paired dual-stack IaaS primary (v4) pool and its sibling v6 pool,
	// sourcing BOTH addresses from the SAME status.ipMetaData.metadata entry
	// of the primary pool so cross-entry mixing is structurally impossible
	// (spec.md SC-004). Both returned IPConfigs are metadata-sourced, so the
	// caller must skip the synchronous IaaS provider allocation call for
	// them (FR-015).
	AllocateIPPair(ctx context.Context, poolName, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, *models.IPConfig, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndUIDs []types.IPAndUID) error
	UpdateAllocatedIPs(ctx context.Context, poolName, namespacedName string, ipAndCIDs []types.IPAndUID) error
	ParseWildcardPoolNameList(ctx context.Context, PoolNames []string, ipVersion types.IPVersion) (newPoolNames []string, hasWildcard bool, err error)
}

type ipPoolManager struct {
	config        IPPoolManagerConfig
	client        client.Client
	apiReader     client.Reader
	rIPManager    reservedipmanager.ReservedIPManager
	metadataCache *metadataSnapshotCache
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
		config:        setDefaultsForIPPoolManagerConfig(config),
		client:        client,
		apiReader:     apiReader,
		rIPManager:    rIPManager,
		metadataCache: newMetadataSnapshotCache(),
	}, nil
}

func SetupIPMetadataCache(ctx context.Context, manager IPPoolManager, runtimeCache ctrlcache.Cache) error {
	if runtimeCache == nil {
		return fmt.Errorf("runtime cache %w", constant.ErrMissingRequiredParam)
	}
	im, ok := manager.(*ipPoolManager)
	if !ok {
		return fmt.Errorf("IPPool manager %w", constant.ErrWrongInput)
	}
	return im.metadataCache.register(ctx, runtimeCache)
}

// SyncIPMetadataCache processes one authoritative informer object. Normal
// agent operation invokes the same update path through SetupIPMetadataCache.
func SyncIPMetadataCache(manager IPPoolManager, pool *spiderpoolv2beta1.SpiderIPPool) error {
	im, ok := manager.(*ipPoolManager)
	if !ok {
		return fmt.Errorf("IPPool manager %w", constant.ErrWrongInput)
	}
	im.metadataCache.update(pool)
	return nil
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

func (im *ipPoolManager) AllocateIP(ctx context.Context, poolName, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, bool, error) {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	var ipConfig *models.IPConfig
	var fromIPMetadata bool
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
		allocatedIP, metadataSourced, metadataEntry, err := im.genRandomIP(ctx, ipPool, pod, podController)
		if err != nil {
			return err
		}
		fromIPMetadata = metadataSourced

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
		// Copy the matched metadata entry's MAC/VLAN onto the resulting
		// Pod interface config, the same way a synchronous provider-allocate
		// response is merged in for non-prewarmed IPs (data-model.md §1.3).
		if metadataSourced && metadataEntry != nil {
			if metadataEntry.MAC != "" {
				ipConfig.Mac = metadataEntry.MAC
			}
			if metadataEntry.VLAN != nil {
				ipConfig.Vlan = int64(*metadataEntry.VLAN)
			}
		}

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to allocate IP from IPPool %s", constant.ErrRetriesExhausted, steps, poolName)
		}

		return nil, false, err
	}
	// TODO(@cyclinder): set these values from ippool.spec
	ipConfig.EnableGatewayDetection = im.config.EnableGatewayDetection
	ipConfig.EnableIPConflictDetection = im.config.EnableIPConflictDetection

	return ipConfig, fromIPMetadata, nil
}

// AllocateIPPair implements the pair-or-nothing allocation model for paired
// dual-stack IaaS pools: it selects ONE entry from the v4 primary pool's
// status.ipMetaData.metadata whose both sides are currently available, then
// records the entry's IPv4 key in the primary pool's status.allocatedIPs and
// the entry's ipv6 value in the sibling pool's status.allocatedIPs. The v6
// address is never selected independently, so an IPv4/IPv6 pair can never
// mix addresses from two different metadata entries (spec.md SC-004).
//
// The two status writes cannot be transactional across two custom
// resources. Instead, any write conflict restarts the whole round through
// the standard retry loop: the Pod-UID fast path then finds the
// already-committed v4 record, resolves the SAME metadata entry from it,
// and only completes the missing v6 side. Convergence is guaranteed because
// no other Pod can claim the entry's v6 address without first claiming its
// v4 key, which is already owned by this Pod. If retries are exhausted, any
// committed half-pair is cleaned up by the caller's ordinary
// failure/release handling and the IP GC.
func (im *ipPoolManager) AllocateIPPair(ctx context.Context, poolName, nic string, pod *corev1.Pod, podController types.PodTopController) (*models.IPConfig, *models.IPConfig, error) {
	logger := logutils.FromContext(ctx)

	backoff := retry.DefaultRetry
	steps := backoff.Steps
	var v4Config, v6Config *models.IPConfig
	err := retry.RetryOnConflictWithContext(ctx, backoff, func(ctx context.Context) error {
		logger := logger.With(
			zap.String("IPPoolName", poolName),
			zap.Int("Times", steps-backoff.Steps+1),
		)
		logger.Debug("Re-get the paired IPPools for IP pair allocation")
		v4Pool, err := im.GetIPPoolByName(ctx, poolName, constant.IgnoreCache)
		if err != nil {
			return err
		}
		if !IsPairedIaaSPrimaryPool(v4Pool) {
			return fmt.Errorf("%w: IPPool %s is not a paired IaaS v4 primary pool", constant.ErrWrongInput, poolName)
		}
		pairName := v4Pool.Annotations[constant.AnnoIPPoolPairPool]
		v6Pool, err := im.GetIPPoolByName(ctx, pairName, constant.IgnoreCache)
		if err != nil {
			return fmt.Errorf("failed to get pair IPPool %s of %s: %w", pairName, poolName, err)
		}

		key, err := im.podNamespacedKey(pod, podController)
		if err != nil {
			return err
		}

		ipMetadata, err := im.metadataCache.snapshot(v4Pool)
		if err != nil {
			return err
		}

		v4Records, err := convert.UnmarshalIPPoolAllocatedIPs(v4Pool.Status.AllocatedIPs)
		if err != nil {
			return err
		}
		v6Records, err := convert.UnmarshalIPPoolAllocatedIPs(v6Pool.Status.AllocatedIPs)
		if err != nil {
			return err
		}

		// Convergence fast path: a previous round (or a conflicted retry)
		// already committed the v4 side for this Pod. Resolve the SAME
		// metadata entry from the recorded v4 key instead of selecting a
		// new one, so the retry can only complete the pair, never re-pair.
		var entry *spiderpoolv2beta1.IPMetadataEntry
		var v4IP, v6IP net.IP
		for ip, record := range v4Records {
			if record.PodUID != string(pod.UID) {
				continue
			}
			e, ok := ipMetadata[ip]
			if !ok || e.IPv6 == nil || net.ParseIP(*e.IPv6) == nil {
				return fmt.Errorf("%w: recorded IP %s of Pod %s has no valid pair entry in IPPool %s metadata",
					constant.ErrIPMetadataNotReady, ip, key, poolName)
			}
			entry = &e
			v4IP = net.ParseIP(ip)
			v6IP = net.ParseIP(*e.IPv6)
			logger.Sugar().Infof("The Pod %s UID %s already owns the v4 side %s of a pair in IPPool %s, completing the pair", key, string(pod.UID), ip, poolName)
			break
		}

		if entry == nil {
			v4Candidates, err := im.availablePoolIPs(ctx, v4Pool, v4Records)
			if err != nil {
				return err
			}
			v6Available, err := im.availablePoolIPs(ctx, v6Pool, v6Records)
			if err != nil {
				return err
			}

			e, v4Sel, v6Sel, ok := FindReadyIPPairMetadata(ipMetadata, v4Candidates, v6Available)
			if !ok {
				return constant.ErrIPUsedOut
			}
			entry, v4IP, v6IP = e, v4Sel, v6Sel
		}

		if _, ok := v4Records[v4IP.String()]; !ok {
			if err := im.commitAllocatedIP(ctx, logger, v4Pool, v4Records, v4IP, key, pod); err != nil {
				return err
			}
		}
		if _, ok := v6Records[v6IP.String()]; !ok {
			if err := im.commitAllocatedIP(ctx, logger, v6Pool, v6Records, v6IP, key, pod); err != nil {
				return err
			}
		}

		v4Config = convert.GenIPConfigResult(v4IP, nic, v4Pool)
		v6Config = convert.GenIPConfigResult(v6IP, nic, v6Pool)
		for _, cfg := range []*models.IPConfig{v4Config, v6Config} {
			if entry.MAC != "" {
				cfg.Mac = entry.MAC
			}
			if entry.VLAN != nil {
				cfg.Vlan = int64(*entry.VLAN)
			}
		}

		return nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			err = fmt.Errorf("%w (%d times), failed to allocate an IP pair from IPPool %s", constant.ErrRetriesExhausted, steps, poolName)
		}
		return nil, nil, err
	}

	for _, cfg := range []*models.IPConfig{v4Config, v6Config} {
		cfg.EnableGatewayDetection = im.config.EnableGatewayDetection
		cfg.EnableIPConflictDetection = im.config.EnableIPConflictDetection
	}

	return v4Config, v6Config, nil
}

// podNamespacedKey returns the namespaced key recorded in
// status.allocatedIPs, honoring the KubeVirt static-IP convention of keying
// by the VMI name instead of the launcher Pod name.
func (im *ipPoolManager) podNamespacedKey(pod *corev1.Pod, podController types.PodTopController) (string, error) {
	tmpPod := pod
	if im.config.EnableKubevirtStaticIP && podController.APIVersion == kubevirtv1.SchemeGroupVersion.String() && podController.Kind == constant.KindKubevirtVMI {
		tmpPod = pod.DeepCopy()
		tmpPod.SetName(podController.Name)
	}
	return cache.MetaNamespaceKeyFunc(tmpPod)
}

// availablePoolIPs computes the pool's normal available-candidate set:
// spec.ips minus excludeIPs, cluster reserved IPs and the given allocated
// records — the same computation genRandomIP performs for an IaaS pool.
func (im *ipPoolManager) availablePoolIPs(ctx context.Context, pool *spiderpoolv2beta1.SpiderIPPool, allocatedRecords spiderpoolv2beta1.PoolIPAllocations) ([]net.IP, error) {
	if pool.Spec.IPVersion == nil {
		return nil, fmt.Errorf("%w: IPPool %s has no spec.ipVersion", constant.ErrWrongInput, pool.Name)
	}

	reservedIPs, err := im.rIPManager.AssembleReservedIPs(ctx, *pool.Spec.IPVersion)
	if err != nil {
		return nil, err
	}

	var used []string
	for ip := range allocatedRecords {
		used = append(used, ip)
	}
	usedIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, used)
	if err != nil {
		return nil, err
	}
	excludeIPs, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, pool.Spec.ExcludeIPs)
	if err != nil {
		return nil, err
	}
	excluded := append(excludeIPs, append(reservedIPs, usedIPs...)...)

	return spiderpoolip.FindAvailableIPs(pool.Spec.IPs, excluded, math.MaxInt32), nil
}

// commitAllocatedIP records ip for pod in the pool's status.allocatedIPs and
// writes the status subresource, mirroring the bookkeeping of genRandomIP +
// AllocateIP's status update for a single pool.
func (im *ipPoolManager) commitAllocatedIP(ctx context.Context, logger *zap.Logger, pool *spiderpoolv2beta1.SpiderIPPool, allocatedRecords spiderpoolv2beta1.PoolIPAllocations, ip net.IP, key string, pod *corev1.Pod) error {
	if allocatedRecords == nil {
		allocatedRecords = spiderpoolv2beta1.PoolIPAllocations{}
	}
	allocatedRecords[ip.String()] = spiderpoolv2beta1.PoolIPAllocation{
		NamespacedName: key,
		PodUID:         string(pod.UID),
	}

	data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
	if err != nil {
		return err
	}
	pool.Status.AllocatedIPs = data

	if pool.Status.AllocatedIPCount == nil {
		pool.Status.AllocatedIPCount = new(int64)
	}
	*pool.Status.AllocatedIPCount = int64(len(allocatedRecords))
	if *pool.Status.AllocatedIPCount > int64(*im.config.MaxAllocatedIPs) {
		return fmt.Errorf("%w, threshold of IP records(<=%d) for IPPool %s exceeded", constant.ErrIPUsedOut, im.config.MaxAllocatedIPs, pool.Name)
	}

	resourceVersion := pool.ResourceVersion
	logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).
		Sugar().Debugf("Try to update the allocation status of IPPool %s using IP %s", pool.Name, ip)
	if err := im.client.Status().Update(ctx, pool); err != nil {
		if apierrors.IsConflict(err) {
			metric.IpamAllocationUpdateIPPoolConflictCounts.Add(ctx, 1)
			logger.With(zap.String("IPPool-ResourceVersion", resourceVersion)).
				Sugar().Warnf("An conflict occurred when updating the status of IPPool %s", pool.Name)
		}
		return err
	}

	return nil
}

// genRandomIP selects the next IP address to allocate from ipPool for pod,
// returning (address, fromIPMetadata, metadataEntry, error). Performance
// invariant: for pools WITHOUT the ipam.spidernet.io/iaas-provider label,
// this function performs exactly the same work as before this feature — zero
// added API calls or latency (plan.md "Performance Goals"). Whether
// metadata-gating applies is decided solely by the iaas-provider label,
// never by whether status.ipMetaData.metadata happens to be empty
// (data-model.md §1.3, contracts/spiderippool-iaas-extension.md rule 2).
func (im *ipPoolManager) genRandomIP(ctx context.Context, ipPool *spiderpoolv2beta1.SpiderIPPool, pod *corev1.Pod, podController types.PodTopController) (net.IP, bool, *spiderpoolv2beta1.IPMetadataEntry, error) {
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
		return nil, false, nil, err
	}

	reservedIPs, err := im.rIPManager.AssembleReservedIPs(ctx, *ipPool.Spec.IPVersion)
	if err != nil {
		return nil, false, nil, err
	}

	allocatedRecords, err := convert.UnmarshalIPPoolAllocatedIPs(ipPool.Status.AllocatedIPs)
	if err != nil {
		return nil, false, nil, err
	}

	isIaaSPool := IsIaaSPool(ipPool)

	// The metadata to intersect against is always the pool's OWN
	// ipMetaData.metadata (the primary/v4 pool of a pair, or an unpaired
	// single-stack IaaS pool). The sibling (v6) pool of a pair never
	// allocates through this path: it is filtered out of the Pod's pool
	// candidates, and both families of a pair are allocated together from
	// the primary pool via AllocateIPPair
	// (contracts/spiderippool-iaas-extension.md
	// "Single-Metadata-On-Primary-Pool Model").
	var ipMetadata map[string]spiderpoolv2beta1.IPMetadataEntry
	if isIaaSPool {
		ipMetadata, err = im.metadataCache.snapshot(ipPool)
		if err != nil {
			return nil, false, nil, err
		}
	}

	var used []string
	for ip, record := range allocatedRecords {
		// In a multi-NIC scenario, if one of the NIC pools does not have enough IPs, an allocation failure message will be displayed.
		// However, other IP pools still have IPs, which will cause IPs in other pools to be exhausted.
		// Check if there is a duplicate Pod UID in IPPool.allocatedRecords.
		// If so, we skip this allocation and assume that this Pod has already obtained an IP address in the pool.
		if record.PodUID == string(pod.UID) {
			logger.Sugar().Infof("The Pod %s/%s UID %s already exists in the assigned IP %s", pod.Namespace, pod.Name, ip, string(pod.UID))
			return net.ParseIP(ip), IsIPMetadataAddress(ipMetadata, ip), nil, nil
		}
		used = append(used, ip)
	}

	usedIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, used)
	if err != nil {
		return nil, false, nil, err
	}

	unAvailableIPs, err := spiderpoolip.ParseIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.ExcludeIPs)
	if err != nil {
		return nil, false, nil, err
	}
	excluded := append(unAvailableIPs, append(reservedIPs, usedIPs...)...)

	// Performance invariant: pools without the iaas-provider label keep the
	// original count=1 early-exit fast path; only IaaS-labeled pools need
	// the full available-candidate set to compute the readiness
	// intersection.
	findCount := 1
	if isIaaSPool {
		findCount = math.MaxInt32
	}
	availableIPs := spiderpoolip.FindAvailableIPs(ipPool.Spec.IPs, excluded, findCount)

	var resIP net.IP
	var fromIPMetadata bool
	var metadataEntry *spiderpoolv2beta1.IPMetadataEntry

	if isIaaSPool {
		// The selection model is the INTERSECTION of the normal spec.ips-derived
		// candidate set with the addresses in status.ipMetaData.metadata,
		// not a replacement of it (data-model.md §1.3). If the intersection
		// is empty -- including a freshly-created pool with no metadata
		// entries yet -- this falls through to the same previous-record
		// reuse check as the non-iaas path, then returns the ordinary
		// ErrIPUsedOut.
		entry, addr, ok := FindReadyIPMetadata(ipMetadata, types.IPVersion(*ipPool.Spec.IPVersion), availableIPs)
		if !ok {
			allocatedIPFromRecords, hasFound := findAllocatedIPFromRecords(allocatedRecords, key, string(pod.UID))
			if !hasFound {
				return nil, false, nil, constant.ErrIPUsedOut
			}

			prevIPs, perr := spiderpoolip.ParseIPRange(*ipPool.Spec.IPVersion, allocatedIPFromRecords)
			if perr != nil {
				return nil, false, nil, perr
			}
			resIP = prevIPs[0]
			fromIPMetadata = IsIPMetadataAddress(ipMetadata, resIP.String())
			logger.Sugar().Warnf("find previous IP '%s' from IPPool '%s' recorded IP allocations", allocatedIPFromRecords, ipPool.Name)
		} else {
			resIP = net.ParseIP(addr)
			fromIPMetadata = true
			metadataEntry = entry
		}
	} else {
		if len(availableIPs) == 0 {
			// traverse the usedIPs to find the previous allocated IPs if there be
			// reference issue: https://github.com/spidernet-io/spiderpool/issues/2517
			allocatedIPFromRecords, hasFound := findAllocatedIPFromRecords(allocatedRecords, key, string(pod.UID))
			if !hasFound {
				return nil, false, nil, constant.ErrIPUsedOut
			}

			prevIPs, perr := spiderpoolip.ParseIPRange(*ipPool.Spec.IPVersion, allocatedIPFromRecords)
			if perr != nil {
				return nil, false, nil, perr
			}
			resIP = prevIPs[0]
			logger.Sugar().Warnf("find previous IP '%s' from IPPool '%s' recorded IP allocations", allocatedIPFromRecords, ipPool.Name)
		} else {
			resIP = availableIPs[0]
		}
	}

	if allocatedRecords == nil {
		allocatedRecords = spiderpoolv2beta1.PoolIPAllocations{}
	}
	allocatedRecords[resIP.String()] = spiderpoolv2beta1.PoolIPAllocation{
		NamespacedName: key,
		PodUID:         string(pod.UID),
	}

	data, err := convert.MarshalIPPoolAllocatedIPs(allocatedRecords)
	if err != nil {
		return nil, false, nil, err
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
		return nil, false, nil, fmt.Errorf("%w, threshold of IP records(<=%d) for IPPool %s exceeded", constant.ErrIPUsedOut, im.config.MaxAllocatedIPs, ipPool.Name)
	}

	return resIP, fromIPMetadata, metadataEntry, nil
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
