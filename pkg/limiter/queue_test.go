// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package limiter_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/limiter"
)

var _ = Describe("Limiter", Label("queue_test"), func() {
	var ctx context.Context
	var cancel context.CancelFunc
	var config limiter.LimiterConfig
	var queue limiter.Limiter

	Describe("New", func() {
		It("sets default config", func() {
			queue := limiter.NewLimiter(limiter.LimiterConfig{})
			Expect(queue).NotTo(BeNil())
		})
	})

	Describe("Incorrect use", func() {
		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
			DeferCleanup(cancel)

			config = limiter.LimiterConfig{}
		})

		It("forgets to start the limiter", func() {
			queue := limiter.NewLimiter(config)
			Expect(queue).NotTo(BeNil())

			ctx := context.TODO()
			err := queue.AcquireTicket(ctx)
			Expect(err).To(MatchError(limiter.ErrShutdownQueue))
			queue.ReleaseTicket(ctx)
		})

		It("repeatedly starts the limiter", func() {
			queue := limiter.NewLimiter(config)
			Expect(queue).NotTo(BeNil())

			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()

			go func() {
				defer GinkgoRecover()
				err := queue.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(queue.Started).Should(BeTrue())

			err := queue.Start(ctx)
			Expect(err).To(MatchError(limiter.ErrStartLimiteRrepeatedly))
		})
	})

	Describe("General", func() {
		var queuers int
		var workHours time.Duration

		JustBeforeEach(func() {
			queue = limiter.NewLimiter(config)
			Expect(queue).NotTo(BeNil())

			go func() {
				defer GinkgoRecover()
				err := queue.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(queue.Started).Should(BeTrue())
		})

		Context("Use", func() {
			BeforeEach(func() {
				ctx, cancel = context.WithCancel(context.Background())
				DeferCleanup(cancel)

				maxQueueSize := 2
				config = limiter.LimiterConfig{
					MaxQueueSize: &maxQueueSize,
				}
				queuers = maxQueueSize
				workHours = 1 * time.Second
			})

			It("acquires tickets", func() {
				ctx := context.TODO()
				err := queue.AcquireTicket(ctx)
				Expect(err).NotTo(HaveOccurred())
				queue.ReleaseTicket(ctx)
			})

			It("acquires tickets but queue is full", func() {
				ctx := context.TODO()
				wg := sync.WaitGroup{}
				wg.Add(queuers + 2)
				for i := 0; i < queuers+2; i++ {
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						err := queue.AcquireTicket(ctx)
						if err != nil {
							Expect(err).To(MatchError(limiter.ErrFullQueue))
							return
						}

						time.Sleep(workHours)
						queue.ReleaseTicket(ctx)
					}()
				}
				wg.Wait()
			})

			PIt("acquires tickets but ctx timeout", func() {
				ctx, cancel := context.WithTimeout(context.TODO(), workHours)
				defer cancel()

				wg := sync.WaitGroup{}
				wg.Add(queuers)
				for i := 0; i < queuers; i++ {
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						err := queue.AcquireTicket(ctx)
						if err != nil {
							Expect(err).To(MatchError(ctx.Err()))
							return
						}

						time.Sleep(workHours)
						queue.ReleaseTicket(ctx)
					}()
				}
				wg.Wait()
			})
		})

		Context("Shutdown", func() {
			BeforeEach(func() {
				ctx, cancel = context.WithCancel(context.Background())
				DeferCleanup(cancel)

				maxQueueSize := 3
				config = limiter.LimiterConfig{
					MaxQueueSize: &maxQueueSize,
				}
				queuers = 3
				workHours = 1 * time.Second
			})

			It("shutdowns the empty queue", func() {
				cancel()
			})

			It("shutdowns the working queue", func() {
				ctx := context.TODO()
				wg := sync.WaitGroup{}
				wg.Add(queuers)
				for i := 0; i < queuers; i++ {
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						err := queue.AcquireTicket(ctx)
						Expect(err).NotTo(HaveOccurred())

						time.Sleep(workHours)
						queue.ReleaseTicket(ctx)
					}()
				}
				time.Sleep(1 * time.Second)
				cancel()
				wg.Wait()
			})

			It("acquires tickets but queue shutdown", func() {
				ctx := context.TODO()
				cancel()
				time.Sleep(1 * time.Second)

				err := queue.AcquireTicket(ctx)
				Expect(err).To(MatchError(limiter.ErrShutdownQueue))
			})
		})
	})
})
