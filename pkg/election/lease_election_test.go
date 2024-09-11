// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package election

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/leaderelection"
)

var _ = Describe("Leader Election", Label("unittest", "election_test"), func() {
	type newLeaseElectorParams struct {
		leaseLockNS         string
		leaseLockName       string
		leaseLockIdentity   string
		leaseDuration       *time.Duration
		leaseRenewDeadline  *time.Duration
		leaseRetryPeriod    *time.Duration
		leaderRetryElectGap *time.Duration
	}

	var globalParams *newLeaseElectorParams
	var leaseDuration = 15 * time.Second
	var renewDeadline = 2 * time.Second
	var retryPeriod = 1 * time.Second
	var retryElectGap = 5 * time.Second

	BeforeEach(func() {
		globalParams = new(newLeaseElectorParams)
		globalParams.leaseLockNS = "foo"
		globalParams.leaseLockName = "bar"
		globalParams.leaseLockIdentity = "baz"
		globalParams.leaseDuration = &leaseDuration
		globalParams.leaseRenewDeadline = &renewDeadline
		globalParams.leaseRetryPeriod = &retryPeriod
		globalParams.leaderRetryElectGap = &retryElectGap
	})

	DescribeTable("check new lease elector", func(leaseParamsFunc func() *newLeaseElectorParams, shouldError bool) {
		defer GinkgoRecover()

		leaseParams := leaseParamsFunc()

		_, err := NewLeaseElector(
			leaseParams.leaseLockNS,
			leaseParams.leaseLockName,
			leaseParams.leaseLockIdentity,
			leaseParams.leaseDuration,
			leaseParams.leaseRenewDeadline,
			leaseParams.leaseRetryPeriod,
			leaseParams.leaderRetryElectGap)

		if shouldError {
			Expect(err).Should(HaveOccurred())
		} else {
			Expect(err).ShouldNot(HaveOccurred())
		}
	},
		Entry("no namespace", func() *newLeaseElectorParams {
			globalParams.leaseLockNS = ""
			return globalParams
		}, true),
		Entry("no name", func() *newLeaseElectorParams {
			globalParams.leaseLockName = ""
			return globalParams
		}, true),
		Entry("no identity", func() *newLeaseElectorParams {
			globalParams.leaseLockIdentity = ""
			return globalParams
		}, true),
		Entry("no lease duration", func() *newLeaseElectorParams {
			globalParams.leaseDuration = nil
			return globalParams
		}, true),
		Entry("no lease renew deadline", func() *newLeaseElectorParams {
			globalParams.leaseRenewDeadline = nil
			return globalParams
		}, true),
		Entry("no lease retry period", func() *newLeaseElectorParams {
			globalParams.leaseRetryPeriod = nil
			return globalParams
		}, true),
		Entry("no lease retry elect gap", func() *newLeaseElectorParams {
			globalParams.leaderRetryElectGap = nil
			return globalParams
		}, true),
		Entry("good", func() *newLeaseElectorParams {
			return globalParams
		}, false),
	)

	Describe("register spider lease elector", func() {
		var spiderLeaseElector SpiderLeaseElector
		var err error
		var becameLeader bool
		var lostLeader bool
		var wg sync.WaitGroup

		BeforeEach(func() {
			spiderLeaseElector, err = NewLeaseElector(globalParams.leaseLockNS, globalParams.leaseLockName, globalParams.leaseLockIdentity,
				globalParams.leaseDuration, globalParams.leaseRenewDeadline, globalParams.leaseRetryPeriod, globalParams.leaderRetryElectGap)
			Expect(err).NotTo(HaveOccurred())
			becameLeader = false
			lostLeader = false
			wg = sync.WaitGroup{}
		})

		It("check leader election function", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel() // Ensure context cancellation is handled after test

			// Define the leader election callbacks
			callbacks := leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					becameLeader = true
				},
				OnStoppedLeading: func() {
					lostLeader = true
				},
				OnNewLeader: func(identity string) {
					if identity == globalParams.leaseLockIdentity {
						becameLeader = true
					}
				},
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				err = spiderLeaseElector.Run(ctx, fake.NewSimpleClientset(), callbacks)
				Expect(err).NotTo(HaveOccurred())
			}()

			// Wait for leader election to happen
			Eventually(func() bool { return becameLeader }).WithTimeout(5 * time.Second).Should(BeTrue())
			Expect(spiderLeaseElector.GetLeader()).Should(Equal(globalParams.leaseLockIdentity))

			// Simulate context cancellation to stop leader election
			cancel()

			// Ensure context is canceled and leadership is lost
			Eventually(ctx.Done()).WithTimeout(1 * time.Second).Should(BeClosed())
			Eventually(func() bool { return lostLeader }).WithTimeout(3 * time.Second).Should(BeTrue())

			wg.Wait()
		})
	})

})
