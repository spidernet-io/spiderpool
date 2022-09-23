// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type WorkloadEndpointManager interface {
	GetEndpointByName(ctx context.Context, namespace, podName string) (*spiderpoolv1.SpiderEndpoint, error)
	ListEndpoints(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderEndpointList, error)
	Delete(ctx context.Context, wep *spiderpoolv1.SpiderEndpoint) error
	MarkIPAllocation(ctx context.Context, containerID string, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) (*spiderpoolv1.SpiderEndpoint, error)
	PatchIPAllocation(ctx context.Context, allocation *spiderpoolv1.PodIPAllocation, we *spiderpoolv1.SpiderEndpoint) (*spiderpoolv1.SpiderEndpoint, error)
	ClearCurrentIPAllocation(ctx context.Context, containerID string, we *spiderpoolv1.SpiderEndpoint) error
	RemoveFinalizer(ctx context.Context, namespace, podName string) error
	ListAllHistoricalIPs(ctx context.Context, namespace, podName string) (map[string][]types.IPAndCID, error)
	IsIPBelongWEPCurrent(ctx context.Context, namespace, podName, poolIP string) (bool, error)
	CheckCurrentContainerID(ctx context.Context, namespace, podName, containerID string) (bool, error)
	UpdateCurrentStatus(ctx context.Context, containerID string, pod *corev1.Pod) error
}

type workloadEndpointManager struct {
	config *EndpointManagerConfig
	client client.Client
	scheme *runtime.Scheme
}

func NewWorkloadEndpointManager(c *EndpointManagerConfig, mgr ctrl.Manager) (WorkloadEndpointManager, error) {
	if c == nil {
		return nil, errors.New("endpoint manager config must be specified")
	}
	if mgr == nil {
		return nil, errors.New("k8s manager must be specified")
	}

	return &workloadEndpointManager{
		config: c,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}, nil
}

func (em *workloadEndpointManager) GetEndpointByName(ctx context.Context, namespace, podName string) (*spiderpoolv1.SpiderEndpoint, error) {
	var we spiderpoolv1.SpiderEndpoint
	if err := em.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); nil != err {
		return nil, err
	}

	return &we, nil
}

func (em *workloadEndpointManager) ListEndpoints(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderEndpointList, error) {
	var weList spiderpoolv1.SpiderEndpointList
	if err := em.client.List(ctx, &weList, opts...); err != nil {
		return nil, err
	}

	return &weList, nil
}

func (em *workloadEndpointManager) MarkIPAllocation(ctx context.Context, containerID string, we *spiderpoolv1.SpiderEndpoint, pod *corev1.Pod) (*spiderpoolv1.SpiderEndpoint, error) {
	logger := logutils.FromContext(ctx)

	allocation := &spiderpoolv1.PodIPAllocation{
		ContainerID:  containerID,
		Node:         &pod.Spec.NodeName,
		CreationTime: &metav1.Time{Time: time.Now()},
	}

	if we == nil {
		newWE := &spiderpoolv1.SpiderEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
		}

		controllerutil.AddFinalizer(newWE, constant.SpiderFinalizer)
		ownerControllerType, ownerControllerName := podmanager.GetOwnerControllerType(pod)

		// We don't set ownerReference for Endpoint when the pod belongs to StatefulSet
		// Once the StatefulSet pod restarts, we can retrieve the corresponding data immediately.
		// And we don't need to wait the corresponding Endpoint clean up to create a new one if ownerReference exists.
		if ownerControllerType != constant.OwnerStatefulSet {
			if err := controllerutil.SetOwnerReference(pod, newWE, em.scheme); err != nil {
				return nil, err
			}
		}

		if err := em.client.Create(ctx, newWE); err != nil {
			return nil, err
		}

		newWE.Status.Current = allocation
		newWE.Status.History = []spiderpoolv1.PodIPAllocation{*allocation}
		newWE.Status.OwnerControllerType = ownerControllerType
		newWE.Status.OwnerControllerName = ownerControllerName
		if err := em.client.Status().Update(ctx, newWE); err != nil {
			return nil, err
		}
		return newWE, nil
	}

	if we.Status.Current != nil && we.Status.Current.ContainerID == containerID {
		return we, nil
	}

	we.Status.Current = allocation
	we.Status.History = append([]spiderpoolv1.PodIPAllocation{*allocation}, we.Status.History...)
	if len(we.Status.History) > em.config.MaxHistoryRecords {
		logger.Sugar().Warnf("threshold of historical IP allocation records(<=%d) exceeded", em.config.MaxHistoryRecords)
		we.Status.History = we.Status.History[:em.config.MaxHistoryRecords]
	}

	if err := em.client.Status().Update(ctx, we); err != nil {
		return nil, err
	}

	return we, nil
}

