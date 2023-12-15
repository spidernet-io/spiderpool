# errgroup

## Background

This file originates from "golang.org/x/sync/errgroup". The coordinator plugin uses errgroup to concurrently check the reachability of gateways and whether IP addresses conflict.
However, when launching goroutines in [netns.Do]("github.com/containernetworking/plugins/pkg/ns"), the Go runtime cannot guarantee that the code will be executed in the specified 
network namespace. Therefore, we modified the `Go()` method of errgroup: manually switch to the target network namespace when launching a goroutine and return to the original network 
namespace after execution.

Please see `errgroup.go` to find more details.
