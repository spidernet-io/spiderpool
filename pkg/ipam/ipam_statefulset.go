// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// retrieveStsAllocatedIPs servers for re-allocate StatefulSet pod
func (i *ipam) retrieveStsAllocatedIPs(ctx context.Context, containerID string, pod *corev1.Pod, sep *spiderpoolv1.SpiderEndpoint) (*models.IpamAddResponse, error) {
	logger := logutils.FromContext(ctx)
	logger.Sugar().Debugf("try to retrieve StatefulSet allocated pod '%s/%s' IPs", pod.Namespace, pod.Name)

	if sep == nil {
		logger.Sugar().Debugf("no exist StatefulSet wep '%s/%s', do not retrieve anything for StatefulSet, try to allocate", pod.Namespace, pod.Name)
		return nil, nil
	}

	// there's no possible that a StatefulSet pod Endpoint's property 'Status/Current' is nil
	if sep.Status.Current == nil {
		return nil, fmt.Errorf("spiderpool endpoint '%s/%s' data broken, details: '%+v'", sep.Namespace, sep.Name, sep)
	}

	ipConfigs, routes := convertIPDetailsToIPConfigsAndAllRoutes(sep.Status.Current.IPs)
	for _, ipConfig := range ipConfigs {
		tmpIPConfig := ipConfig

		err := i.ipPoolManager.UpdateAllocatedIPs(ctx, containerID, pod, *tmpIPConfig)
		if nil != err {
			return nil, fmt.Errorf("failed to re-assign IPPool IP for StatefulSet pod, error: %v", err)
		}
	}

	addResp := models.IpamAddResponse{Ips: ipConfigs, Routes: routes}
	// refresh wep
	err := i.weManager.UpdateCurrentStatus(ctx, containerID, pod)
	if nil != err {
		return nil, err
	}

	logger.Sugar().Infof("Succeed to re-allocate StatefulSet pod '%s/%s', result: '%+v'", pod.Namespace, pod.Name, addResp.Ips)
	return &addResp, nil
}
