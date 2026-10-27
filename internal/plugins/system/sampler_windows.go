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

// Windows sampling through kernel32 and pdh, with no cgo: GetSystemTimes for
// the processor, GlobalMemoryStatusEx for memory, GetSystemPowerStatus for the
// battery, GetTickCount64 for the uptime, and the performance counters for the
// network's rates and the graphics processor's load and memory.

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
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

// missingCard is what the page says when the card the settings name is not in
// the machine any more.
const missingCard = "找不到这张显卡"

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

	// gpuKey is the card the settings ask for and gpu the card that turned out
	// to be it. The counters are read per adapter, so every sample has to know
	// which one the page is about; the answer is kept until the setting
	// changes, because enumerating the cards costs more than a sample.
	gpuKey   string
	gpu      gpuAdapter
	gpuFound bool
	resolved bool
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
	if q.add(gpuEnginePath, true) {
		s.gpuUtil = q.counters[len(q.counters)-1]
	}
	if q.add(gpuMemoryPath, true) {
		s.vram = q.counters[len(q.counters)-1]
	}
	if len(q.counters) == 0 {
		q.close()
		return
	}
	s.perf = q
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

// sample reads the machine. gpuKey names the card the page is about: a key
// from the settings page, or gpuAuto.
func (s *sampler) sample(gpuKey string) metrics {
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

	// The card is resolved whether or not the counters opened: a choice the
	// machine cannot honour is worth saying even when nothing can be counted.
	card, found := s.adapter(gpuKey)
	if s.perf != nil && s.perf.refresh() {
		net := s.sampleNet()
		m.NetRxRate, m.NetTxRate = net[0], net[1]
		s.sampleGPU(&m, card, found, gpuKey)
	} else if !found && gpuKey != "" && gpuKey != gpuAuto {
		m.GPUProblem = missingCard
	}
	return m
}

// sampleNet reads the network's byte rates.
func (s *sampler) sampleNet() [2]float64 {
	var net [2]float64
	// Every interface is summed, adapters the user is not using included: the
	// panel answers "how fast is this machine talking", not "how fast is one
	// adapter". A disconnected adapter reports nothing.
	for _, in := range s.perf.instanceValues(s.netRx) {
		net[0] += in.Value
	}
	for _, in := range s.perf.instanceValues(s.netTx) {
		net[1] += in.Value
	}
	return net
}

// sampleGPU fills the graphics numbers for the card the settings name.
func (s *sampler) sampleGPU(m *metrics, card gpuAdapter, found bool, gpuKey string) {
	if !found && gpuKey != "" && gpuKey != gpuAuto {
		// The user picked a card this machine no longer has. Saying so is
		// better than quietly counting another one.
		m.GPUProblem = missingCard
		return
	}
	// The name the settings row uses, so that the page and the row it comes
	// from call the card the same thing: the driver's full string has the
	// vendor word and the trademark marks in it.
	m.GPUName = shortGPUName(card.Name)
	if m.GPUName == "" {
		// No card could be enumerated: the counters are still there, and the
		// machine's graphics are worth showing without naming them.
		m.GPUName = "显卡"
	}

	// The engines of a graphics processor each report their own utilization,
	// and Task Manager's number is their sum. Another adapter's engines are
	// another card's work.
	var busy float64
	for _, in := range s.perf.instanceValues(s.gpuUtil) {
		if card.isOurs(in.Name) {
			busy += in.Value
		}
	}
	m.GPUPercent = clampPercent(busy)

	// A virtual display adapter sits beside the real one and reports nothing,
	// so the card's own memory is read rather than the largest of them all.
	for _, in := range s.perf.instanceValues(s.vram) {
		if card.isOurs(in.Name) && in.Value > m.VRAMUsed {
			m.VRAMUsed = in.Value
		}
	}
	m.VRAMTotal = card.Memory
	m.HasGPU = true
}

// adapter resolves the card the page reports on: the one the settings name, or
// the largest when they ask for automatic. A machine whose cards cannot be
// enumerated resolves to none, and the counters are then read for the machine
// as a whole.
func (s *sampler) adapter(key string) (gpuAdapter, bool) {
	if key == "" {
		key = gpuAuto
	}
	if s.resolved && s.gpuKey == key {
		return s.gpu, s.gpuFound
	}
	cards := graphicsAdapters()
	var chosen gpuAdapter
	found := false
	if key != gpuAuto {
		chosen, found = findAdapter(cards, key)
		if !found {
			// The list was read once; a card that was plugged in or switched
			// back on since is worth finding, and finding out it is really
			// gone is worth a second look too.
			chosen, found = findAdapter(refreshAdapters(), key)
		}
	}
	if !found && key == gpuAuto {
		for _, c := range cards {
			if !found || c.Memory > chosen.Memory {
				chosen, found = c, true
			}
		}
	}
	s.gpuKey, s.gpu, s.gpuFound, s.resolved = key, chosen, found, true
	return chosen, found
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

// findAdapter looks a card up by the key the settings hold.
func findAdapter(cards []gpuAdapter, key string) (gpuAdapter, bool) {
	for _, c := range cards {
		if c.Key == key {
			return c, true
		}
	}
	return gpuAdapter{}, false
}
