package system

import (
	"time"

	"seeall/internal/core"
)

// metrics is one sample of the machine.
type metrics struct {
	Hostname string
	Platform string

	CPUPercent float64
	// CPUName is the processor as the registry names it: "AMD Ryzen 7 6800H
	// with Radeon Graphics". The dashboard strips it down to "Ryzen 7 6800H".
	CPUName string
	// CPUClockMHz is the clock the processor is running at now, not the one
	// it was sold at: a laptop at idle sits well under its nominal speed.
	CPUClockMHz float64
	// Threads is how many logical processors the machine has.
	Threads int

	MemTotal       float64
	MemUsed        float64
	MemUsedPercent float64

	// HasTemp is false on a machine whose thermal zones cannot be read, and
	// then the page publishes no temperature at all rather than a zero.
	HasTemp bool
	TempC   float64
	// HasGPUTemp is false nearly everywhere: no plain Windows API reports a
	// graphics processor's temperature.
	HasGPUTemp bool
	GPUTempC   float64

	// The volume Windows itself is installed on, which is the one whose free
	// space the dashboard's progress bar is about.
	HasDisk   bool
	DiskDrive string
	DiskLabel string
	DiskTotal float64
	DiskFree  float64

	// NetKind names the link the machine is using: "Wi-Fi", "以太网", or ""
	// when no adapter is up.
	NetKind string

	HasBattery     bool
	BatteryPercent float64
	Charging       bool
	// BatteryMinutes is how long the battery lasts, or -1 when unknown.
	BatteryMinutes int
	// BatteryHealth is the charge the battery holds now as a share of the
	// capacity it was built with, 0..100. HasBatteryHealth is false when the
	// battery does not report both numbers.
	HasBatteryHealth bool
	BatteryHealth    float64

	// NetRxRate and NetTxRate are the bytes per second the machine is
	// receiving and sending, over every interface that is not a loopback.
	NetRxRate float64
	NetTxRate float64

	// HasGPU is false on a machine whose graphics cannot be counted, or on
	// the card the settings name when it is no longer there.
	HasGPU bool
	// GPUPercent is how busy the graphics processor is: the sum of its
	// engines' utilization, as Task Manager shows it.
	GPUPercent float64
	// GPUName is the card those numbers are about.
	GPUName string
	// GPUProblem says why a card the user chose reports nothing.
	GPUProblem string
	// VRAMUsed and VRAMTotal are the graphics memory in use and the one the
	// adapter has.
	VRAMUsed  float64
	VRAMTotal float64

	// OSName is the Windows version as the header pill writes it: "Windows 11
	// 24H2".
	OSName string

	// Processes are the programs running now, one entry per process and in no
	// particular order: the page puts a program's processes together, sorts
	// them by memory and keeps the few the card has room for.
	Processes []core.Process

	Uptime time.Duration
}

// log writes a line, if anyone is listening. A sampler reports what it could
// not read, which is otherwise invisible: the page simply has one card fewer.
func (s *sampler) log(format string, args ...any) {
	if s.logf != nil {
		s.logf(format, args...)
	}
}

// sayOnce reports a problem the first time it is seen and again only after it
// has cleared: a sample that fails every two seconds would otherwise turn one
// problem into a log full of copies of itself.
func (s *sampler) sayOnce(key, format string, args ...any) {
	if s.said == nil {
		s.said = map[string]bool{}
	}
	if s.said[key] {
		return
	}
	s.said[key] = true
	s.log(format, args...)
}

// sayClear forgets a problem that is over, so that it is worth a line again if
// it comes back.
func (s *sampler) sayClear(keys ...string) {
	for _, key := range keys {
		delete(s.said, key)
	}
}
