// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func (sm *subnetManager) Run(ctx context.Context, client kubernetes.Interface) {
	for {
		if !sm.leader.IsElected() {
			time.Sleep(sm.config.LeaderRetryElectGap)
			continue
		}

		logger.Info("Starting subnet manager controllers")
		kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, 0)
		stopper := make(chan struct{})

		go sm.StartDeploymentController(kubeInformerFactory.Apps().V1().Deployments().Informer(), stopper)
		//kubeInformerFactory.Apps().V1().ReplicaSets().Informer()
		//kubeInformerFactory.Apps().V1().DaemonSets().Informer()
		//kubeInformerFactory.Apps().V1().StatefulSets().Informer()
		//kubeInformerFactory.Batch().V1().Jobs().Informer()
		//kubeInformerFactory.Batch().V1().CronJobs().Informer()

		go func() {
			for {
				if !sm.leader.IsElected() {
					logger.Warn("leader lost! stop subnet controllers!")
					close(stopper)
					return
				}

				time.Sleep(sm.config.LeaderRetryElectGap)
			}
		}()

		<-stopper
		logger.Error("subnet manager controllers broken")
	}
}

func (sm *subnetManager) reconcile(ctx context.Context, podSubnetConfig *PodSubnetAnno, appKind string, app metav1.Object, podLabels map[string]string, appReplicas int) error {
	// retrieve application pools
	f := func(ctx context.Context, pool *spiderpoolv1.SpiderIPPool, subnetMgrName string, ipVersion types.IPVersion) error {
		var ipNum int
		if podSubnetConfig.flexibleIPNum != nil {
			ipNum = appReplicas + *podSubnetConfig.flexibleIPNum
		} else {
			ipNum = podSubnetConfig.assignIPNum
		}

		// verify whether the pool IPs need to be expanded or not
		if pool == nil {
			// create IPPool when the subnet manager was specified
			err := sm.AllocateIPPool(ctx, subnetMgrName, appKind, app, podLabels, ipNum, ipVersion)
			if nil != err {
				return err
			}
		} else {
			err := sm.CheckScaleIPPool(ctx, pool, subnetMgrName, ipNum)
			if nil != err {
				return err
			}
		}

		return nil
	}

	if len(podSubnetConfig.subnetManagerV4) != 0 {
		v4Pool, err := sm.RetrieveIPPool(ctx, appKind, app, podSubnetConfig.subnetManagerV4, constant.IPv4)
		if nil != err {
			return err
		}
		err = f(ctx, v4Pool, podSubnetConfig.subnetManagerV4, constant.IPv4)
		if nil != err {
			return err
		}
	}

	if len(podSubnetConfig.subnetManagerV6) != 0 {
		v6Pool, err := sm.RetrieveIPPool(ctx, appKind, app, podSubnetConfig.subnetManagerV6, constant.IPv6)
		if nil != err {
			return err
		}
		err = f(ctx, v6Pool, podSubnetConfig.subnetManagerV6, constant.IPv6)
		if nil != err {
			return err
		}
	}

	return nil
}

func hasSubnetConfigChanged(ctx context.Context, oldSubnetConfig, newSubnetConfig *PodSubnetAnno) bool {
	log := logutils.FromContext(ctx)

	switch {
	// do not use subnet manager
	case oldSubnetConfig == nil && newSubnetConfig == nil:
		log.Sugar().Debugf("onDeploymentUpdate: new application used standard IPAM mode")
		return false

	// the old use but the new one do not use subnet manager, the pod will recreate with default IPAM mode
	case oldSubnetConfig != nil && newSubnetConfig == nil:
		log.Sugar().Debugf("onDeploymentUpdate: new application discarded to use SpiderSubnet feature")
		return false

	// use subnet manager
	case oldSubnetConfig == nil && newSubnetConfig != nil:
		// reconcile
		return true

	// check whether subnet manager configuration changed
	case oldSubnetConfig != nil && newSubnetConfig != nil:
		if (oldSubnetConfig.subnetManagerV4 != "" && newSubnetConfig.subnetManagerV4 != "" && oldSubnetConfig.subnetManagerV4 != newSubnetConfig.subnetManagerV4) ||
			(oldSubnetConfig.subnetManagerV6 != "" && newSubnetConfig.subnetManagerV6 != "" && oldSubnetConfig.subnetManagerV6 != newSubnetConfig.subnetManagerV6) {
			log.Sugar().Errorf("onDeploymentUpdate: it's invalid to change SpiderSubnet")
			return false
		}

		if reflect.DeepEqual(oldSubnetConfig, newSubnetConfig) {
			log.Sugar().Debugf("onDeploymentUpdate: new application didn't change SpiderSubnet configuration")
			return false
		}

	default:
		return false
	}

	return false
}
