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

func (f *Framework) ExecCommandInPod(podName, nameSpace, command string, ctx context.Context) ([]byte, error) {
	command = fmt.Sprintf("exec %s -n %s -- %s", podName, nameSpace, command)
	return f.ExecKubectl(command, ctx)
}

// DockerExecCommand is eq to `docker exec $containerId $command `
func (f *Framework) DockerExecCommand(ctx context.Context, containerId string, command string) ([]byte, error) {
	fullCommand := fmt.Sprintf("docker exec -i %s %s ", containerId, command)
	f.t.Logf(fullCommand)
	return exec.CommandContext(ctx, "/bin/sh", "-c", fullCommand).CombinedOutput()
}

// DockerRunCommand eq to `docker run $command`
func (f *Framework) DockerRunCommand(ctx context.Context, command string) ([]byte, error) {
	fullCommand := fmt.Sprintf("docker run %s ", command)
	f.t.Logf(fullCommand)
	return exec.CommandContext(ctx, "/bin/sh", "-c", fullCommand).CombinedOutput()
}

// DockerRMCommand is eq to `docker rm $containerId`
func (f *Framework) DockerRMCommand(ctx context.Context, containerId string) ([]byte, error) {
	fullCommand := fmt.Sprintf("docker rm -f %s", containerId)
	f.t.Logf(fullCommand)
	return exec.CommandContext(ctx, "/bin/bash", "-c", fullCommand).CombinedOutput()
}
