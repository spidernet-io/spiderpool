// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type WorkloadEndpointManager interface {
	GetEndpointByName(ctx context.Context, namespace, podName string, cached bool) (*spiderpoolv2beta1.SpiderEndpoint, error)
	ListEndpoints(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderEndpointList, error)
	DeleteEndpoint(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint) error
	RemoveFinalizer(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint) error
	PatchIPAllocationResults(ctx context.Context, results []*types.AllocationResult, endpoint *spiderpoolv2beta1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController, isMultipleNicWithNoName bool) error
	ReallocateCurrentIPAllocation(ctx context.Context, uid, nodeName, nic string, endpoint *spiderpoolv2beta1.SpiderEndpoint, isMultipleNicWithNoName bool) error
	UpdateAllocationNICName(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint, nic string) (*spiderpoolv2beta1.PodIPAllocation, error)
	ReleaseEndpointIPs(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint, uid string) ([]spiderpoolv2beta1.IPAllocationDetail, error)
	ReleaseEndpointAndFinalizer(ctx context.Context, namespace, podName string, cached bool) error
}

type workloadEndpointManager struct {
	client    client.Client
	apiReader client.Reader

	enableStatefulSet      bool
	enableKubevirtStaticIP bool
}

func NewWorkloadEndpointManager(client client.Client, apiReader client.Reader, enableStatefulSet, enableKubevirtStaticIP bool) (WorkloadEndpointManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	return &workloadEndpointManager{
		client:                 client,
		apiReader:              apiReader,
		enableStatefulSet:      enableStatefulSet,
		enableKubevirtStaticIP: enableKubevirtStaticIP,
	}, nil
}

func (em *workloadEndpointManager) GetEndpointByName(ctx context.Context, namespace, podName string, cached bool) (*spiderpoolv2beta1.SpiderEndpoint, error) {
	reader := em.apiReader
	if cached == constant.UseCache {
		reader = em.client
	}

	var endpoint spiderpoolv2beta1.SpiderEndpoint
	if err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &endpoint); nil != err {
		return nil, err
	}

	return &endpoint, nil
}

func (em *workloadEndpointManager) ListEndpoints(ctx context.Context, cached bool, opts ...client.ListOption) (*spiderpoolv2beta1.SpiderEndpointList, error) {
	reader := em.apiReader
	if cached == constant.UseCache {
		reader = em.client
	}

	var endpointList spiderpoolv2beta1.SpiderEndpointList
	if err := reader.List(ctx, &endpointList, opts...); err != nil {
		return nil, err
	}

	return &endpointList, nil
}

func (em *workloadEndpointManager) DeleteEndpoint(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint) error {
	if err := em.client.Delete(ctx, endpoint); err != nil {
		return client.IgnoreNotFound(err)
	}

	return nil
}

func (em *workloadEndpointManager) RemoveFinalizer(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint %w", constant.ErrMissingRequiredParam)
	}

	if !controllerutil.ContainsFinalizer(endpoint, constant.SpiderFinalizer) {
		return nil
	}

	oldEndpoint := endpoint.DeepCopy()
	controllerutil.RemoveFinalizer(endpoint, constant.SpiderFinalizer)

	if err := em.client.Patch(ctx, endpoint, client.MergeFrom(oldEndpoint)); err != nil {
		return fmt.Errorf("failed to remove finalizer %s from Endpoint %s/%s: %w", constant.SpiderFinalizer, endpoint.Namespace, endpoint.Name, err)
	}

	return nil
}

func (em *workloadEndpointManager) PatchIPAllocationResults(ctx context.Context, results []*types.AllocationResult, endpoint *spiderpoolv2beta1.SpiderEndpoint, pod *corev1.Pod, podController types.PodTopController, isMultipleNicWithNoName bool) error {
	if pod == nil {
		return fmt.Errorf("pod %w", constant.ErrMissingRequiredParam)
	}

	logger := logutils.FromContext(ctx)

	if endpoint == nil {
		endpoint = &spiderpoolv2beta1.SpiderEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
			Status: spiderpoolv2beta1.WorkloadEndpointStatus{
				Current: spiderpoolv2beta1.PodIPAllocation{
					UID:  string(pod.UID),
					Node: pod.Spec.NodeName,
					IPs:  convert.ConvertResultsToIPDetails(results, isMultipleNicWithNoName),
				},
				OwnerControllerType: podController.Kind,
				OwnerControllerName: podController.Name,
			},
		}

		// Do not set ownerReference for Endpoint when its corresponding Pod is
		// controlled by StatefulSet/KubevirtVMI. Once the Pod of StatefulSet/KubevirtVMI is recreated,
		// we can immediately retrieve the old IP allocation results from the
		// Endpoint without worrying about the cascading deletion of the Endpoint.
		switch {
		case em.enableStatefulSet && podController.APIVersion == appsv1.SchemeGroupVersion.String() && podController.Kind == constant.KindStatefulSet:
			logger.Sugar().Infof("do not set OwnerReference for SpiderEndpoint '%s' since the pod top controller is %s", endpoint, podController.Kind)
		case em.enableKubevirtStaticIP && podController.APIVersion == kubevirtv1.SchemeGroupVersion.String() && podController.Kind == constant.KindKubevirtVMI:
			endpoint.Name = podController.Name
			logger.Sugar().Infof("do not set OwnerReference for SpiderEndpoint '%s' since the pod top controller is %s", endpoint, podController.Kind)
		default:
			if err := controllerutil.SetOwnerReference(pod, endpoint, em.client.Scheme()); err != nil {
				return err
			}
		}

		controllerutil.AddFinalizer(endpoint, constant.SpiderFinalizer)
		logger.Sugar().Infof("try to create SpiderEndpoint %s", endpoint)
		return em.client.Create(ctx, endpoint)
	}

	if endpoint.Status.Current.UID != string(pod.UID) {
		return nil
	}

	// TODO(iiiceoo): Only append records with different NIC.
	endpoint.Status.Current.IPs = append(endpoint.Status.Current.IPs, convert.ConvertResultsToIPDetails(results, isMultipleNicWithNoName)...)
	logger.Sugar().Infof("try to update SpiderEndpoint %s", endpoint)
	return em.client.Update(ctx, endpoint)
}

