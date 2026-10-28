//go:build windows

package system

import (
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"seeall/internal/core"
)

// TestTheMachineSamplesItself checks the Windows sampler against the machine it
// runs on. The dashboard's header and cards need a processor name, a Windows
// release, a volume with room and a handful of programs; none of that can be
// faked, so a machine that reports nothing fails here instead of drawing a page
// of dashes.
func TestTheMachineSamplesItself(t *testing.T) {
	s := &sampler{}
	s.logf = t.Logf
	s.init()
	first := s.sample(gpuAuto)
	// The second sample is the first one with a processor delta and a process
	// delta behind it, which is what the page really draws.
	start := time.Now()
	m := s.sample(gpuAuto)
	elapsed := time.Since(start)

	t.Logf("cpu=%q clock=%.0fMHz threads=%d uptime=%v", m.CPUName, m.CPUClockMHz, m.Threads, core.FormatSpan(m.Uptime))
	t.Logf("mem=%.2fGB/%.2fGB (%.0f%%) os=%q platform=%q", m.MemUsed/(1<<30), m.MemTotal/(1<<30), m.MemUsedPercent, m.OSName, m.Platform)
	t.Logf("disk=%v %q label=%q free=%.1fGB total=%.1fGB", m.HasDisk, m.DiskDrive, m.DiskLabel, m.DiskFree/(1<<30), m.DiskTotal/(1<<30))
	t.Logf("temp=%v %.1f°C gpuTemp=%v net=%q", m.HasTemp, m.TempC, m.HasGPUTemp, m.NetKind)
	t.Logf("battery=%v %.0f%% charging=%v health=%v %.0f%%", m.HasBattery, m.BatteryPercent, m.Charging, m.HasBatteryHealth, m.BatteryHealth)
	t.Logf("cpu load %.1f%% (first sample %.1f%%), %d processes, sample took %v", m.CPUPercent, first.CPUPercent, len(m.Processes), elapsed)
	if len(m.Processes) > 0 {
		top := topProcesses(m.Processes, 3)
		for _, p := range top {
			t.Logf("  %s %.0f MB %.1f%%", p.Name, p.Mem/(1<<20), p.CPU)
		}
	}

	if m.Threads != runtime.NumCPU() {
		t.Errorf("%d threads, want the %d the runtime counts", m.Threads, runtime.NumCPU())
	}
	if m.CPUName == "" {
		t.Error("the registry did not name the processor, so the header has no model")
	}
	if m.MemTotal <= 0 || m.MemUsedPercent < 0 || m.MemUsedPercent > 100 {
		t.Errorf("memory reads %v of %v (%v%%)", m.MemUsed, m.MemTotal, m.MemUsedPercent)
	}
	if !strings.HasPrefix(m.OSName, "Windows") {
		t.Errorf("the Windows release reads %q", m.OSName)
	}
	if !m.HasDisk {
		t.Error("the system volume could not be read, so the free-space card is missing")
	} else {
		if m.DiskDrive == "" || !strings.HasSuffix(m.DiskDrive, ":") {
			t.Errorf("the volume is called %q", m.DiskDrive)
		}
		if m.DiskFree <= 0 || m.DiskFree > m.DiskTotal {
			t.Errorf("free space is %v of %v bytes", m.DiskFree, m.DiskTotal)
		}
	}
	if m.CPUPercent < 0 || m.CPUPercent > 100 {
		t.Errorf("the processor reads %v%%", m.CPUPercent)
	}
	if m.Uptime <= 0 {
		t.Error("the machine has been up for no time at all")
	}

	// The process list is the dashboard's bottom card. A machine with none is
	// not a machine, and the rows must be real: a zero-memory row would be a
	// program the sampler could not open. A few protected processes report a
	// working set of zero — they are not rows the card can show, and the
	// sampler leaves them where they fall, at the bottom of the list.
	if len(m.Processes) == 0 {
		t.Fatal("no processes were listed")
	}
	blank := 0
	for _, p := range m.Processes {
		if p.Name == "" {
			t.Error("a process has no name")
		}
		if p.Mem <= 0 {
			blank++
		}
		if p.CPU < 0 || p.CPU > 100 {
			t.Errorf("%s reports %v%% of the machine", p.Name, p.CPU)
		}
	}
	if blank == len(m.Processes) {
		t.Error("no process on the machine reported any memory")
	}
	if blank > 0 {
		t.Logf("%d of %d processes report an unreadable working set", blank, len(m.Processes))
	}
	// The list is the heaviest few, and it is sorted: two samples of the same
	// machine must not disagree about which program is biggest.
	top := topProcesses(m.Processes, processCount)
	if len(top) > processCount || len(top) > len(m.Processes) {
		t.Errorf("%d rows out of %d processes", len(top), len(m.Processes))
	}
	if !sort.SliceIsSorted(top, func(i, j int) bool { return top[i].Mem > top[j].Mem }) {
		t.Errorf("the top processes are out of order: %v", top)
	}
	for _, p := range top {
		if p.Mem <= 0 {
			t.Errorf("the card's row for %s claims no memory", p.Name)
		}
	}
}

// TestTheInventoryIsThere checks what is read once when the program starts: the
// nominal clock the current one is reported against, and the volume Windows is
// installed on.
func TestTheInventoryIsThere(t *testing.T) {
	inv := readInventory()
	if inv.BaseMHz <= 0 {
		t.Errorf("the registry did not report a nominal clock: %+v", inv)
	}
	if !strings.HasSuffix(inv.Drive, ":") {
		t.Errorf("the system drive is %q", inv.Drive)
	}
	if inv.OSName == "" {
		t.Error("the Windows release could not be read")
	}
}

// TestTheTemperatureIsEitherThereOrNot checks the thermal zones: this machine
// may not publish them, but a value that is there has to be a temperature a
// computer could be at rather than a firmware placeholder in kelvin.
func TestTheTemperatureIsEitherThereOrNot(t *testing.T) {
	s := &sampler{}
	s.logf = t.Logf
	s.init()
	m := s.sample(gpuAuto)
	if !m.HasTemp {
		t.Skip("this machine publishes no thermal zone temperature")
	}
	if m.TempC < 0 || m.TempC > 125 {
		t.Errorf("the thermal zone reads %v°C", m.TempC)
	}
}
