// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
)

type SubnetManager interface {
	SetupWebhook() error
	SetupInformer(ctx context.Context, client crdclientset.Interface, controllerLeader election.SpiderLeaseElector) error
	GetSubnetByName(ctx context.Context, subnetName string) (*spiderpoolv1.SpiderSubnet, error)
	ListSubnets(ctx context.Context, opts ...client.ListOption) (*spiderpoolv1.SpiderSubnetList, error)
	UpdateSubnetStatusOnce(ctx context.Context, subnet *spiderpoolv1.SpiderSubnet) error
}
