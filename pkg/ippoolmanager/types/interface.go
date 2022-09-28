// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	subnetmanagertypes "github.com/spidernet-io/spiderpool/pkg/subnetmanager/types"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type IPPoolManager interface {
	SetupWebhook() error
	SetupInformer(client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error
	InjectSubnetManager(subnetManager subnetmanagertypes.SubnetManager)
	GetIPPoolByName(ctx context.Context, poolName string) (*spiderpoolv1.SpiderIPPool, error)
	ListIPPools(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderIPPoolList, error)
	AllocateIP(ctx context.Context, poolName, containerID, nic string, pod *corev1.Pod) (*models.IPConfig, *spiderpoolv1.SpiderIPPool, error)
	ReleaseIP(ctx context.Context, poolName string, ipAndCIDs []types.IPAndCID) error
	CheckVlanSame(ctx context.Context, poolNameList []string) (map[types.Vlan][]string, bool, error)
	RemoveFinalizer(ctx context.Context, poolName string) error
	UpdateAllocatedIPs(ctx context.Context, containerID string, pod *corev1.Pod, oldIPConfig models.IPConfig) error
	CreateIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error
	ScaleIPPoolIPs(ctx context.Context, poolName string, expandIPs []string) error
	DeleteIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool) error
}