func (em *workloadEndpointManager) ReallocateCurrentIPAllocation(ctx context.Context, uid, nodeName, nic string, endpoint *spiderpoolv2beta1.SpiderEndpoint, isMultipleNicWithNoName bool) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint %w", constant.ErrMissingRequiredParam)
	}

	log := logutils.FromContext(ctx)
	deepCopy := endpoint.DeepCopy()
	deepCopy.Status.Current.UID = uid
	deepCopy.Status.Current.Node = nodeName

	if isMultipleNicWithNoName {
		for index := range deepCopy.Status.Current.IPs {
			if deepCopy.Status.Current.IPs[index].NIC != "" {
				if deepCopy.Status.Current.IPs[index].NIC == nic {
					log.Sugar().Debugf("no need to update Endpoint Current IPs %s due to same NIC %s", deepCopy.Status.Current.IPs[index].String(), nic)
					break
				}
			} else {
				// For the multiple NICs allocation with no NIC name specified,
				// we'll allocate all NICs IPs in the first allocation and record the details in the Endpoint resource.
				// This is NIC must be "eth0" and others with empty NIC name, we reset the real NIC name for the empties in the retrieve actions.
				log.Sugar().Debugf("update Endpoint Current IPs %s with NIC %s", deepCopy.Status.Current.IPs[index].String(), nic)
				deepCopy.Status.Current.IPs[index].NIC = nic
				break
			}
		}
	}

	if reflect.DeepEqual(deepCopy, endpoint) {
		return nil
	}

	deepCopy.DeepCopyInto(endpoint)
	log.Sugar().Infof("try to update Endpoint %s", endpoint)
	return em.client.Update(ctx, endpoint)
}

func (em *workloadEndpointManager) UpdateAllocationNICName(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint, nic string) (*spiderpoolv2beta1.PodIPAllocation, error) {
	for index := range endpoint.Status.Current.IPs {
		if endpoint.Status.Current.IPs[index].NIC != "" {
			if endpoint.Status.Current.IPs[index].NIC == nic {
				return &endpoint.Status.Current, nil
			}
		} else {
			// the SpiderEndpoint status allocation is already in order by NIC sequence
			endpoint.Status.Current.IPs[index].NIC = nic
			break
		}
	}

	err := em.client.Update(ctx, endpoint)
	if nil != err {
		return nil, err
	}

	return &endpoint.Status.Current, nil
}

// ReleaseEndpointIPs will release the SpiderEndpoint status recorded IPs.
func (em *workloadEndpointManager) ReleaseEndpointIPs(ctx context.Context, endpoint *spiderpoolv2beta1.SpiderEndpoint, podUID string) ([]spiderpoolv2beta1.IPAllocationDetail, error) {
	log := logutils.FromContext(ctx)

	if endpoint.Status.Current.UID != podUID {
		return nil, fmt.Errorf("the SpiderEndpoint recorded PodUID '%s' is unmacthed with the given PodUID '%s'", endpoint.Status.Current.UID, podUID)
	}

	recordedIPAllocationDetails := endpoint.Status.Current.IPs
	if len(recordedIPAllocationDetails) != 0 {
		endpoint.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{}
		log.Sugar().Debugf("try to clean up SpiderEndpoint recorded IPs: %s", endpoint)
		err := em.client.Update(ctx, endpoint)
		if nil != err {
			return nil, err
		}
	}

	return recordedIPAllocationDetails, nil
}

func (em *workloadEndpointManager) ReleaseEndpointAndFinalizer(ctx context.Context, namespace, podName string, cached bool) error {
	log := logutils.FromContext(ctx)

	endpoint, err := em.GetEndpointByName(ctx, namespace, podName, cached)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Sugar().Debugf("SpiderEndpoint '%s/%s' does not exist and may have been cleaned up", namespace, podName)
			return nil
		}
		return err
	}

	if endpoint.DeletionTimestamp == nil {
		err := em.DeleteEndpoint(ctx, endpoint)
		if err != nil {
			return err
		}
	}

	if err := em.RemoveFinalizer(ctx, endpoint); err != nil {
		return err
	} else {
		log.Sugar().Infof("remove SpiderEndpoint '%s/%s' finalizer successfully", namespace, podName)
	}

	return nil
}
