// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

// IPAMDurationConstruct is Singleton
var IPAMDurationConstruct = new(ipamDurationConstruct)

type ipamDurationConstruct struct {
	allocate      durationConstruct
	release       durationConstruct
	allocateLimit durationConstruct
	releaseLimit  durationConstruct
}

type durationConstruct struct {
	cacheLock lock.RWMutex

	avgDuration float64
	maxDuration float64
	minDuration float64

	counts int
}

// RecordIPAMAllocationDuration serves for spiderpool agent IPAM allocation.
func (idc *ipamDurationConstruct) RecordIPAMAllocationDuration(ctx context.Context, allocationDuration float64) {
	if !globalEnableMetric {
		return
	}

	// latest allocation duration
	ipamAllocationLatestDurationSeconds.Record(allocationDuration)

	// allocation duration histogram
	ipamAllocationDurationSecondsHistogram.Record(ctx, allocationDuration)

	idc.allocate.cacheLock.Lock()

	// IPAM average allocation duration
	idc.allocate.avgDuration = (idc.allocate.avgDuration*float64(idc.allocate.counts) + allocationDuration) / float64(idc.allocate.counts+1)
	idc.allocate.counts++
	ipamAllocationAverageDurationSeconds.Record(idc.allocate.avgDuration)

	// IPAM maximum allocation duration
	if allocationDuration > idc.allocate.maxDuration {
		idc.allocate.maxDuration = allocationDuration
		ipamAllocationMaxDurationSeconds.Record(idc.allocate.maxDuration)
	}

	// IPAM minimum allocation duration
	if idc.allocate.counts == 1 || allocationDuration < idc.allocate.minDuration {
		idc.allocate.minDuration = allocationDuration
		ipamAllocationMinDurationSeconds.Record(idc.allocate.minDuration)
	}

	idc.allocate.cacheLock.Unlock()
}

// RecordIPAMReleaseDuration serves for spiderpool agent IPAM allocation.
func (idc *ipamDurationConstruct) RecordIPAMReleaseDuration(ctx context.Context, releaseDuration float64) {
	if !globalEnableMetric {
		return
	}

	// latest release duration
	ipamReleaseLatestDurationSeconds.Record(releaseDuration)

	// release duration histogram
	ipamReleaseDurationSecondsHistogram.Record(ctx, releaseDuration)

	idc.release.cacheLock.Lock()

	// IPAM average release duration
	idc.release.avgDuration = (idc.release.avgDuration*float64(idc.release.counts) + releaseDuration) / float64(idc.release.counts+1)
	idc.release.counts++
	ipamReleaseAverageDurationSeconds.Record(idc.release.avgDuration)

	// IPAM maximum release duration
	if releaseDuration > idc.release.maxDuration {
		idc.release.maxDuration = releaseDuration
		ipamReleaseMaxDurationSeconds.Record(idc.release.maxDuration)
	}

	// IPAM minimum release duration
	if idc.release.counts == 1 || releaseDuration < idc.release.minDuration {
		idc.release.minDuration = releaseDuration
		ipamReleaseMinDurationSeconds.Record(idc.release.minDuration)
	}

	idc.release.cacheLock.Unlock()
}

func (idc *ipamDurationConstruct) RecordIPAMAllocationLimitDuration(ctx context.Context, limitDuration float64) {
	if !globalEnableMetric {
		return
	}

	ipamAllocationLatestLimitDurationSeconds.Record(limitDuration)
	ipamAllocationLimitDurationSecondsHistogram.Record(ctx, limitDuration)

	idc.allocateLimit.cacheLock.Lock()
	defer idc.allocateLimit.cacheLock.Unlock()

	idc.allocateLimit.avgDuration = (idc.allocateLimit.avgDuration*float64(idc.allocateLimit.counts) + limitDuration) / float64(idc.allocateLimit.counts+1)
	idc.allocateLimit.counts++
	ipamAllocationAverageLimitDurationSeconds.Record(idc.allocateLimit.avgDuration)

	if limitDuration > idc.allocateLimit.maxDuration {
		idc.allocateLimit.maxDuration = limitDuration
		ipamAllocationMaxLimitDurationSeconds.Record(idc.allocateLimit.maxDuration)
	}

	if idc.allocateLimit.counts == 1 || limitDuration < idc.allocateLimit.minDuration {
		idc.allocateLimit.minDuration = limitDuration
		ipamAllocationMinLimitDurationSeconds.Record(idc.allocateLimit.minDuration)
	}
}

func (idc *ipamDurationConstruct) RecordIPAMReleaseLimitDuration(ctx context.Context, limitDuration float64) {
	if !globalEnableMetric {
		return
	}

	ipamReleaseLatestLimitDurationSeconds.Record(limitDuration)
	ipamReleaseLimitDurationSecondsHistogram.Record(ctx, limitDuration)

	idc.releaseLimit.cacheLock.Lock()
	defer idc.releaseLimit.cacheLock.Unlock()

	idc.releaseLimit.avgDuration = (idc.releaseLimit.avgDuration*float64(idc.releaseLimit.counts) + limitDuration) / float64(idc.releaseLimit.counts+1)
	idc.releaseLimit.counts++
	ipamReleaseAverageLimitDurationSeconds.Record(idc.releaseLimit.avgDuration)

	if limitDuration > idc.releaseLimit.maxDuration {
		idc.releaseLimit.maxDuration = limitDuration
		ipamReleaseMaxLimitDurationSeconds.Record(idc.releaseLimit.maxDuration)
	}

	if idc.releaseLimit.counts == 1 || limitDuration < idc.releaseLimit.minDuration {
		idc.releaseLimit.minDuration = limitDuration
		ipamReleaseMinLimitDurationSeconds.Record(idc.releaseLimit.minDuration)
	}
}
