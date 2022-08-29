// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

// AllocDurationConstruct is Singleton
var AllocDurationConstruct = new(allocationDurationConstruct)

// DeallocDurationConstruct is Singleton
var DeallocDurationConstruct = new(deallocationDurationConstruct)

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
		ipamAllocationLatestDurationSeconds.observerLock.Lock()
		*ipamAllocationLatestDurationSeconds.observerValueToReport = allocationDuration
		ipamAllocationLatestDurationSeconds.observerLock.Unlock()

		// allocation duration histogram
		ipamAllocationDurationSeconds.Record(ctx, allocationDuration)

		adc.cacheLock.Lock()

		// IPAM average allocation duration
		adc.allocationAvgDuration = (adc.allocationAvgDuration*float64(adc.allocationCounts) + allocationDuration) / float64(adc.allocationCounts+1)
		adc.allocationCounts++
		ipamAllocationAverageDurationSeconds.observerLock.Lock()
		*ipamAllocationAverageDurationSeconds.observerValueToReport = adc.allocationAvgDuration
		ipamAllocationAverageDurationSeconds.observerLock.Unlock()

		// IPAM maximum allocation duration
		if allocationDuration > adc.maxAllocationDuration {
			adc.maxAllocationDuration = allocationDuration
			ipamAllocationMaxDurationSeconds.observerLock.Lock()
			*ipamAllocationMaxDurationSeconds.observerValueToReport = adc.maxAllocationDuration
			ipamAllocationMaxDurationSeconds.observerLock.Unlock()
		}

		// IPAM minimum allocation duration
		if adc.allocationCounts == 1 || allocationDuration < adc.minAllocationDuration {
			adc.minAllocationDuration = allocationDuration
			ipamAllocationMinDurationSeconds.observerLock.Lock()
			*ipamAllocationMinDurationSeconds.observerValueToReport = adc.minAllocationDuration
			ipamAllocationMinDurationSeconds.observerLock.Unlock()
		}

		adc.cacheLock.Unlock()
	}()
}

type deallocationDurationConstruct struct {
	cacheLock lock.RWMutex

	deallocationAvgDuration float64
	maxDeallocationDuration float64
	minDeallocationDuration float64

	deallocationCounts int
}

// RecordIPAMDeallocationDuration serves for spiderpool agent IPAM allocation.
func (ddc *deallocationDurationConstruct) RecordIPAMDeallocationDuration(ctx context.Context, deallocationDuration float64) {
	if !globalEnableMetric {
		return
	}

	go func() {
		// latest deallocation duration
		ipamAllocationLatestDurationSeconds.observerLock.Lock()
		*ipamAllocationLatestDurationSeconds.observerValueToReport = deallocationDuration
		ipamAllocationLatestDurationSeconds.observerLock.Unlock()

		// deallocation duration histogram
		ipamAllocationDurationSeconds.Record(ctx, deallocationDuration)

		ddc.cacheLock.Lock()

		// IPAM average deallocation duration
		ddc.deallocationAvgDuration = (ddc.deallocationAvgDuration*float64(ddc.deallocationCounts) + deallocationDuration) / float64(ddc.deallocationCounts+1)
		ddc.deallocationCounts++
		ipamAllocationAverageDurationSeconds.observerLock.Lock()
		*ipamAllocationAverageDurationSeconds.observerValueToReport = ddc.deallocationAvgDuration
		ipamAllocationAverageDurationSeconds.observerLock.Unlock()

		// IPAM maximum deallocation duration
		if deallocationDuration > ddc.maxDeallocationDuration {
			ddc.maxDeallocationDuration = deallocationDuration
			ipamAllocationMaxDurationSeconds.observerLock.Lock()
			*ipamAllocationMaxDurationSeconds.observerValueToReport = ddc.maxDeallocationDuration
			ipamAllocationMaxDurationSeconds.observerLock.Unlock()
		}

		// IPAM minimum deallocation duration
		if ddc.deallocationCounts == 1 || deallocationDuration < ddc.minDeallocationDuration {
			ddc.minDeallocationDuration = deallocationDuration
			ipamAllocationMinDurationSeconds.observerLock.Lock()
			*ipamAllocationMinDurationSeconds.observerValueToReport = ddc.minDeallocationDuration
			ipamAllocationMinDurationSeconds.observerLock.Unlock()
		}

		ddc.cacheLock.Unlock()
	}()
}
