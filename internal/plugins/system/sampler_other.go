//go:build !windows

package system

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// Other systems read what Go's runtime knows: the processor count and the
// process's own memory use. A fuller sampler is what a port adds: the page
// publishes the readings it has and leaves the rest out rather than
// pretending to know a laptop's temperature.

type sampler struct {
	// logf records what the page cannot say: a reading this platform does not
	// have is one missing from a page that otherwise looks fine.
	logf func(format string, args ...any)
	// said keeps the problems already written, so that one that lasts is
	// written once.
	said map[string]bool
}

func (s *sampler) init() {}

func (s *sampler) sample(gpuKey string) metrics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = "本机"
	}
	return metrics{
		Hostname:       name,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Threads:        runtime.NumCPU(),
		MemTotal:       float64(stats.Sys),
		MemUsed:        float64(stats.HeapAlloc),
		MemUsedPercent: 100 * float64(stats.HeapAlloc) / float64(max(stats.Sys, 1)),
		BatteryMinutes: -1,
		Uptime:         time.Duration(time.Since(started)),
	}
}

var started = time.Now()
