//go:build windows

package system

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// Windows sampling through kernel32, with no cgo: GetSystemTimes for the
// processor, GlobalMemoryStatusEx for memory, GetDiskFreeSpaceExW for a
// drive, GetSystemPowerStatus for the battery and GetTickCount64 for the
// uptime.

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

type sampler struct {
	prevIdle  uint64
	prevTotal uint64
	cpu       float64
	havePrev  bool
}

func (s *sampler) init() {
	s.sampleCPU()
}

// sampleCPU reads the processor's total and idle time since boot, and turns
// the difference from the last call into a load percentage.
func (s *sampler) sampleCPU() {
	var idle, kernel, user syscall.Filetime
	if ok, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	); ok == 0 {
		return
	}
	idleT := uint64(idle.HighDateTime)<<32 | uint64(idle.LowDateTime)
	kernelT := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	userT := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	// Kernel time includes idle time.
	total := kernelT + userT
	if !s.havePrev {
		s.prevIdle, s.prevTotal, s.havePrev = idleT, total, true
		return
	}
	dIdle := idleT - s.prevIdle
	dTotal := total - s.prevTotal
	s.prevIdle, s.prevTotal = idleT, total
	if dTotal == 0 {
		return
	}
	s.cpu = float64(dTotal-dIdle) * 100 / float64(dTotal)
	if s.cpu < 0 {
		s.cpu = 0
	} else if s.cpu > 100 {
		s.cpu = 100
	}
}

func (s *sampler) sample(disk string) metrics {
	s.sampleCPU()

	m := metrics{
		Hostname:       hostname(),
		Platform:       fmt.Sprintf("Windows · %s/%s", runtime.GOOS, runtime.GOARCH),
		CPUPercent:     s.cpu,
		BatteryMinutes: -1,
	}

	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))
	if ok, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); ok != 0 {
		m.MemTotal = float64(mem.TotalPhys)
		m.MemUsed = float64(mem.TotalPhys - mem.AvailPhys)
		m.MemUsedPercent = float64(mem.MemoryLoad)
	}

	if disk != "" {
		path, err := syscall.UTF16PtrFromString(disk + "\\")
		if err == nil {
			var freeToCaller, total, free uint64
			ok, _, _ := procGetDiskFreeSpaceExW.Call(
				uintptr(unsafe.Pointer(path)),
				uintptr(unsafe.Pointer(&freeToCaller)),
				uintptr(unsafe.Pointer(&total)),
				uintptr(unsafe.Pointer(&free)),
			)
			if ok != 0 {
				m.DiskTotal = float64(total)
				m.DiskFree = float64(free)
			}
		}
	}

	var power systemPowerStatus
	if ok, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&power))); ok != 0 {
		// 255 means the machine has no battery, as a desktop.
		if power.BatteryLifePercent != 255 {
			m.HasBattery = true
			m.BatteryPercent = float64(power.BatteryLifePercent)
			m.Charging = power.ACLineStatus == 1
			if power.BatteryLifeTime != 0xFFFFFFFF {
				m.BatteryMinutes = int(power.BatteryLifeTime / 60)
			}
		}
	}

	if ticks, _, _ := procGetTickCount64.Call(); ticks != 0 {
		m.Uptime = time.Duration(ticks) * time.Millisecond
	}
	return m
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "本机"
	}
	return name
}
