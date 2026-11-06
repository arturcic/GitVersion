//go:build !linux && !darwin

package parity

import "os"

// Zero means unavailable, not zero memory consumption.
func peakRSS(state *os.ProcessState) int64 { return 0 }
