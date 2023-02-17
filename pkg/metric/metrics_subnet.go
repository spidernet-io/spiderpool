// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

// AutoPoolCreationDurationConstruct is Singleton
var AutoPoolCreationDurationConstruct = new(autoPoolCreationDurationConstruct)

// AutoPoolScaleDurationConstruct is Singleton
var AutoPoolScaleDurationConstruct = new(autoPoolScaleDurationConstruct)

type autoPoolCreationDurationConstruct struct {
	cacheLock lock.RWMutex

	creationAvgDuration float64
	maxCreationDuration float64
	minCreationDuration float64

	creationCounts int
}

func (a *autoPoolCreationDurationConstruct) RecordAutoPoolCreationDuration(ctx context.Context, duration float64) {
	if !globalEnableMetric {
		return
	}

	go func() {
		// latest auto-created IPPool creation duration
		autoPoolCreationLatestDurationSeconds.Record(duration)

		// auto-created IPPool creation duration histogram
		autoPoolCreationDurationSecondsHistogram.Record(ctx, duration)

		a.cacheLock.Lock()

		// auto-created IPPool creation average duration
		a.creationAvgDuration = (a.creationAvgDuration*float64(a.creationCounts) + duration) / float64(a.creationCounts+1)
		a.creationCounts++
		autoPoolCreationAverageDurationSeconds.Record(a.creationAvgDuration)

		// auto-created IPPool creation maximum duration
		if duration > a.maxCreationDuration {
			a.maxCreationDuration = duration
			autoPoolCreationMaxDurationSeconds.Record(a.maxCreationDuration)
		}

		// auto-created IPPool creation minimum duration
		if a.creationCounts == 1 || duration < a.minCreationDuration {
			a.minCreationDuration = duration
			autoPoolCreationMinDurationSeconds.Record(a.minCreationDuration)
		}

		a.cacheLock.Unlock()
	}()
}

type autoPoolScaleDurationConstruct struct {
	cacheLock lock.RWMutex

	scaleAvgDuration float64
	maxScaleDuration float64
	minScaleDuration float64

	scaleCounts int
}

func (a *autoPoolScaleDurationConstruct) RecordAutoPoolScaleDuration(ctx context.Context, duration float64) {
	if !globalEnableMetric {
		return
	}

	go func() {
		// latest auto-created IPPool creation duration
		autoPoolScaleLatestDurationSeconds.Record(duration)

		// auto-created IPPool creation duration histogram
		autoPoolScaleDurationSecondsHistogram.Record(ctx, duration)

		a.cacheLock.Lock()

		// auto-created IPPool creation average duration
		a.scaleAvgDuration = (a.scaleAvgDuration*float64(a.scaleCounts) + duration) / float64(a.scaleCounts+1)
		a.scaleCounts++
		autoPoolScaleAverageDurationSeconds.Record(a.scaleAvgDuration)

		// auto-created IPPool creation maximum duration
		if duration > a.maxScaleDuration {
			a.maxScaleDuration = duration
			autoPoolScaleMaxDurationSeconds.Record(a.maxScaleDuration)
		}

		// auto-created IPPool creation minimum duration
		if a.scaleCounts == 1 || duration < a.minScaleDuration {
			a.minScaleDuration = duration
			autoPoolScaleMinDurationSeconds.Record(a.minScaleDuration)
		}

		a.cacheLock.Unlock()
	}()
}
