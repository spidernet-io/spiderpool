// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-multierror"
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

func GetNodeNetworkInfo(ctx context.Context, frame *e2e.Framework, nodeList []string) error {
	var jobResult *multierror.Error
	for _, node := range nodeList {
		GinkgoWriter.Printf("=============== Check the network information of the node %v ============== \n", node)
		commands := []string{
			"ip a",
			"ip link show",
			"ip n",
			"ip -6 n",
			"ip rule",
			"ip -6 rule",
			"ip route",
			"ip route show table 100",
			"ip route show table 101",
			"ip route show table 500",
			"ip -6 route",
			"ip -6 route show table 100",
			"ip -6 route show table 101",
			"ip -6 route show table 500",
		}

		for _, command := range commands {
			GinkgoWriter.Printf("--------------- execute %v in node: %v ------------ \n", command, node)
			out, err := frame.DockerExecCommand(ctx, node, command)
			if err != nil {
				jobResult = multierror.Append(jobResult, fmt.Errorf("node %v: command '%v' failed with error: %w, output: %s", node, command, err, out))
			}
		}
	}

	return jobResult.ErrorOrNil()
}
