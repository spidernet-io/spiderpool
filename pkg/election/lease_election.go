// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package election

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var logger *zap.Logger

type SpiderLeaseElector interface {
	// IsElected returns a boolean value to check current Elector whether is a leader
	IsElected() bool
}

type SpiderLeader struct {
	lock.RWMutex

	leaseLockName       string
	leaseLockNamespace  string
	leaseLockIdentity   string
	leaseDuration       time.Duration
	leaseRenewDeadline  time.Duration
	leaseRetryPeriod    time.Duration
	leaderRetryElectGap time.Duration

	isLeader      bool
	leaderElector *leaderelection.LeaderElector
}

// NewLeaseElector will return a SpiderLeaseElector object
func NewLeaseElector(ctx context.Context, leaseLockNS, leaseLockName, leaseLockIdentity string,
	leaseDuration, leaseRenewDeadline, leaseRetryPeriod, leaderRetryElectGap *time.Duration) (SpiderLeaseElector, error) {
	if len(leaseLockNS) == 0 {
		return nil, fmt.Errorf("failed to new lease elector: Lease Lock Namespace must be specified")
	}

	if len(leaseLockName) == 0 {
		return nil, fmt.Errorf("failed to new lease elector: Lease Lock Name must be specified")
	}

	if len(leaseLockIdentity) == 0 {
		return nil, fmt.Errorf("failed to new lease elector: Lease Lock Identity must be specified")
	}

	if leaseDuration == nil {
		return nil, fmt.Errorf("failed to new lease elector: Lease Duration must be specified")
	}

	if leaseRenewDeadline == nil {
		return nil, fmt.Errorf("failed to new lease elector: Lease Renew Deadline must be specified")
	}

	if leaseRetryPeriod == nil {
		return nil, fmt.Errorf("failed to new lease elector: Lease Retry Period must be specified")
	}

	if leaderRetryElectGap == nil {
		return nil, fmt.Errorf("failed to new lease elector: Leader Retry Gap must be specified")
	}

	re := regexp.MustCompile(constant.QualifiedK8sObjNameFmt)
	if !re.MatchString(leaseLockName) {
		return nil, fmt.Errorf("the given leaseLockName is invalid, regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')")
	}

	sl := &SpiderLeader{
		isLeader:            false,
		leaseLockName:       leaseLockName,
		leaseLockNamespace:  leaseLockNS,
		leaseLockIdentity:   leaseLockIdentity,
		leaseDuration:       *leaseDuration,
		leaseRenewDeadline:  *leaseRenewDeadline,
		leaseRetryPeriod:    *leaseRetryPeriod,
		leaderRetryElectGap: *leaderRetryElectGap,
	}

	err := sl.register()
	if nil != err {
		return nil, err
	}

	logger = logutils.Logger.Named("Lease-Lock-Election")

	go sl.tryToElect(ctx)

	return sl, nil
}

// register will new client-go LeaderElector object with options configurations
func (sl *SpiderLeader) register() error {
	coordinationClient, err := coordinationv1client.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return fmt.Errorf("unable to new coordination client: %w", err)
	}

	leaseLock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      sl.leaseLockName,
			Namespace: sl.leaseLockNamespace,
		},
		Client: coordinationClient,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: sl.leaseLockIdentity,
		},
	}

	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          leaseLock,
		LeaseDuration: sl.leaseDuration,
		RenewDeadline: sl.leaseRenewDeadline,
		RetryPeriod:   sl.leaseRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				sl.Lock()
				sl.isLeader = true
				sl.Unlock()
				logger.Sugar().Infof("leader elected: %s/%s/%s", sl.leaseLockNamespace, sl.leaseLockName, sl.leaseLockIdentity)
			},
			OnStoppedLeading: func() {
				// we can do cleanup here
				sl.Lock()
				sl.isLeader = false
				sl.Unlock()
				logger.Sugar().Warnf("leader lost: %s/%s/%s", sl.leaseLockNamespace, sl.leaseLockName, sl.leaseLockIdentity)
			},
		},
		ReleaseOnCancel: true,
	})
	if nil != err {
		return fmt.Errorf("unable to new leader elector: %w", err)
	}

	sl.leaderElector = le
	return nil
}

func (sl *SpiderLeader) IsElected() bool {
	sl.RLock()
	defer sl.RUnlock()

	return sl.isLeader
}

// tryToElect will elect continually
func (sl *SpiderLeader) tryToElect(ctx context.Context) {
	for {
		logger.Sugar().Infof("'%s/%s/%s' is trying to elect",
			sl.leaseLockNamespace, sl.leaseLockName, sl.leaseLockIdentity)

		// Once a node acquire the lease lock and become the leader, it will renew the lease lock continually until it failed to interact with API server.
		// In this case the node will lose leader title and try to elect again.
		// If there's a leader and another node will try to acquire the lease lock persistently until the leader renew failed.
		sl.leaderElector.Run(ctx)

		logger.Sugar().Warnf("'%s/%s/%s' election request disconnected, and it will continue to elect after '%d' seconds",
			sl.leaseLockNamespace, sl.leaseLockName, sl.leaseLockIdentity, sl.leaderRetryElectGap)

		time.Sleep(sl.leaderRetryElectGap)
	}
}
