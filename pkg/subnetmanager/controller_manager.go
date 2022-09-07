// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (sm *subnetMgr) InitControllers(ctx context.Context, client kubernetes.Interface) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
	go sm.StartDeploymentController(kubeInformerFactory.Apps().V1().Deployments().Informer())

	//kubeInformerFactory.Apps().V1().ReplicaSets().Informer()
	//kubeInformerFactory.Apps().V1().DaemonSets().Informer()

	//kubeInformerFactory.Apps().V1().StatefulSets().Informer()
	//kubeInformerFactory.Batch().V1().Jobs().Informer()
	//kubeInformerFactory.Batch().V1().CronJobs().Informer()
}

func (sm *subnetMgr) reconcile(ctx context.Context, podSubnetConfig PodSubnetAnno, appKind string, app metav1.Object) error {
	// retrieve application pools
	v4Pool, v6Pool, err := sm.RetrieveIPPools(ctx, appKind, app)
	if nil != err {
		return err
	}

	f := func(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetMgrName string, ipVersion types.IPVersion) error {
		// verify whether the pool IPs need to be expanded or not
		if pool != nil {
			err := sm.IPPoolExpansion(ctx, pool, subnetMgrName, podSubnetConfig.assignIPNum)
			if nil != err {
				return err
			}
		}

		// create IPPool when the subnet manager was specified
		if pool == nil && len(subnetMgrName) != 0 {
			err := sm.AllocateIPPool(ctx, subnetMgrName, appKind, app, podSubnetConfig.assignIPNum, ipVersion, podSubnetConfig.reclaimIPPool)
			if nil != err {
				return err
			}
		}

		return nil
	}

	err = f(ctx, v4Pool, podSubnetConfig.subnetManagerV4, constant.IPv4)
	if nil != err {
		return err
	}

	err = f(ctx, v6Pool, podSubnetConfig.subnetManagerV6, constant.IPv6)
	if nil != err {
		return err
	}

	return nil
}
