// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of spidernet-io

//go:build !lockdebug
// +build !lockdebug

package lock_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

var _ = Describe("Fast lock", Label("unitest", "lock_test"), func() {
	Describe("Mutex", func() {
		var mutex *lock.Mutex

		BeforeEach(func() {
			mutex = &lock.Mutex{}
		})

		It("general use", func() {
			mutex.Lock()
			Expect(mutex.TryLock()).NotTo(BeTrue())

			mutex.Unlock()
			Expect(mutex.TryLock()).To(BeTrue())
			mutex.Unlock()

			mutex.Lock()
			mutex.UnlockIgnoreTime()
		})
	})

	Describe("RWMutex", func() {
		var rwMutex *lock.RWMutex

		BeforeEach(func() {
			rwMutex = &lock.RWMutex{}
		})

		It("general use", func() {
			rwMutex.Lock()
			Expect(rwMutex.TryRLock()).NotTo(BeTrue())
			Expect(rwMutex.TryLock()).NotTo(BeTrue())

			rwMutex.Unlock()
			Expect(rwMutex.TryLock()).To(BeTrue())
			Expect(rwMutex.TryRLock()).NotTo(BeTrue())
			rwMutex.Unlock()

			rwMutex.RLock()
			Expect(rwMutex.TryLock()).NotTo(BeTrue())
			Expect(rwMutex.TryRLock()).To(BeTrue())

			rwMutex.RUnlock()
			rwMutex.RUnlock()
			Expect(rwMutex.TryLock()).To(BeTrue())
			rwMutex.Unlock()

			rwMutex.Lock()
			rwMutex.UnlockIgnoreTime()
		})
	})
})
