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
		ipamAllocationDurationSeconds.Record(ctx, allocationDuration)

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
func (ddc *releaseDurationConstruct) RecordIPAMReleaseDuration(ctx context.Context, releaseDuration float64) {
	if !globalEnableMetric {
		return
	}

	go func() {
		// latest release duration
		ipamReleaseLatestDurationSeconds.Record(releaseDuration)

		// release duration histogram
		ipamAllocationDurationSeconds.Record(ctx, releaseDuration)

		ddc.cacheLock.Lock()

		// IPAM average release duration
		ddc.releaseAvgDuration = (ddc.releaseAvgDuration*float64(ddc.releaseCounts) + releaseDuration) / float64(ddc.releaseCounts+1)
		ddc.releaseCounts++
		ipamReleaseAverageDurationSeconds.Record(ddc.releaseAvgDuration)

		// IPAM maximum release duration
		if releaseDuration > ddc.maxReleaseDuration {
			ddc.maxReleaseDuration = releaseDuration
			ipamReleaseMaxDurationSeconds.Record(ddc.maxReleaseDuration)
		}

		// IPAM minimum release duration
		if ddc.releaseCounts == 1 || releaseDuration < ddc.minReleaseDuration {
			ddc.minReleaseDuration = releaseDuration
			ipamReleaseMinDurationSeconds.Record(ddc.minReleaseDuration)
		}

		ddc.cacheLock.Unlock()
	}()
}
