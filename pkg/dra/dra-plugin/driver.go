// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package draPlugin

import (
	"context"
	"fmt"

	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	drapbv1 "k8s.io/kubelet/pkg/apis/dra/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
)

type driver struct {
	lock.RWMutex
	logger          *zap.Logger
	State           *NodeDeviceState
	K8sClientSet    kubernetes.Interface
	SpiderClientSet clientset.Interface
}

func NewDriver(logger *zap.Logger, cdiRoot string, so string) (*driver, error) {
	restConfig := ctrl.GetConfigOrDie()
	state, err := NewDeviceState(logger, cdiRoot, so)
	if err != nil {
		return nil, err
	}

	return &driver{
		logger:          logger,
		State:           state,
		K8sClientSet:    kubernetes.NewForConfigOrDie(restConfig),
		SpiderClientSet: clientset.NewForConfigOrDie(restConfig),
	}, nil
}

func (d *driver) NodePrepareResources(ctx context.Context, req *drapbv1.NodePrepareResourcesRequest) (*drapbv1.NodePrepareResourcesResponse, error) {
	d.logger.Info("NodePrepareResource is called")
	preparedResources := &drapbv1.NodePrepareResourcesResponse{Claims: map[string]*drapbv1.NodePrepareResourceResponse{}}
	for _, claim := range req.Claims {
		preparedResources.Claims[claim.Uid] = d.nodePrepareResource(ctx, claim)
	}

	d.logger.Info("NodePrepareResource returning newly prepared devices", zap.Any("response", preparedResources))
	return preparedResources, nil
}

func (d *driver) nodePrepareResource(ctx context.Context, claim *drapbv1.Claim) *drapbv1.NodePrepareResourceResponse {
	d.Lock()
	defer d.Unlock()

	isPrepared, devices, err := d.isPrepared(ctx, claim.Uid)
	if err != nil {
		return &drapbv1.NodePrepareResourceResponse{
			Error: fmt.Sprintf("error checking if claim is already prepared: %v", err),
		}
	}

	if isPrepared {
		d.logger.Info("[NodePrepareResource] Claim has already prepared, returning cached device resources", zap.String("claim", claim.Uid))
		return &drapbv1.NodePrepareResourceResponse{CDIDevices: devices}
	}

	d.logger.Info("[NodePrepareResource] Preparing devices for claim", zap.String("claim", claim.Uid))
	devices, err = d.prepare(ctx, claim)
	if err != nil {
		d.logger.Error("error preparing devices for claim", zap.String("cliam", claim.Uid), zap.Error(err))
		return &drapbv1.NodePrepareResourceResponse{
			Error: fmt.Sprintf("error preparing devices for claim %v: %v", claim.Uid, err),
		}
	}

	return &drapbv1.NodePrepareResourceResponse{CDIDevices: devices}
}

func (d *driver) prepare(ctx context.Context, claim *drapbv1.Claim) ([]string, error) {
	var err error
	var prepared []string
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		resourceClaim, err := d.K8sClientSet.ResourceV1alpha2().ResourceClaims(claim.Namespace).
			Get(ctx, claim.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// TODO(@cyclinder): check if the claim.ParametersRef is SpiderClaimParameters.
		scp, err := d.SpiderClientSet.SpiderpoolV2beta1().SpiderClaimParameters(claim.Namespace).
			Get(ctx, resourceClaim.Spec.ParametersRef.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		prepared, err = d.State.Prepare(ctx, claim.Uid, scp)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return prepared, nil
}

func (d *driver) isPrepared(ctx context.Context, claimUID string) (bool, []string, error) {
	// TODO(@cyclinder): should be check if the claim is prepared.
	return false, nil, nil
}

func (d *driver) NodeUnprepareResources(ctx context.Context, req *drapbv1.NodeUnprepareResourcesRequest) (*drapbv1.NodeUnprepareResourcesResponse, error) {
	d.logger.Info("NodeUnprepareResources is called")
	response := make(map[string]*drapbv1.NodeUnprepareResourceResponse, len(req.Claims))
	for _, claim := range req.Claims {
		response[claim.Uid] = d.unPrepareResoruce(ctx, claim)

	}
	return &drapbv1.NodeUnprepareResourcesResponse{Claims: response}, nil
}

func (d *driver) unPrepareResoruce(ctx context.Context, claim *drapbv1.Claim) *drapbv1.NodeUnprepareResourceResponse {
	d.logger.Info("UnPrepareResource for claim", zap.String("claim", claim.Uid))
	if err := d.State.UnPrepare(ctx, claim.Uid); err != nil {
		d.logger.Error("error unprepare resource for claim", zap.String("claim", claim.Uid), zap.Error(err))
		return &drapbv1.NodeUnprepareResourceResponse{Error: err.Error()}
	}

	return &drapbv1.NodeUnprepareResourceResponse{}
}
