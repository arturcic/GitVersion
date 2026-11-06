//go:build linux || darwin

package parity

import (
	"os"
	"runtime"
	"syscall"
)

func peakRSS(state *os.ProcessState) int64 {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	bytes := usage.Maxrss
	if runtime.GOOS == "linux" {
		bytes *= 1024
	}
	return bytes
}
