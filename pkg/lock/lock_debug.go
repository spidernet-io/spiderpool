// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

//go:build lockdebug
// +build lockdebug

package lock

import (
	"bytes"
	"fmt"
	deadlock "github.com/sasha-s/go-deadlock"
	"io"
	"os"
	"runtime/debug"
	"time"
)

const (
	// selfishThresholdSec is the number of seconds that should be used when
	// detecting if a lock was held for more than the specified time.
	selfishThresholdSec = 0.1

	// Waiting for a lock for longer than DeadlockTimeout is considered a deadlock.
	// Ignored is DeadlockTimeout <= 0.
	deadLockTimeout = 3 * time.Second
)

func init() {
	deadlock.Opts.DeadlockTimeout = deadLockTimeout
	deadlock.Opts.Disable = false
	deadlock.Opts.DisableLockOrderDetection = false
	deadlock.Opts.PrintAllCurrentGoroutines = true
	deadlock.Opts.LogBuf = os.Stderr
	deadlock.Opts.OnPotentialDeadlock = func() {
		os.Exit(2)
	}
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
	if sec := time.Since(i.t).Seconds(); sec >= selfishThresholdSec {
		printStackTo(sec, debug.Stack(), os.Stderr)
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
	if sec := time.Since(i.Time).Seconds(); sec >= selfishThresholdSec {
		printStackTo(sec, debug.Stack(), os.Stderr)
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

	fmt.Printf("goroutine=%v : Goroutine took lock %v seconds for more than %.2f seconds\n",
		string(goRoutineNumber[len("goroutine"):len(goRoutineNumber)-1]), sec, selfishThresholdSec)

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
