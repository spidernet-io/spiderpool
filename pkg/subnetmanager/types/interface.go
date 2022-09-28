// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type SubnetManager interface {
	SetupWebhook() error
	SetupInformer(ctx context.Context, client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error
	GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error)
	UpdateSubnetStatusOnce(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error
	Run(ctx context.Context, client kubernetes.Interface)
	GenerateIPsFromSubnet(ctx context.Context, subnetMgrName string, ipNum int) ([]string, error)
	AllocateIPPool(ctx context.Context, subnetMgrName string, appKind string, app metav1.Object, podLabels map[string]string, ipNum int, ipVersion types.IPVersion) error
	RetrieveIPPool(ctx context.Context, appKind string, app metav1.Object, subnetMgrName string, ipVersion types.IPVersion) (pool *spiderpoolv1.SpiderIPPool, err error)
	CheckScaleIPPool(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetManagerName string, ipNum int) error
}
