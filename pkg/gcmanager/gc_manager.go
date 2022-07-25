// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/election"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

type GarbageCollectionConfig struct {
	EnableGCIP                bool
	EnableGCForTerminatingPod bool

	ReleaseIPWorkerNum     int
	GCIPChannelBuffer      int
	MaxPodEntryDatabaseCap int

	DefaultGCIntervalDuration int
	TracePodGapDuration       int
	GCSignalTimeoutDuration   int
	GCSignalGapDuration       int
	AdditionalGraceDelay      int
}

var logger *zap.Logger

type GCManager interface {
	Start(ctx context.Context) error

	GetPodDatabase() PodDBer

	TriggerGCAll()

	Health()
}

var _ GCManager = &SpiderGC{}

type SpiderGC struct {
	KClient client.Client
	PodDB   PodDBer

	// env configuration
	gcConfig *GarbageCollectionConfig

	// signal
	gcSignal         chan struct{}
	gcIPPoolIPSignal chan gcIPPoolIPIdentify

	controllerMgr ctrl.Manager
	wepMgr        workloadendpointmanager.WorkloadEndpointManager
	ippoolMgr     ippoolmanager.IPPoolManager
	podMgr        podmanager.PodManager

	leader election.SpiderLeaseElector
}

func NewGCManager(ctx context.Context, client client.Client, config *GarbageCollectionConfig,
	crdMgr ctrl.Manager,
	wepManager workloadendpointmanager.WorkloadEndpointManager,
	ippoolManager ippoolmanager.IPPoolManager,
	podManager podmanager.PodManager,
	spiderControllerLeader election.SpiderLeaseElector) (GCManager, error) {
	if config == nil {
		return nil, fmt.Errorf("gc configuration must be specified")
	}

	if client == nil {
		return nil, fmt.Errorf("k8s client must be specified")
	}

	if crdMgr == nil {
		return nil, fmt.Errorf("crd manager must be specified")
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
		KClient:  client,
		PodDB:    NewPodDBer(config.MaxPodEntryDatabaseCap),
		gcConfig: config,

		gcSignal:         make(chan struct{}, 1),
		gcIPPoolIPSignal: make(chan gcIPPoolIPIdentify, config.GCIPChannelBuffer),

		controllerMgr: crdMgr,
		wepMgr:        wepManager,
		ippoolMgr:     ippoolManager,
		podMgr:        podManager,

		leader: spiderControllerLeader,
	}

	return spiderGC, nil
}

func (s *SpiderGC) Start(ctx context.Context) error {
	if !s.gcConfig.EnableGCIP {
		logger.Warn("IP garbage collection is forbidden")
		return nil
	}

	// 1. register pod reconcile and start to watch
	err := s.registerPodReconcile()
	if nil != err {
		return fmt.Errorf("register pod reconcile failed '%v'", err)
	}

	// trace pod worker
	go s.tracePodWorker()

	// monitor gc signal from CLI or DefaultGCInterval
	go s.monitorGCSignal(ctx)

	for i := 0; i < s.gcConfig.ReleaseIPWorkerNum; i++ {
		go s.releaseIPPoolIPExecutor(ctx, i)
	}

	logger.Info("running IP garbage collection")
	return nil
}

func (s *SpiderGC) GetPodDatabase() PodDBer {
	// TODO (Icarus9913): ??????
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

func (s *SpiderGC) Health() {
	//TODO (Icarus9913): implement me
}
