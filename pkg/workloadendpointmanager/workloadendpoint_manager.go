// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"errors"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

type WorkloadEndpointManager interface {
	RetriveIPAllocation(ctx context.Context, namespace, podName, containerID, nic string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error)
	UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error
	ClearCurrentIPAllocation(ctx context.Context, namespace, podName, containerID string) error
	Get(ctx context.Context, namespace, podName string) (*spiderpoolv1.WorkloadEndpoint, error)
	RemoveFinalizer(ctx context.Context, namespace, podName string) error
	ListAllHistoricalIPs(ctx context.Context, namespace, podName string) (map[string][]ippoolmanager.IPAndCID, error)
	IsIPBelongWEPCurrent(ctx context.Context, namespace, podName, poolIP string) (bool, error)
}

type workloadEndpointManager struct {
	client            client.Client
	runtimeMgr        ctrl.Manager
	maxHistoryRecords int
}

func NewWorkloadEndpointManager(c client.Client, mgr ctrl.Manager, maxHistoryRecords int) (WorkloadEndpointManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}
	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}

	return &workloadEndpointManager{
		client:            c,
		runtimeMgr:        mgr,
		maxHistoryRecords: maxHistoryRecords,
	}, nil
}

func (r *workloadEndpointManager) RetriveIPAllocation(ctx context.Context, namespace, podName, containerID, nic string, includeHistory bool) (*spiderpoolv1.PodIPAllocation, bool, error) {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if we.Status.Current != nil && we.Status.Current.ContainerID == containerID {
		for _, d := range we.Status.Current.IPs {
			if d.NIC == nic {
				return we.Status.Current, true, nil
			}
		}
	}

	if includeHistory {
		for _, a := range we.Status.History[1:] {
			if a.ContainerID == containerID {
				for _, d := range a.IPs {
					if d.NIC == nic {
						return &a, false, nil
					}
				}
			}
		}
	}

	return nil, false, nil
}

func (r *workloadEndpointManager) UpdateIPAllocation(ctx context.Context, namespace, podName string, allocation *spiderpoolv1.PodIPAllocation) error {
	logger := logutils.FromContext(ctx)

	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		var pod corev1.Pod
		if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
			return err
		}

		newWE := &spiderpoolv1.WorkloadEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
		}

		controllerutil.AddFinalizer(newWE, constant.SpiderWorkloadEndpointFinalizer)
		if err := controllerutil.SetOwnerReference(&pod, newWE, r.runtimeMgr.GetScheme()); err != nil {
			return nil
		}

		if err := r.client.Create(ctx, newWE); err != nil {
			return err
		}

		newWE.Status.Current = allocation
		newWE.Status.History = []spiderpoolv1.PodIPAllocation{*allocation}
		if err := r.client.Status().Update(ctx, newWE); err != nil {
			return err
		}
	} else {
		if we.Status.Current != nil && we.Status.Current.ContainerID == allocation.ContainerID {
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
		} else {
			we.Status.Current = allocation
			we.Status.History = append([]spiderpoolv1.PodIPAllocation{*allocation}, we.Status.History...)
			if len(we.Status.History) > r.maxHistoryRecords {
				logger.Sugar().Warnf("threshold of historical IP allocation records(<=%d) exceeded", r.maxHistoryRecords)
				we.Status.History = we.Status.History[:r.maxHistoryRecords]
			}
		}

		if err := r.client.Status().Update(ctx, &we); err != nil {
			return err
		}
	}

	return nil
}

// TODO(iiiceoo): refactor
func mergeIPDetails(target, delta *spiderpoolv1.IPAllocationDetail) {
	if target.IPv4 == nil {
		target.IPv4 = delta.IPv4
	}

	if target.IPv4Pool == nil {
		target.IPv4Pool = delta.IPv4Pool
	}

	if target.IPv4Gateway == nil {
		target.IPv4Gateway = delta.IPv4Gateway
	}

	if target.IPv6 == nil {
		target.IPv6 = delta.IPv6
	}

	if target.IPv6Pool == nil {
		target.IPv6Pool = delta.IPv6Pool
	}

	if target.IPv6Gateway == nil {
		target.IPv6Gateway = delta.IPv6Gateway
	}
}

func (r *workloadEndpointManager) ClearCurrentIPAllocation(ctx context.Context, namespace, podName, containerID string) error {
	var we spiderpoolv1.WorkloadEndpoint
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if we.Status.Current == nil {
		return nil
	}

	if we.Status.Current.ContainerID != containerID {
		return nil
	}

	we.Status.Current = nil
	if err := r.client.Status().Update(ctx, &we); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (r *workloadEndpointManager) Get(ctx context.Context, namespace, podName string) (*spiderpoolv1.WorkloadEndpoint, error) {
	wep := &spiderpoolv1.WorkloadEndpoint{}
	err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, wep)
	if nil != err {
		return nil, err
	}

	return wep, nil
}

// RemoveFinalizer removes a specific finalizer field in finalizers string array.
func (r *workloadEndpointManager) RemoveFinalizer(ctx context.Context, namespace, podName string) error {
	logger := logutils.FromContext(ctx)

	wep := &spiderpoolv1.WorkloadEndpoint{}
	err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, wep)
	if nil != err {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Debugf("wep '%s/%s' not found", namespace, podName)
			return nil
		}

		return err
	}

	// remove wep finalizer
	controllerutil.RemoveFinalizer(wep, constant.SpiderWorkloadEndpointFinalizer)

	err = r.client.Update(ctx, wep)
	if nil != err {
		return err
	}

	return nil
}

// ListAllHistoricalIPs collect wep history IPs and classify them with each pool name.
func (r *workloadEndpointManager) ListAllHistoricalIPs(ctx context.Context, namespace, podName string) (map[string][]ippoolmanager.IPAndCID, error) {
	wep, err := r.Get(ctx, namespace, podName)
	if nil != err {
		return nil, err
	}

	recordHistoryIPs := func(historyIPs map[string][]ippoolmanager.IPAndCID, poolName, ipAndCIDR *string, podName, podNS, containerID string) {
		if poolName != nil {
			if ipAndCIDR == nil {
				logutils.Logger.Sugar().Errorf("WEP data broken, pod '%s/%s' containerID '%s' used ippool '%s' with no ip")
				return
			}

			ip, _, _ := strings.Cut(*ipAndCIDR, "/")

			ips, ok := historyIPs[*poolName]
			if !ok {
				ips = []ippoolmanager.IPAndCID{{IP: ip, ContainerID: containerID}}
			} else {
				ips = append(ips, ippoolmanager.IPAndCID{IP: ip, ContainerID: containerID})
			}
			historyIPs[*poolName] = ips
		}
	}

	wepHistoryIPs := make(map[string][]ippoolmanager.IPAndCID)

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
func (r *workloadEndpointManager) IsIPBelongWEPCurrent(ctx context.Context, namespace, podName, poolIP string) (bool, error) {
	wep, err := r.Get(ctx, namespace, podName)
	if nil != err {
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
