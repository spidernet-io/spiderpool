// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package kubevirtmanager

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

type KubevirtManager interface {
	IsValidVMPod(ctx context.Context, namespace, podControllerType, podControllerName string) (bool, error)
	GetVMByName(ctx context.Context, namespace, name string, cached bool) (*kubevirtv1.VirtualMachine, error)
	GetVMIByName(ctx context.Context, namespace, name string, cached bool) (*kubevirtv1.VirtualMachineInstance, error)
	GetVMIMByName(ctx context.Context, namespace, name string, cached bool) (*kubevirtv1.VirtualMachineInstanceMigration, error)
}

type kubevirtManager struct {
	client    client.Client
	apiReader client.Reader
}

func NewKubevirtManager(client client.Client, apiReader client.Reader) KubevirtManager {
	km := &kubevirtManager{
		client:    client,
		apiReader: apiReader,
	}

	return km
}

func (km *kubevirtManager) IsValidVMPod(ctx context.Context, namespace, podControllerType, podControllerName string) (bool, error) {
	if podControllerType != constant.KindKubevirtVMI {
		return false, fmt.Errorf("pod is controlled by '%s' instead of %s", podControllerType, constant.KindKubevirtVMI)
	}

	log := logutils.FromContext(ctx)
	vmi, err := km.GetVMIByName(ctx, namespace, podControllerName, false)
	if nil != err {
		// if the vmi was deleted, try to check its controller vm
		if apierrors.IsNotFound(err) {
			log.Sugar().Warnf("kubevirt vmi '%s/%s' is not exist, try to get vm", namespace, podControllerName)
		} else {
			return false, err
		}
	} else {
		if vmi.DeletionTimestamp != nil {
			// if the vmi is terminating and no owner controller vm, the pod is no longer valid.
			if !isVMIControlledByVM(vmi) {
				return false, nil
			}
		} else {
			// if the vmi is still alive, the pod is valid.
			return true, nil
		}
	}

	vm, err := km.GetVMByName(ctx, namespace, podControllerName, false)
	if nil != err {
		// is the vm is not exist, the pod is no longer valid
		if apierrors.IsNotFound(err) {
			log.Sugar().Warnf("kubevirt vm '%s/%s' is not exist", namespace, podControllerName)
			return false, nil
		}
		return false, err
	}

	// is the vm is terminating, the pod is no longer valid
	if vm.DeletionTimestamp != nil {
		log.Sugar().Debugf("kubevirt vm '%s/%s' is terminating", namespace, podControllerName)
		return false, nil
	}

	return true, nil
}

func (km *kubevirtManager) GetVMByName(ctx context.Context, namespace, name string, cached bool) (*kubevirtv1.VirtualMachine, error) {
	reader := km.apiReader
	if cached == constant.UseCache {
		reader = km.client
	}

	var vm kubevirtv1.VirtualMachine
	err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &vm)
	if nil != err {
		return nil, fmt.Errorf("failed to get kubevirt vm '%s/%s', error: %w", namespace, name, err)
	}

	return &vm, nil
}

func (km *kubevirtManager) GetVMIByName(ctx context.Context, namespace, name string, cached bool) (*kubevirtv1.VirtualMachineInstance, error) {
	reader := km.apiReader
	if cached == constant.UseCache {
		reader = km.client
	}

	var vmi kubevirtv1.VirtualMachineInstance
	err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &vmi)
	if nil != err {
		return nil, fmt.Errorf("failed to get kubevirt vmi '%s/%s', error: %w", namespace, name, err)
	}

	return &vmi, nil
}

func (km *kubevirtManager) GetVMIMByName(ctx context.Context, namespace, name string, cached bool) (*kubevirtv1.VirtualMachineInstanceMigration, error) {
	reader := km.apiReader
	if cached == constant.UseCache {
		reader = km.client
	}

	var vmim kubevirtv1.VirtualMachineInstanceMigration
	err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: name}, &vmim)
	if nil != err {
		return nil, fmt.Errorf("failed to get kubevirt vmim '%s/%s', error: %w", namespace, name, err)
	}

	return &vmim, nil
}
