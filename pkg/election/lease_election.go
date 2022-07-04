// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package election

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var logger = logutils.Logger.Named("Lease-Lock-Election")

const (
	// Values taken from: https://github.com/kubernetes/component-base/blob/master/config/v1alpha1/defaults.go
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second

	// default retry elect gap duration
	defaultRetryElectGap = 1 * time.Second

	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

type LeaseLockOptions struct {
	LeaseLockName      string
	LeaseLockNamespace string
	LeaseLockIdentity  string
	LeaseDuration      *time.Duration
	LeaseRenewDeadline *time.Duration
	LeaseRetryPeriod   *time.Duration
	RetryElectGap      *time.Duration
}

// setLeaseLockOptionsDefaults set default values for leaseLockOptions fields.
func setLeaseLockOptionsDefaults(options LeaseLockOptions) (LeaseLockOptions, error) {
	if options.LeaseLockName == "" {
		options.LeaseLockName = constant.AnnotationPre + "-" + resourcelock.LeasesResourceLock
	}

	if options.LeaseLockNamespace == "" {
		var err error
		options.LeaseLockNamespace, err = getInClusterNamespace()
		if err != nil {
			return LeaseLockOptions{}, fmt.Errorf("unable to find leader election namespace: %w", err)
		}
	}

	// Leader id needs to be unique
	hostName, err := os.Hostname()
	if err != nil {
		return LeaseLockOptions{}, fmt.Errorf("unable to find host name: %w", err)
	}
	options.LeaseLockIdentity = hostName + "_" + string(uuid.NewUUID())

	leaseDuration, renewDeadline, retryPeriod, retryElectGap := defaultLeaseDuration, defaultRenewDeadline, defaultRetryPeriod, defaultRetryElectGap
	if options.LeaseDuration == nil {
		options.LeaseDuration = &leaseDuration
	}

	if options.LeaseRenewDeadline == nil {
		options.LeaseRenewDeadline = &renewDeadline
	}

	if options.LeaseRetryPeriod == nil {
		options.LeaseRetryPeriod = &retryPeriod
	}

	if options.RetryElectGap == nil {
		options.RetryElectGap = &retryElectGap
	}

	return options, nil
}

type SpiderLeaseElector interface {
	// TryToElect will elect continually
	TryToElect(ctx context.Context)
	// IsElected returns a boolean value to check current Elector whether is a leader
	IsElected() bool
}

type SpiderLeader struct {
	lock.RWMutex

	isLeader      bool
	leaderElector *leaderelection.LeaderElector

	// unique sign
	leaseLockIdentity string
	retryElectGap     time.Duration
}

// NewLeaseElector will return a SpiderLeaseElector object
func NewLeaseElector(options LeaseLockOptions) (SpiderLeaseElector, error) {
	// Set default values for options fields
	opts, err := setLeaseLockOptionsDefaults(options)
	if nil != err {
		return nil, err
	}

	sl := &SpiderLeader{
		isLeader:          false,
		leaseLockIdentity: opts.LeaseLockIdentity,
		retryElectGap:     *opts.RetryElectGap,
	}

	err = sl.register(options)
	if nil != err {
		return nil, err
	}

	return sl, nil
}

// register will new client-go LeaderElector object with options configurations
func (sl *SpiderLeader) register(options LeaseLockOptions) error {
	// Set default values for options fields
	lockOptionsDefaults, err := setLeaseLockOptionsDefaults(options)
	if nil != err {
		return fmt.Errorf("failed to set lease lock options default value, error: %w", err)
	}

	coordinationClient, err := coordinationv1client.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return fmt.Errorf("unable to new coordination client: %w", err)
	}

	leaseLock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockOptionsDefaults.LeaseLockName,
			Namespace: lockOptionsDefaults.LeaseLockNamespace,
		},
		Client: coordinationClient,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: lockOptionsDefaults.LeaseLockIdentity,
		},
	}

	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          leaseLock,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				sl.Lock()
				sl.isLeader = true
				sl.Unlock()
				logger.Sugar().Infof("leader elected: %s", lockOptionsDefaults.LeaseLockIdentity)
			},
			OnStoppedLeading: func() {
				// we can do cleanup here
				sl.Lock()
				sl.isLeader = false
				sl.Unlock()
				logger.Sugar().Warnf("leader lost: %s", lockOptionsDefaults.LeaseLockIdentity)
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

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please specify LeaderElectionNamespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := ioutil.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

func (sl *SpiderLeader) IsElected() bool {
	sl.RLock()
	defer sl.RUnlock()

	return sl.isLeader
}

func (sl *SpiderLeader) TryToElect(ctx context.Context) {
	logger.Sugar().Infof("'%s' is trying to elect", sl.leaseLockIdentity)

	for {
		// Once a node acquire the lease lock and become the leader, it will renew the lease lock continually until it failed to interact with API server.
		// In this case the node will lose leader title and try to elect again.
		// If there's a leader and another node will try to acquire the lease lock persistently until the leader renew failed.
		sl.leaderElector.Run(ctx)

		logger.Sugar().Infof("'%s' is continue to elect", sl.leaseLockIdentity)
		time.Sleep(sl.retryElectGap)
	}
}
