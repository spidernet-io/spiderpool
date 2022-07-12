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
	e2e "github.com/spidernet-io/e2eframework/framework"
)

func RestartNodeUntilReady(frame *e2e.Framework, nodeMap map[string]bool, timeOut time.Duration, ctx context.Context) (bool, error) {
	for node := range nodeMap {
		arg := fmt.Sprintf("docker restart %s", node)
		cmd := exec.Command("/bin/bash", "-c", arg)
		stdout, exitCode := ExecCommand(cmd, timeOut)
		GinkgoWriter.Printf("node: %v stdout: %v exitCode: %v\n", node, stdout, exitCode)
		if exitCode != 0 {
			return false, errors.New("exitCode is not 0")
		}
	}
	// check node until ready
	readyok, err := frame.WaitClusterNodeReady(ctx)
	return readyok, err
}
