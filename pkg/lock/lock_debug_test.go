// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

//go:build lockdebug

package lock_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

var _ = Describe("Debug lock", Label("unittest", "lock_test"), func() {
	var buffer *gbytes.Buffer
	var selfishTimeout time.Duration

	BeforeEach(func() {
		buffer = gbytes.NewBuffer()
		lock.OutputWriter = buffer
		selfishTimeout = time.Duration(lock.SelfishThresholdSec*1000) * time.Millisecond
	})

	Describe("Mutex", func() {
		var mutex *lock.Mutex

		BeforeEach(func() {
			mutex = &lock.Mutex{}
		})

		It("locks", func() {
			mutex.Lock()
			mutex.Unlock()
		})

		It("holds the lock timeout", func() {
			go func() {
				defer GinkgoRecover()

				mutex.Lock()
				time.Sleep(selfishTimeout)
				mutex.Unlock()
			}()

			Eventually(buffer).Should(gbytes.Say("goroutine"))
		})

		It("ignores the timeout when unlocking", func() {
			go func() {
				defer GinkgoRecover()

				mutex.Lock()
				time.Sleep(selfishTimeout)
				mutex.UnlockIgnoreTime()
			}()

			Consistently(buffer).Should(gbytes.Say(""))
		})
	})

	Describe("RWMutex", func() {
		var rwMutex *lock.RWMutex

		BeforeEach(func() {
			rwMutex = &lock.RWMutex{}
		})

		It("locks", func() {
			rwMutex.RLock()
			rwMutex.RUnlock()

			rwMutex.Lock()
			rwMutex.Unlock()
		})

		It("holds the lock timeout", func() {
			go func() {
				defer GinkgoRecover()

				rwMutex.Lock()
				time.Sleep(selfishTimeout)
				rwMutex.Unlock()
			}()

			Eventually(buffer).Should(gbytes.Say("goroutine"))
		})

		It("ignores the timeout when unlocking", func() {
			go func() {
				defer GinkgoRecover()

				rwMutex.Lock()
				time.Sleep(selfishTimeout)
				rwMutex.UnlockIgnoreTime()
			}()

			Consistently(buffer).Should(gbytes.Say(""))
		})
	})
})
