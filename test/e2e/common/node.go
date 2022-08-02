// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2e "github.com/spidernet-io/e2eframework/framework"
)

func RestartNodeUntilClusterReady(frame *e2e.Framework, nodeMap map[string]bool, timeOut time.Duration, ctx context.Context) (bool, error) {
	// send command to reboot node
	for node := range nodeMap {
		arg := fmt.Sprintf("docker restart %s", node)
		cmd := exec.Command("/bin/bash", "-c", arg)
		stdout, exitCode := ExecCommand(cmd, timeOut)
		GinkgoWriter.Printf("node: %v stdout: %v exitCode: %v\n", node, stdout, exitCode)
		if exitCode != 0 {
			return false, errors.New("exitCode is not 0")
		}
	}
	// wait node until ready
	readyok, err := frame.WaitClusterNodeReady(ctx)
	Expect(err).ShouldNot(HaveOccurred())

	// wait for cluster all pod running
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	errall := waitAllPodRunning(frame, ctx)
	if errall != nil {
		return false, errall
	}
	return readyok, errall
}

func waitAllPodRunning(frame *e2e.Framework, ctx context.Context) error {
	for {
		select {
		default:
			podlistAll, errget := frame.GetPodList()
			if errget != nil {
				return errget
			}
			if frame.CheckPodListRunning(podlistAll) {
				return nil
			}

			time.Sleep(time.Second)
		case <-ctx.Done():
			return errors.New("time out to wait all pod running")
		}
	}
}
