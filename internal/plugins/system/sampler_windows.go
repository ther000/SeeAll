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

var advapi32 = syscall.NewLazyDLL("advapi32.dll")

var (
	procRegOpenKeyEx    = advapi32.NewProc("RegOpenKeyExW")
	procRegEnumKeyEx    = advapi32.NewProc("RegEnumKeyExW")
	procRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyLocalMachine = 0x80000002
	keyRead          = 0x00020019
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

	// perf holds the counters kernel32 does not have: the network's rates and
	// the graphics processor's load and memory. Its counters are collected
	// once on init so that the first sample has a rate to report.
	perf    *pdhQuery
	netRx   *pdhCounter
	netTx   *pdhCounter
	gpuUtil *pdhCounter
	vram    *pdhCounter
	// vramTotal is the adapter's memory, read from the registry once.
	vramTotal float64
}

func (s *sampler) init() {
	s.sampleCPU()
	s.initPerf()
}

// initPerf opens the performance counters and takes the first reading of each
// rate counter, which PDH needs before it can report a rate.
func (s *sampler) initPerf() {
	q := openQuery()
	if q == nil {
		return
	}
	// Every interface is summed, so the rates cover the machine rather than
	// one adapter the user may not be using.
	if q.add(`\Network Interface(*)\Bytes Received/sec`, true) {
		s.netRx = q.counters[len(q.counters)-1]
	}
	if q.add(`\Network Interface(*)\Bytes Sent/sec`, true) {
		s.netTx = q.counters[len(q.counters)-1]
	}
	if q.add(`\GPU Engine(*)\Utilization Percentage`, true) {
		s.gpuUtil = q.counters[len(q.counters)-1]
	}
	if q.add(`\GPU Local Adapter Memory(*)\Local Usage`, true) {
		s.vram = q.counters[len(q.counters)-1]
	}
	if len(q.counters) == 0 {
		q.close()
		return
	}
	s.perf = q
	s.vramTotal = adapterMemory()
	q.refresh()
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
	net, gpu, gpuOK := s.samplePerf()

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

	m.NetRxRate, m.NetTxRate = net[0], net[1]
	m.HasGPU = gpuOK
	m.GPUPercent, m.VRAMUsed, m.VRAMTotal = gpu[0], gpu[1], gpu[2]
	return m
}

// samplePerf reads the performance counters and turns them into the network
// rates and the graphics numbers.
func (s *sampler) samplePerf() (net [2]float64, gpu [3]float64, gpuOK bool) {
	if s.perf == nil || !s.perf.refresh() {
		return net, gpu, false
	}

	// Every interface is summed, adapters the user is not using included: the
	// panel answers "how fast is this machine talking", not "how fast is one
	// adapter". A disconnected adapter reports nothing.
	net[0] = sum(s.perf.instances(s.netRx))
	net[1] = sum(s.perf.instances(s.netTx))

	// The engines of a graphics processor each report their own utilization,
	// and Task Manager's number is their sum. Loopback and software engines
	// are not work the GPU did.
	engines := s.perf.instances(s.gpuUtil)
	if len(engines) > 0 {
		gpuOK = true
		gpu[0] = clampPercent(sum(engines))
	}
	// A virtual display adapter sits beside the real one and reports nothing,
	// so the adapter with the most memory in use is the one to show.
	used := maxOf(s.perf.instances(s.vram))
	if used > 0 && s.vramTotal > 0 {
		gpuOK = true
		gpu[1] = used
		gpu[2] = s.vramTotal
	}
	return net, gpu, gpuOK
}

func sum(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}

func maxOf(values []float64) float64 {
	var best float64
	for _, v := range values {
		if v > best {
			best = v
		}
	}
	return best
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "本机"
	}
	return name
}