func (em *workloadEndpointManager) PatchIPAllocation(ctx context.Context, allocation *spiderpoolv1.PodIPAllocation, we *spiderpoolv1.SpiderEndpoint) (*spiderpoolv1.SpiderEndpoint, error) {
	if we == nil || we.Status.Current == nil {
		return nil, errors.New("patch a unmarked Endpoint")
	}

	if we.Status.Current.ContainerID != allocation.ContainerID {
		return nil, fmt.Errorf("patch a mismarked Endpoint with IP allocation: %v", *we.Status.Current)
	}

	var merged bool
	for i, d := range we.Status.Current.IPs {
		if d.NIC == allocation.IPs[0].NIC {
			mergeIPDetails(&we.Status.Current.IPs[i], &allocation.IPs[0])
			mergeIPDetails(&we.Status.History[0].IPs[i], &allocation.IPs[0])
			merged = true
			break
		}
	}

	if !merged {
		we.Status.Current.IPs = append(we.Status.Current.IPs, allocation.IPs...)
		we.Status.History[0].IPs = append(we.Status.History[0].IPs, allocation.IPs...)
	}

	if err := em.client.Status().Update(ctx, we); err != nil {
		return nil, err
	}

	return we, nil
}

func (em *workloadEndpointManager) ClearCurrentIPAllocation(ctx context.Context, containerID string, we *spiderpoolv1.SpiderEndpoint) error {
	if we == nil || we.Status.Current == nil {
		return nil
	}

	if we.Status.Current.ContainerID != containerID {
		return nil
	}

	we.Status.Current = nil
	if err := em.client.Status().Update(ctx, we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// RemoveFinalizer removes a specific finalizer field in finalizers string array.
func (em *workloadEndpointManager) RemoveFinalizer(ctx context.Context, namespace, podName string) error {
	for i := 0; i <= em.config.MaxConflictRetrys; i++ {
		we, err := em.GetEndpointByName(ctx, namespace, podName)
		if err != nil {
			return err
		}

		if !controllerutil.ContainsFinalizer(we, constant.SpiderFinalizer) {
			return nil
		}

		controllerutil.RemoveFinalizer(we, constant.SpiderFinalizer)
		if err := em.client.Update(ctx, we); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == em.config.MaxConflictRetrys {
				return fmt.Errorf("insufficient retries(<=%d) to remove finalizer '%s' from Endpoint %s", em.config.MaxConflictRetrys, constant.SpiderFinalizer, podName)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * em.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
}

// ListAllHistoricalIPs collect wep history IPs and classify them with each pool name.
func (em *workloadEndpointManager) ListAllHistoricalIPs(ctx context.Context, namespace, podName string) (map[string][]types.IPAndCID, error) {
	wep, err := em.GetEndpointByName(ctx, namespace, podName)
	if err != nil {
		return nil, err
	}

	recordHistoryIPs := func(historyIPs map[string][]types.IPAndCID, poolName, ipAndCIDR *string, podName, podNS, containerID string) {
		if poolName != nil {
			if ipAndCIDR == nil {
				logutils.Logger.Sugar().Errorf("WEP data broken, pod '%s/%s' containerID '%s' used ippool '%s' with no ip", podNS, podName, containerID, *poolName)
				return
			}

			ip, _, _ := strings.Cut(*ipAndCIDR, "/")

			ips, ok := historyIPs[*poolName]
			if !ok {
				ips = []types.IPAndCID{{IP: ip, ContainerID: containerID}}
			} else {
				ips = append(ips, types.IPAndCID{IP: ip, ContainerID: containerID})
			}
			historyIPs[*poolName] = ips
		}
	}

	wepHistoryIPs := make(map[string][]types.IPAndCID)

	// circle to traverse each allocation
	for _, PodIPAllocation := range wep.Status.History {
		// circle to traverse each NIC
		for _, ipAllocationDetail := range PodIPAllocation.IPs {
			// collect IPv4
			recordHistoryIPs(wepHistoryIPs, ipAllocationDetail.IPv4Pool, ipAllocationDetail.IPv4, wep.Name, wep.Namespace, PodIPAllocation.ContainerID)

			// collect IPv6
			recordHistoryIPs(wepHistoryIPs, ipAllocationDetail.IPv6Pool, ipAllocationDetail.IPv6, wep.Name, wep.Namespace, PodIPAllocation.ContainerID)
		}
	}

	return wepHistoryIPs, nil
}

// IsIPBelongWEPCurrent will check the given IP whether belong to the wep current IPs.
func (em *workloadEndpointManager) IsIPBelongWEPCurrent(ctx context.Context, namespace, podName, poolIP string) (bool, error) {
	wep, err := em.GetEndpointByName(ctx, namespace, podName)
	if err != nil {
		return false, err
	}

	isBelongWEPCurrent := false
	if wep.Status.Current != nil {
		// wep will record the IP address and CIDR and the given arg poolIP doesn't contain CIDR
		for _, wepCurrentAllocationDetail := range wep.Status.Current.IPs {
			var currentIPv4, currentIPv6 string

			if wepCurrentAllocationDetail.IPv4 != nil {
				currentIPv4AndCIDR := *wepCurrentAllocationDetail.IPv4
				currentIPv4, _, _ = strings.Cut(currentIPv4AndCIDR, "/")
			}

			if wepCurrentAllocationDetail.IPv6 != nil {
				currentIPv6AndCIDR := *wepCurrentAllocationDetail.IPv6
				currentIPv6, _, _ = strings.Cut(currentIPv6AndCIDR, "/")
			}

			// if the given poolIP is same with the current IP, just break
			if poolIP == currentIPv4 || poolIP == currentIPv6 {
				isBelongWEPCurrent = true
				break
			}
		}
	}

	return isBelongWEPCurrent, nil
}

// CheckCurrentContainerID will check whether the current containerID of SpiderEndpoint is same with the given args or not
func (em *workloadEndpointManager) CheckCurrentContainerID(ctx context.Context, namespace, podName, containerID string) (bool, error) {
	wep, err := em.GetEndpointByName(ctx, namespace, podName)
	if err != nil {
		return false, err
	}

	// data broken
	if len(wep.Status.History) == 0 {
		return false, fmt.Errorf("WEP '%s/%s' data broken, no current data, details: '%+v'", namespace, podName, wep)
	}

	if wep.Status.Current != nil && wep.Status.Current.ContainerID == containerID {
		return true, nil
	}

	return false, nil
}

func (r *workloadEndpointManager) Delete(ctx context.Context, sep *spiderpoolv1.SpiderEndpoint) error {
	err := r.client.Delete(ctx, sep)

	return client.IgnoreNotFound(err)
}

// UpdateCurrentStatus serves for StatefulSet pod re-create
func (em *workloadEndpointManager) UpdateCurrentStatus(ctx context.Context, containerID string, pod *corev1.Pod) error {
	for i := 0; i <= em.config.MaxConflictRetrys; i++ {
		sep, err := em.GetEndpointByName(ctx, pod.Namespace, pod.Name)
		if nil != err {
			return err
		}

		// refresh containerID and NodeName
		var hasChange bool
		if sep.Status.Current.ContainerID != containerID {
			sep.Status.Current.ContainerID = containerID

			if *sep.Status.Current.Node != pod.Spec.NodeName {
				*sep.Status.Current.Node = pod.Spec.NodeName
			}
			hasChange = true
		}

		if hasChange {
			currentPodIPAllocation := *sep.Status.Current
			sep.Status.History = append([]spiderpoolv1.PodIPAllocation{currentPodIPAllocation}, sep.Status.History...)

			err = em.client.Status().Update(ctx, sep)
			if nil != err {
				if !apierrors.IsConflict(err) {
					return err
				}

				if i == em.config.MaxConflictRetrys {
					return fmt.Errorf("insufficient retries(<=%d) to re-allocate StatefulSet SpiderEndpoint '%s/%s'", em.config.MaxConflictRetrys, pod.Namespace, pod.Name)
				}

				time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * em.config.ConflictRetryUnitTime)
				continue
			}
		}
		break
	}

	return nil
}
