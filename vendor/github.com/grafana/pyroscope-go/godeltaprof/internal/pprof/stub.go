//go:build go1.16 && !go1.22
// +build go1.16,!go1.22

package pprof

// unsafe is required for go:linkname
import _ "unsafe"

//go:linkname runtime_expandFinalInlineFrame runtime/pprof.runtime_expandFinalInlineFrame
func runtime_expandFinalInlineFrame(stk []uintptr) []uintptr

//go:linkname runtime_cyclesPerSecond runtime/pprof.runtime_cyclesPerSecond
func runtime_cyclesPerSecond() int64
