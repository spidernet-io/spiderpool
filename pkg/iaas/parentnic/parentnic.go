// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package parentnic publishes the parent NIC of IaaS-managed node-level
// (prewarm) SpiderIPPools to their status.parentNic field: the
// spiderpool-agent running on the pool's node copies the NIC name from the
// pool annotation ipam.spidernet.io/parent-nic and resolves its MAC address
// locally via netlink, so that the external IaaS network provider can locate
// the cloud-side parent port without any out-of-band node inventory.
package parentnic

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

// macResolver resolves the MAC address of a local NIC by name. It is an
// indirection for unit tests; production uses ResolveLocalNicMac.
type macResolver func(nicName string) (string, error)

// ResolveLocalNicMac reads the MAC address of the named NIC in the host
// network namespace via netlink.
func ResolveLocalNicMac(nicName string) (string, error) {
	link, err := netlink.LinkByName(nicName)
	if err != nil {
		return "", fmt.Errorf("failed to get link %s: %w", nicName, err)
	}
	mac := link.Attrs().HardwareAddr.String()
	if mac == "" {
		return "", fmt.Errorf("link %s has no MAC address", nicName)
	}
	return mac, nil
}

// StatusWriter reconciles SpiderIPPools and writes status.parentNic on the
// IaaS-managed node-level pools pinned to the local node.
type StatusWriter struct {
	client     ctrlclient.Client
	nodeName   string
	resolveMac macResolver
	logger     *zap.Logger
}

// Setup registers the StatusWriter controller with the agent's
// controller-runtime manager. It must be called before the manager starts.
func Setup(mgr ctrl.Manager, nodeName string, logger *zap.Logger) error {
	w := &StatusWriter{
		client:     mgr.GetClient(),
		nodeName:   nodeName,
		resolveMac: ResolveLocalNicMac,
		logger:     logger.Named("parentnic-status-writer"),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderIPPool{}).
		Complete(w)
}

// eligible reports whether the pool's status.parentNic is owned by the agent
// on nodeName: an IaaS-managed pool pinned to exactly this node, excluding
// the sibling v6 pool of a paired dual-stack set (by convention only the
// primary pool carries parentNic, mirroring status.ipMetaData).
func eligible(pool *spiderpoolv2beta1.SpiderIPPool, nodeName string) bool {
	if pool == nil {
		return false
	}
	if _, ok := pool.Labels[constant.LabelIPPoolIaasProvider]; !ok {
		return false
	}
	if len(pool.Spec.NodeName) != 1 || pool.Spec.NodeName[0] != nodeName {
		return false
	}
	if pool.Spec.IPVersion != nil && *pool.Spec.IPVersion == constant.IPv6 &&
		pool.Annotations[constant.AnnoIPPoolPairPool] != "" {
		// Sibling v6 pool of a paired set: parentNic lives on the primary.
		return false
	}
	return true
}

// Reconcile resolves the parent NIC MAC of one eligible pool and patches
// status.parentNic. A resolution failure (e.g. the NIC named by the
// annotation does not exist on this node) is returned as an error so that
// controller-runtime retries with backoff; the provider waits for the MAC
// and prewarming fails closed in the meantime.
func (w *StatusWriter) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pool := &spiderpoolv2beta1.SpiderIPPool{}
	if err := w.client.Get(ctx, req.NamespacedName, pool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !eligible(pool, w.nodeName) {
		return ctrl.Result{}, nil
	}

	nicName := pool.Annotations[constant.AnnoIPPoolParentNic]
	if nicName == "" {
		// The validating webhook requires the annotation on node-scoped
		// IaaS pools; a legacy pool without it is skipped.
		w.logger.Sugar().Warnf("IaaS node-level pool %s has no %s annotation, skip publishing status.parentNic",
			pool.Name, constant.AnnoIPPoolParentNic)
		return ctrl.Result{}, nil
	}

	mac, err := w.resolveMac(nicName)
	if err != nil {
		w.logger.Sugar().Errorf("failed to resolve MAC of parent NIC %s for pool %s: %v", nicName, pool.Name, err)
		return ctrl.Result{}, fmt.Errorf("failed to resolve MAC of parent NIC %s for pool %s: %w", nicName, pool.Name, err)
	}

	if pool.Status.ParentNic != nil && pool.Status.ParentNic.Name == nicName && pool.Status.ParentNic.MAC == mac {
		return ctrl.Result{}, nil
	}

	// Merge-patch only status.parentNic: status.ipMetaData is provider-owned
	// and the allocation counters are written elsewhere, so the patch must
	// not stomp them.
	orig := pool.DeepCopy()
	pool.Status.ParentNic = &spiderpoolv2beta1.ParentNicStatus{
		Name: nicName,
		MAC:  mac,
	}
	if err := w.client.Status().Patch(ctx, pool, ctrlclient.MergeFrom(orig)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch status.parentNic of pool %s: %w", pool.Name, err)
	}

	w.logger.Sugar().Infof("Published status.parentNic of pool %s: name=%s mac=%s", pool.Name, nicName, mac)
	return ctrl.Result{}, nil
}
