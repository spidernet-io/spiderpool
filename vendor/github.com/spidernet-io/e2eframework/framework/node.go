// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) GetNode(nodeName string) (*corev1.Node, error) {
	ctx := context.TODO()
	node := &corev1.Node{}
	err1 := f.KClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)
	if err1 != nil {
		return nil, err1
	}
	return node, nil
}

func (f *Framework) IsClusterNodeReady() (bool, error) {

	for _, nodeName := range f.Info.KindNodeList {
		nodelist, err2 := f.GetNode(nodeName)
		if err2 != nil {
			break
		}
		isnodeready := f.CheckNodeStatus(nodelist, true)
		if !isnodeready {
			return false, nil
		}
	}
	return true, nil
}

func (f *Framework) GetNodeList(opts ...client.ListOption) (*corev1.NodeList, error) {
	nodes := &corev1.NodeList{}
	e := f.ListResource(nodes, opts...)
	if e != nil {
		return nil, e
	}
	return nodes, nil
}

func (f *Framework) CheckNodeStatus(node *corev1.Node, expectReady bool) bool {

	unreachTaintTemp := &corev1.Taint{
		Key:    corev1.TaintNodeUnreachable,
		Effect: corev1.TaintEffectNoExecute,
	}
	notReadyTaintTemp := &corev1.Taint{
		Key:    corev1.TaintNodeNotReady,
		Effect: corev1.TaintEffectNoExecute,
	}
	for _, cond := range node.Status.Conditions {
		// check whether the ready host have taints
		if cond.Type == corev1.NodeReady {
			haveTaints := false
			tat := node.Spec.Taints
			for _, tat := range tat {
				if tat.MatchTaint(unreachTaintTemp) || tat.MatchTaint(notReadyTaintTemp) {
					haveTaints = true
					break
				}
			}
			if expectReady {
				if (cond.Status == corev1.ConditionTrue) && !haveTaints {
					return true
				}
				return false
			}
			if cond.Status != corev1.ConditionTrue {
				return true
			}
			f.Log("nodename: %s is %v Reason: %v, message: %v",
				node.Name, cond.Status == corev1.ConditionTrue, cond.Reason, cond.Message)
			return false
		}
	}
	f.Log("%v failed to find condition %v", node.Name, corev1.NodeReady)
	return false
}
