// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	e2e "github.com/spidernet-io/e2eframework/framework"
	corev1 "k8s.io/api/core/v1"
)

// Restart the node and wait for the cluster to be ready and the Pods in the cluster to "running".
// In a "Kind" cluster, it is not recommended to set `nodes` to `nil`.
// Restarting all nodes will cause the spiderpool component to fail to pull up, further rendering the cluster unavailable.
func RestartNodeUntilClusterReady(ctx context.Context, frame *e2e.Framework, nodes ...string) error {
	var err error
	var nodeList *corev1.NodeList

	nodeList, err = frame.GetNodeList()
	if err != nil {
		return err
	}
	if nodes == nil {
		for _, nodeObj := range nodeList.Items {
			nodes = append(nodes, nodeObj.Name)
		}
	}

	for _, v := range nodes {
		arg := fmt.Sprintf("docker restart %s", v)
		cmd := exec.Command("/bin/bash", "-c", arg)
		_, err := ExecCommand(ctx, cmd)
		if err != nil {
			return errors.New("failed to execute command")
		}
		GinkgoWriter.Printf("Successful execution of the command %v \n", arg)
	}

	// Waiting for nodes to be ready
	_, err = frame.WaitClusterNodeReady(ctx)
	if err != nil {
		GinkgoWriter.Printf("Nodes %v not ready in the expected time \n", nodes)
		return err
	}
	GinkgoWriter.Println("All nodes in the cluster are ready")

	err = frame.WaitAllPodUntilRunning(ctx)
	if err != nil {
		return err
	}
	GinkgoWriter.Println("Check that the status of all Pods in the cluster is running")
	return nil
}
