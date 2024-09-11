// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package election

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var logger *zap.Logger

// SpiderLeaseElector interface defines the leader election methods
type SpiderLeaseElector interface {
	Run(ctx context.Context, clientSet kubernetes.Interface, callbacks leaderelection.LeaderCallbacks) error
	IsElected() bool
	GetLeader() string
}

// SpiderLeader implements SpiderLeaseElector
type SpiderLeader struct {
	lock.RWMutex
	leaseLockName       string
	leaseLockNamespace  string
	leaseLockIdentity   string
	leaseDuration       time.Duration
	leaseRenewDeadline  time.Duration
	leaseRetryPeriod    time.Duration
	leaderRetryElectGap time.Duration
	isLeader            bool
	leaderElector       *leaderelection.LeaderElector
}

func NewLeaseElector(leaseLockNS, leaseLockName, leaseLockIdentity string,
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

	return sl, nil
}

// Run executes the leader election process with the given callbacks
func (sl *SpiderLeader) Run(ctx context.Context, clientSet kubernetes.Interface, callbacks leaderelection.LeaderCallbacks) error {
	logger = logutils.Logger.Named("Lease-Lock-Election")

	// Create a LeaseLock object that defines the resource to be locked
	leaseLock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      sl.leaseLockName,
			Namespace: sl.leaseLockNamespace,
		},
		Client: clientSet.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: sl.leaseLockIdentity,
		},
	}

	// Initialize the leader elector with the configuration
	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          leaseLock,
		LeaseDuration: sl.leaseDuration,
		RenewDeadline: sl.leaseRenewDeadline,
		RetryPeriod:   sl.leaseRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			// Callback when this instance becomes the leader
			OnStartedLeading: func(ctx context.Context) {
				sl.Lock()
				sl.isLeader = true
				sl.Unlock()
				callbacks.OnStartedLeading(ctx)
				logger.Sugar().Infof("lease %s/%s leader election succeeded, leader identity: [%s]", sl.leaseLockNamespace, sl.leaseLockName, sl.leaseLockIdentity)
			},
			// Callback when this instance stops being the leader
			OnStoppedLeading: func() {
				sl.Lock()
				sl.isLeader = false
				sl.Unlock()
				callbacks.OnStoppedLeading()
				logger.Sugar().Warnf("lease %s/%s leader election failed, leader lost: [%s]", sl.leaseLockNamespace, sl.leaseLockName, sl.leaseLockIdentity)
			},
			// Callback when a new leader is elected
			OnNewLeader: callbacks.OnNewLeader,
		},
		ReleaseOnCancel: true,
	})

	// Return error if leader elector creation fails
	if err != nil {
		return fmt.Errorf("unable to create new leader elector: %w", err)
	}

	// Assign the leader elector to the SpiderLeader object
	sl.leaderElector = le

	// Start the leader election process
	sl.leaderElector.Run(ctx)

	return nil
}

// IsElected checks if the current instance is the leader
func (sl *SpiderLeader) IsElected() bool {
	sl.RLock()
	defer sl.RUnlock()
	return sl.isLeader
}

// GetLeader returns the current leader's identity
func (sl *SpiderLeader) GetLeader() string {
	return sl.leaderElector.GetLeader()
}
