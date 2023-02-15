// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

// AllocDurationConstruct is Singleton
var AllocDurationConstruct = new(allocationDurationConstruct)

// DeallocDurationConstruct is Singleton
var DeallocDurationConstruct = new(releaseDurationConstruct)

type allocationDurationConstruct struct {
	cacheLock lock.RWMutex

	allocationAvgDuration float64
	maxAllocationDuration float64
	minAllocationDuration float64

	allocationCounts int
}

// RecordIPAMAllocationDuration serves for spiderpool agent IPAM allocation.
func (adc *allocationDurationConstruct) RecordIPAMAllocationDuration(ctx context.Context, allocationDuration float64) {
	if !globalEnableMetric {
		return
	}

	go func() {
		// latest allocation duration
		ipamAllocationLatestDurationSeconds.Record(allocationDuration)

		// allocation duration histogram
		ipamAllocationDurationSecondsHistogram.Record(ctx, allocationDuration)

		adc.cacheLock.Lock()

		// IPAM average allocation duration
		adc.allocationAvgDuration = (adc.allocationAvgDuration*float64(adc.allocationCounts) + allocationDuration) / float64(adc.allocationCounts+1)
		adc.allocationCounts++
		ipamAllocationAverageDurationSeconds.Record(adc.allocationAvgDuration)

		// IPAM maximum allocation duration
		if allocationDuration > adc.maxAllocationDuration {
			adc.maxAllocationDuration = allocationDuration
			ipamAllocationMaxDurationSeconds.Record(adc.maxAllocationDuration)
		}

		// IPAM minimum allocation duration
		if adc.allocationCounts == 1 || allocationDuration < adc.minAllocationDuration {
			adc.minAllocationDuration = allocationDuration
			ipamAllocationMinDurationSeconds.Record(adc.minAllocationDuration)
		}

		adc.cacheLock.Unlock()
	}()
}

type releaseDurationConstruct struct {
	cacheLock lock.RWMutex

	releaseAvgDuration float64
	maxReleaseDuration float64
	minReleaseDuration float64

	releaseCounts int
}

// RecordIPAMReleaseDuration serves for spiderpool agent IPAM allocation.
func (rdc *releaseDurationConstruct) RecordIPAMReleaseDuration(ctx context.Context, releaseDuration float64) {
	if !globalEnableMetric {
		return
	}

	go func() {
		// latest release duration
		ipamReleaseLatestDurationSeconds.Record(releaseDuration)

		// release duration histogram
		ipamReleaseDurationSecondsHistogram.Record(ctx, releaseDuration)

		rdc.cacheLock.Lock()

		// IPAM average release duration
		rdc.releaseAvgDuration = (rdc.releaseAvgDuration*float64(rdc.releaseCounts) + releaseDuration) / float64(rdc.releaseCounts+1)
		rdc.releaseCounts++
		ipamReleaseAverageDurationSeconds.Record(rdc.releaseAvgDuration)

		// IPAM maximum release duration
		if releaseDuration > rdc.maxReleaseDuration {
			rdc.maxReleaseDuration = releaseDuration
			ipamReleaseMaxDurationSeconds.Record(rdc.maxReleaseDuration)
		}

		// IPAM minimum release duration
		if rdc.releaseCounts == 1 || releaseDuration < rdc.minReleaseDuration {
			rdc.minReleaseDuration = releaseDuration
			ipamReleaseMinDurationSeconds.Record(rdc.minReleaseDuration)
		}

		rdc.cacheLock.Unlock()
	}()
}
