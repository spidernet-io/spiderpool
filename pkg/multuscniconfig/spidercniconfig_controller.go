// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"context"
	"fmt"
	"reflect"
	"time"

	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/utils"
)

var spiderCNIConfigController controller.Controller

func SetupSpiderCNIConfigController(mgr ctrl.Manager, leader election.SpiderLeaseElector) error {
	if mgr == nil {
		return fmt.Errorf("controller-runtime manager %w", constant.ErrMissingRequiredParam)
	}

	r := &spiderCNIConfigReconciler{
		client:          mgr.GetClient(),
		scheme:          mgr.GetScheme(),
		leader:          leader,
		targetNamespace: utils.GetAgentNamespace(),
		logger:          logutils.Logger.Named("SpiderCNIConfig-Controller"),
	}

	var err error
	if spiderCNIConfigController == nil {
		spiderCNIConfigController, err = controller.New(constant.KindSpiderCNIConfig, mgr, controller.Options{Reconciler: r, SkipNameValidation: ptr.To(true)})
		if err != nil {
			return err
		}
	}

	if err := spiderCNIConfigController.Watch(
		source.Kind[*spiderpoolv2beta1.SpiderCNIConfig](
			mgr.GetCache(),
			&spiderpoolv2beta1.SpiderCNIConfig{},
			&handler.TypedEnqueueRequestForObject[*spiderpoolv2beta1.SpiderCNIConfig]{},
		),
	); err != nil {
		return err
	}

	return nil
}

type spiderCNIConfigReconciler struct {
	client          client.Client
	scheme          *runtime.Scheme
	leader          election.SpiderLeaseElector
	targetNamespace string
	logger          *zap.Logger
}

func (r *spiderCNIConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if r.leader != nil && !r.leader.IsElected() {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	log := r.logger.With(zap.String("SpiderCNIConfig", req.Name))

	cnicfg := &spiderpoolv2beta1.SpiderCNIConfig{}
	if err := r.client.Get(ctx, ktypes.NamespacedName{Name: req.Name}, cnicfg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	netAttachName := cnicfg.Name
	if cnicfg.Annotations != nil {
		if tmpName, ok := cnicfg.Annotations[constant.AnnoNetAttachConfName]; ok {
			netAttachName = tmpName
		}
	}

	anno := make(map[string]string)
	for k, v := range cnicfg.Annotations {
		anno[k] = v
	}

	isExist := true
	netAttachDef := &netv1.NetworkAttachmentDefinition{}
	err := r.client.Get(ctx, ktypes.NamespacedName{Namespace: r.targetNamespace, Name: netAttachName}, netAttachDef)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		isExist = false
	}

	newNetAttachDef, err := generateNetAttachDefWithSpec(netAttachName, r.targetNamespace, cnicfg.Spec, anno)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate net-attach-def, error: %w", err)
	}

	if err := controllerutil.SetControllerReference(cnicfg, newNetAttachDef, r.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set net-attach-def %s owner reference with SpiderCNIConfig %s, error: %w",
			newNetAttachDef.Name, cnicfg.Name, err)
	}

	if isExist {
		if netAttachDef.DeletionTimestamp != nil {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}

		isNeedUpdate := false
		if !reflect.DeepEqual(netAttachDef.Annotations, newNetAttachDef.Annotations) {
			log.Debug("SpiderCNIConfig annotation changed")
			netAttachDef.SetAnnotations(newNetAttachDef.Annotations)
			isNeedUpdate = true
		}

		if netAttachDef.Spec.Config != newNetAttachDef.Spec.Config {
			log.Debug("SpiderCNIConfig CNI configuration changed")
			netAttachDef.Spec.Config = newNetAttachDef.Spec.Config
			isNeedUpdate = true
		}

		if !metav1.IsControlledBy(netAttachDef, cnicfg) {
			log.Debug("net-attach-def ownerReference was removed, try to add it")
			netAttachDef.SetOwnerReferences(newNetAttachDef.GetOwnerReferences())
			isNeedUpdate = true
		}

		if isNeedUpdate {
			log.Info("try to update net-attach-def", zap.String("nad", fmt.Sprintf("%s/%s", netAttachDef.Namespace, netAttachDef.Name)))
			if err := r.client.Update(ctx, netAttachDef); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update net-attach-def %s/%s, error: %w", netAttachDef.Namespace, netAttachDef.Name, err)
			}
		}

		return ctrl.Result{}, nil
	}

	log.Info("try to create net-attach-def", zap.String("nad", fmt.Sprintf("%s/%s", newNetAttachDef.Namespace, newNetAttachDef.Name)))
	if err := r.client.Create(ctx, newNetAttachDef); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create net-attach-def %s/%s, error: %w", newNetAttachDef.Namespace, newNetAttachDef.Name, err)
	}

	return ctrl.Result{}, nil
}
