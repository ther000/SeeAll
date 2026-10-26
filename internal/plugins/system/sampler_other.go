//go:build !windows

package system

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// Other systems read what Go's runtime knows: the processor count and the
// process's own memory use. A fuller sampler is what a port adds.

type sampler struct{}

func (s *sampler) init() {}

func (s *sampler) sample(disk string) metrics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	name, err := os.Hostname()
	if err != nil || name == "" {
		name = "本机"
	}
	return metrics{
		Hostname:       name,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		MemTotal:       float64(stats.Sys),
		MemUsed:        float64(stats.HeapAlloc),
		MemUsedPercent: 100 * float64(stats.HeapAlloc) / float64(max(stats.Sys, 1)),
		BatteryMinutes: -1,
		Uptime:         time.Duration(time.Since(started)),
	}
}

var started = time.Now()
