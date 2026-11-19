//go:build !linux

package neta

import "time"

// CPU/op stays unavailable (and INCONCLUSIVE for qualification) on platforms
// without this fixture's Linux process-time measurement.
func benchmarkCPUTime() (time.Duration, bool) {
	return 0, false
}
