// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

//go:build lockdebug

package lock

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	deadlock "github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var (
	OutputWriter io.Writer = os.Stderr

	// SelfishThresholdSec is the number of seconds that should be used when
	// detecting if a lock was held for more than the specified time.
	SelfishThresholdSec = 0.5

	// Waiting for a lock for longer than DeadlockTimeout is considered a deadlock.
	// Ignored is DeadlockTimeout <= 0.
	DeadlockTimeout = 3 * time.Second
)

var (
	logger = logutils.Logger.Named("Debug-Lock")

	// selfishThresholdMsg is the message that will be printed when a lock was
	// held for more than selfishThresholdSec.
	selfishThresholdMsg = fmt.Sprintf("Goroutine took lock for more than %.2f seconds", SelfishThresholdSec)
)

func init() {
	deadlock.Opts.DeadlockTimeout = DeadlockTimeout
}

type internalRWMutex struct {
	deadlock.RWMutex
	t time.Time
}

func (i *internalRWMutex) Lock() {
	i.RWMutex.Lock()
	i.t = time.Now()
}

func (i *internalRWMutex) Unlock() {
	if sec := time.Since(i.t).Seconds(); sec >= SelfishThresholdSec {
		printStackTo(sec, debug.Stack(), OutputWriter)
	}
	i.RWMutex.Unlock()
}

func (i *internalRWMutex) UnlockIgnoreTime() {
	i.RWMutex.Unlock()
}

func (i *internalRWMutex) RLock() {
	i.RWMutex.RLock()
}

func (i *internalRWMutex) RUnlock() {
	i.RWMutex.RUnlock()
}

type internalMutex struct {
	deadlock.Mutex
	time.Time
}

func (i *internalMutex) Lock() {
	i.Mutex.Lock()
	i.Time = time.Now()
}

func (i *internalMutex) Unlock() {
	if sec := time.Since(i.Time).Seconds(); sec >= SelfishThresholdSec {
		printStackTo(sec, debug.Stack(), OutputWriter)
	}
	i.Mutex.Unlock()
}

func (i *internalMutex) UnlockIgnoreTime() {
	i.Mutex.Unlock()
}

func printStackTo(sec float64, stack []byte, writer io.Writer) {
	goRoutineNumber := []byte("0")
	newLines := 0

	if bytes.Equal([]byte("goroutine"), stack[:len("goroutine")]) {
		newLines = bytes.Count(stack, []byte{'\n'})
		goroutineLine := bytes.IndexRune(stack, '[')
		goRoutineNumber = stack[:goroutineLine]
	}

	logger.With(
		zap.Float64("Seconds", sec),
		zap.String("Goroutine", string(goRoutineNumber[len("goroutine"):len(goRoutineNumber)-1])),
	).Warn(selfishThresholdMsg)

	// A stack trace is usually in the following format:
	// goroutine 1432 [running]:
	// runtime/debug.Stack(0xc424c4a370, 0xc421f7f750, 0x1)
	//   /usr/local/go/src/runtime/debug/stack.go:24 +0xa7
	//   ...
	// To know which trace belongs to which go routine we will append the
	// go routine number to every line of the stack trace.
	writer.Write(bytes.Replace(
		stack,
		[]byte{'\n'},
		append([]byte{'\n'}, goRoutineNumber...),
		// Don't replace the last '\n'
		newLines-1),
	)
}
