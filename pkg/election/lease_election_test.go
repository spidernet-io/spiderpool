// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package election

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
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

	var (
		globalParams  *newLeaseElectorParams
		leaseDuration = 15 * time.Second
		renewDeadline = 2 * time.Second
		retryPeriod   = 1 * time.Second
		retryElectGap = 5 * time.Second
	)

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
		BeforeEach(func() {
			spiderLeaseElector, err = NewLeaseElector(globalParams.leaseLockNS, globalParams.leaseLockName, globalParams.leaseLockIdentity,
				globalParams.leaseDuration, globalParams.leaseRenewDeadline, globalParams.leaseRetryPeriod, globalParams.leaderRetryElectGap)
		})

		It("check leader election function", func() {
			ctx, cancel := context.WithCancel(context.TODO())
			err = spiderLeaseElector.Run(ctx, fake.NewSimpleClientset())
			Expect(err).NotTo(HaveOccurred())

			// wait for us to become leader
			Eventually(spiderLeaseElector.IsElected).WithTimeout(5 * time.Second).Should(BeTrue())
			Expect(spiderLeaseElector.GetLeader()).Should(Equal(globalParams.leaseLockIdentity))

			cancel()
			Eventually(ctx.Done()).WithTimeout(1 * time.Second).Should(BeClosed())

			// we will lose the leader
			Eventually(spiderLeaseElector.IsElected).WithTimeout(3 * time.Second).Should(BeFalse())
		})
	})
})
