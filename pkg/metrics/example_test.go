// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"time"
)

func ExampleSinceInSeconds() {
	startTime := time.Now()
	fmt.Println(SinceInSeconds(startTime))
	// Output:
	// 6.000178514
}

func ExampleRecordIncWithAttr() {
	ctx := context.TODO()
	// The third param depends on the sencond one.
	RecordIncWithAttr(ctx, NodeAllocatedIpTotalCounts, TotalIpAttr("node1", "default/pool1"))
	// execute the IP allocate/deallocate steps...
}

func ExampleTimerWithAttr() {
	ctx := context.TODO()
	stop := TimerWithAttr(ctx, NodeAllocatedIpDuration, SuccessIpDurationAttr("node1"))
	// execute the IP allocate steps....
	// if IP allocate successfully
	if true {
		stop()
	}
}
