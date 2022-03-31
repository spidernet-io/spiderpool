// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of spidernet-io

//go:build !lockdebug
// +build !lockdebug

package lock_test

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"time"
)

var _ = Describe("LockFast", Label("unitest"), func() {

	// it is daemon , add more test here
	It("test lock", func() {
		l := &lock.Mutex{}
		l.Lock()
		time.Sleep(1 * time.Second)
		l.Unlock()
	})
	It("test RWMutex lock", func() {
		l := &lock.RWMutex{}
		l.RLock()
		time.Sleep(1 * time.Second)
		l.RUnlock()

		l.Lock()
		time.Sleep(1 * time.Second)
		l.Unlock()
	})
})
