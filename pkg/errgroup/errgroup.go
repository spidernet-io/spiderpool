// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errgroup provides synchronization, error propagation, and Context
// cancellation for groups of goroutines working on subtasks of a common task.

package errgroup

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
)

type token struct{}

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// A zero Group is valid, has no limit on the number of active goroutines,
// and does not cancel on error.
type Group struct {
	cancel func(error)

	wg sync.WaitGroup

	sem chan token

	errOnce sync.Once
	err     error
}

func (g *Group) done() {
	if g.sem != nil {
		<-g.sem
	}
	g.wg.Done()
}

// WithContext returns a new Group and an associated Context derived from ctx.
//
// The derived Context is canceled the first time a function passed to Go
// returns a non-nil error or the first time Wait returns, whichever occurs
// first.
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Group{cancel: cancel}, ctx
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}

// Go calls the given function in a new goroutine.
// It blocks until the new goroutine can be added without the number of
// active goroutines in the group exceeding the configured limit.
//
// The first call to return a non-nil error cancels the group's context, if the
// group was created by calling WithContext. The error will be returned by Wait.
//
// UPDATED: golang each OS thread can have a different
// network namespace, and Go's thread scheduling is highly
// variable, callers cannot guarantee any specific namespace
// is set unless operations that require that namespace are
// wrapped with Do().
// see https://github.com/golang/go/wiki/LockOSThread and
// https://www.weave.works/blog/linux-namespaces-golang-followup
// to more details.
func (g *Group) Go(srcNs, targetNs ns.NetNS, f func() error) {
	if g.sem != nil {
		g.sem <- token{}
	}

	g.wg.Add(1)
	go func() {
		defer g.done()
		runtime.LockOSThread()

		// switch to pod's netns
		if err := targetNs.Set(); err != nil {
			g.errOnce.Do(func() {
				g.err = fmt.Errorf("failed to switch to pod's netns: %v", err)
				if g.cancel != nil {
					g.cancel(g.err)
				}
			})
		}

		defer func() {
			err := srcNs.Set() // switch back
			if err == nil {
				// Unlock the current thread only when we successfully switched back
				// to the original namespace; otherwise leave the thread locked which
				// will force the runtime to scrap the current thread, that is maybe
				// not as optimal but at least always safe to do.
				runtime.UnlockOSThread()
			}
		}()

		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel(g.err)
				}
			})
		}
	}()
}
