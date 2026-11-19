//go:build linux

package neta

import (
	"syscall"
	"time"
)

// benchmarkCPUTime reads process user+system CPU time. Sampled only at the
// start and end of test-only workloads; no production hot-path probes.
func benchmarkCPUTime() (time.Duration, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	user := time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond
	system := time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond
	return user + system, true
}
