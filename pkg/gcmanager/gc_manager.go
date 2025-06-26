// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/kubevirtmanager"
	"github.com/spidernet-io/spiderpool/pkg/limiter"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"

	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type GarbageCollectionConfig struct {
	EnableGCIP                                     bool
	EnableGCStatelessTerminatingPodOnReadyNode     bool
	EnableGCStatelessTerminatingPodOnNotReadyNode  bool
	EnableGCStatelessRunningPodOnEmptyPodStatusIPs bool
	EnableStatefulSet                              bool
	EnableKubevirtStaticIP                         bool
	EnableCleanOutdatedEndpoint                    bool

	ReleaseIPWorkerNum     int
	GCIPChannelBuffer      int
	MaxPodEntryDatabaseCap int
	WorkQueueMaxRetries    int

	DefaultGCIntervalDuration int
	TracePodGapDuration       int
	GCSignalTimeoutDuration   int
	GCSignalGapDuration       int
	AdditionalGraceDelay      int

	LeaderRetryElectGap time.Duration
}

var logger *zap.Logger

type GCManager interface {
	Start(ctx context.Context) <-chan error
	GetPodDatabase() PodDBer
	TriggerGCAll()
	Health() bool
}

var _ GCManager = &SpiderGC{}

type SpiderGC struct {
	k8ClientSet *kubernetes.Clientset
	PodDB       PodDBer

	// env configuration
	gcConfig *GarbageCollectionConfig

	// signal
	gcSignal         chan struct{}
	gcIPPoolIPSignal chan *PodEntry

	wepMgr      workloadendpointmanager.WorkloadEndpointManager
	ippoolMgr   ippoolmanager.IPPoolManager
	podMgr      podmanager.PodManager
	stsMgr      statefulsetmanager.StatefulSetManager
	kubevirtMgr kubevirtmanager.KubevirtManager
	nodeMgr     nodemanager.NodeManager
	leader      election.SpiderLeaseElector

	informerFactory informers.SharedInformerFactory
	gcLimiter       limiter.Limiter
	Locker          lock.Mutex
}

func NewGCManager(clientSet *kubernetes.Clientset, config *GarbageCollectionConfig,
	wepManager workloadendpointmanager.WorkloadEndpointManager,
	ippoolManager ippoolmanager.IPPoolManager,
	podManager podmanager.PodManager,
	stsManager statefulsetmanager.StatefulSetManager,
	kubevirtMgr kubevirtmanager.KubevirtManager,
	nodeMgr nodemanager.NodeManager,
	spiderControllerLeader election.SpiderLeaseElector) (GCManager, error) {
	if clientSet == nil {
		return nil, fmt.Errorf("k8s ClientSet must be specified")
	}

	if config == nil {
		return nil, fmt.Errorf("gc configuration must be specified")
	}

	if wepManager == nil {
		return nil, fmt.Errorf("workload endpoint manager must be specified")
	}

	if ippoolManager == nil {
		return nil, fmt.Errorf("ippool manager must be specified")
	}

	if podManager == nil {
		return nil, fmt.Errorf("pod manager must be specified")
	}

	if spiderControllerLeader == nil {
		return nil, fmt.Errorf("spiderpool controller leader must be specified")
	}

	logger = logutils.Logger.Named("IP-GarbageCollection")

	spiderGC := &SpiderGC{
		k8ClientSet:      clientSet,
		PodDB:            NewPodDBer(config.MaxPodEntryDatabaseCap),
		gcConfig:         config,
		gcSignal:         make(chan struct{}, 1),
		gcIPPoolIPSignal: make(chan *PodEntry, config.GCIPChannelBuffer),

		wepMgr:      wepManager,
		ippoolMgr:   ippoolManager,
		podMgr:      podManager,
		stsMgr:      stsManager,
		kubevirtMgr: kubevirtMgr,
		nodeMgr:     nodeMgr,

		leader:    spiderControllerLeader,
		gcLimiter: limiter.NewLimiter(limiter.LimiterConfig{}),
		Locker:    lock.Mutex{},
	}

	return spiderGC, nil
}

func (s *SpiderGC) Start(ctx context.Context) <-chan error {
	errCh := make(chan error)

	if !s.gcConfig.EnableGCIP {
		logger.Warn("IP garbage collection is forbidden")
		return errCh
	}

	// start pod informer
	go s.startPodInformer(ctx)

	// trace pod worker
	go s.tracePodWorker(ctx)

	// monitor gc signal from CLI or DefaultGCInterval
	go s.monitorGCSignal(ctx)

	for i := 1; i <= s.gcConfig.ReleaseIPWorkerNum; i++ {
		go s.releaseIPPoolIPExecutor(ctx, i)
	}

	go func() {
		err := s.gcLimiter.Start(ctx)
		if nil != err {
			errCh <- err
		}
	}()

	logger.Info("running IP garbage collection")
	return errCh
}

func (s *SpiderGC) GetPodDatabase() PodDBer {
	return s.PodDB
}

func (s *SpiderGC) TriggerGCAll() {
	logger.Info("trigger gc!")
	select {
	case s.gcSignal <- struct{}{}:
	case <-time.After(time.Duration(s.gcConfig.GCSignalTimeoutDuration) * time.Second):
		logger.Sugar().Errorf("failed to trigger GCAll, gcSignal:len=%d", len(s.gcSignal))
	}
}

const waitForCacheSyncTimeout = 5 * time.Second

func (s *SpiderGC) Health() bool {
	ctx, cancelFunc := context.WithTimeout(context.TODO(), waitForCacheSyncTimeout)
	defer cancelFunc()

	if s.leader.IsElected() {
		if s.informerFactory == nil {
			logger.Warn("the IP-GC manager pod informer is not ready")
			return false
		}

		waitForCacheSync := s.informerFactory.WaitForCacheSync(ctx.Done())
		for _, isCacheSync := range waitForCacheSync {
			if !isCacheSync {
				return false
			}
		}
	}

	return true
}
