// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"fmt"
	"os/exec"
)

// operate node container, like shutdown, ssh to login
func (f *Framework) ExecKubectl(command string, ctx context.Context) ([]byte, error) {
	args := fmt.Sprintf("kubectl --kubeconfig %s %s", f.Info.KubeConfigPath, command)
	return exec.CommandContext(ctx, "sh", "-c", args).CombinedOutput()
}
