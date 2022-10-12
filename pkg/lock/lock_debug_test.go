// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

//go:build lockdebug
// +build lockdebug

package lock_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

var _ = Describe("Debug lock", Label("unitest", "lock_test"), func() {
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

		It("general use", func() {
			mutex.Lock()
			mutex.Unlock()
		})

		It("took lock timeout", func() {
			go func() {
				mutex.Lock()
				time.Sleep(selfishTimeout)
				mutex.Unlock()
			}()

			Eventually(buffer).Should(gbytes.Say("goroutine"))
		})

		It("ignore timeout when unlocking", func() {
			go func() {
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

		It("general use", func() {
			rwMutex.RLock()
			rwMutex.RUnlock()

			rwMutex.Lock()
			rwMutex.Unlock()
		})

		It("took lock timeout", func() {
			go func() {
				rwMutex.Lock()
				time.Sleep(selfishTimeout)
				rwMutex.Unlock()
			}()

			Eventually(buffer).Should(gbytes.Say("goroutine"))
		})

		It("ignore timeout when unlocking", func() {
			go func() {
				rwMutex.Lock()
				time.Sleep(selfishTimeout)
				rwMutex.UnlockIgnoreTime()
			}()

			Consistently(buffer).Should(gbytes.Say(""))
		})
	})
})
