// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package limiter_test

import (
	"context"
	"math/rand"
	"strconv"
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

			maxWaitTime := 0 * time.Second
			config = limiter.LimiterConfig{
				MaxWaitTime: &maxWaitTime,
			}
		})

		It("forgets to start the limiter", func() {
			queue := limiter.NewLimiter(config)
			Expect(queue).NotTo(BeNil())

			ctx := context.TODO()
			reason, err := queue.AcquireTicket(ctx)
			Expect(err).To(MatchError(limiter.ErrUnexpectedBlocking))
			Expect(reason).To(Equal(limiter.UnexpectedBlocking))
			queue.ReleaseTicket(ctx)
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
		})

		Context("Use", func() {
			BeforeEach(func() {
				ctx, cancel = context.WithCancel(context.Background())
				DeferCleanup(cancel)

				maxQueueSize := 3
				maxWaitTime := 2 * time.Second
				config = limiter.LimiterConfig{
					MaxQueueSize: &maxQueueSize,
					MaxWaitTime:  &maxWaitTime,
				}
				queuers = 3
				workHours = 1 * time.Second
			})

			It("acquires tickets", func() {
				ctx := context.TODO()
				reason, err := queue.AcquireTicket(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(reason).To(Equal(limiter.Checkin))
				queue.ReleaseTicket(ctx)
			})

			It("acquires tickets but timeout", func() {
				ctx := context.TODO()
				reasonCh := make(chan limiter.Reason, queuers)
				wg := sync.WaitGroup{}
				wg.Add(queuers)
				for i := 0; i < queuers; i++ {
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						reason, err := queue.AcquireTicket(ctx)
						Expect(err).NotTo(HaveOccurred())
						reasonCh <- reason

						time.Sleep(workHours)
						queue.ReleaseTicket(ctx)
					}()
				}
				wg.Wait()
				// Eventually some queuers will wait timeout due to slow consumption.
				Eventually(reasonCh).Should(Receive(Equal(limiter.CheckinTimeout)))
			})

			It("acquires tickets but queue is full", func() {
				ctx := context.TODO()
				wg := sync.WaitGroup{}
				wg.Add(queuers + 2)
				for i := 0; i < queuers+2; i++ {
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						_, err := queue.AcquireTicket(ctx)
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
		})

		Context("Concurrency", func() {
			var ticketSize int
			var randomTicket func(int) []string

			BeforeEach(func() {
				ctx, cancel = context.WithCancel(context.Background())
				DeferCleanup(cancel)

				maxQueueSize := 200
				maxWaitTime := 5 * time.Second
				config = limiter.LimiterConfig{
					MaxQueueSize: &maxQueueSize,
					MaxWaitTime:  &maxWaitTime,
				}
				queuers = 200
				workHours = 50 * time.Millisecond

				ticketSize = 3
				randomTicket = func(ticketSize int) []string {
					var tickets []string
					rand.Seed(time.Now().UnixNano())
					n := rand.Intn(ticketSize) + 1
					for i := 0; i < n; i++ {
						tickets = append(tickets, "t"+strconv.Itoa(rand.Intn(ticketSize)+1))
					}

					return tickets
				}
			})

			It("collects the conflict rate when consumption is too slow", func() {
				ctx := context.TODO()
				reasonCh := make(chan limiter.Reason, queuers)
				wg := sync.WaitGroup{}
				wg.Add(queuers)
				for i := 0; i < queuers; i++ {
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						tickets := randomTicket(ticketSize)
						reason, err := queue.AcquireTicket(ctx, tickets...)
						Expect(err).NotTo(HaveOccurred())
						reasonCh <- reason

						time.Sleep(workHours)
						queue.ReleaseTicket(ctx, tickets...)
					}()
				}
				wg.Wait()
				close(reasonCh)

				var checkin, checkinTimeout int
				for r := range reasonCh {
					if r == limiter.Checkin {
						checkin++
					} else if r == limiter.CheckinTimeout {
						checkinTimeout++
					}
				}

				GinkgoWriter.Printf("%d queuers who take %v to work queue in a queue with a maximum waiting time %v\n", queuers, workHours, config.MaxWaitTime)
				GinkgoWriter.Printf("%d queuers completed their work without conflict\n", checkin)
				GinkgoWriter.Printf("%d queuers work concurrently due to waiting timeout\n", checkinTimeout)
			})
		})

		Context("Shutdown", func() {
			BeforeEach(func() {
				ctx, cancel = context.WithCancel(context.Background())
				DeferCleanup(cancel)

				maxQueueSize := 3
				maxWaitTime := 2 * time.Second
				config = limiter.LimiterConfig{
					MaxQueueSize: &maxQueueSize,
					MaxWaitTime:  &maxWaitTime,
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

						_, err := queue.AcquireTicket(ctx)
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

				_, err := queue.AcquireTicket(ctx)
				Expect(err).To(MatchError(limiter.ErrShutdownQueue))
			})
		})
	})
})
