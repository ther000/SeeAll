//go:build windows

package system

import (
	"syscall"
	"time"
	"unsafe"

	"seeall/internal/core"
)

// The programs the machine is running, through toolhelp32 and psapi. The
// dashboard's bottom card wants the five holding the most memory, with the
// share of the processor each is using.

var psapi = syscall.NewLazyDLL("psapi.dll")

var (
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procGetProcessTimes          = kernel32.NewProc("GetProcessTimes")
	procGetProcessMemoryInfo     = psapi.NewProc("GetProcessMemoryInfo")
)

const (
	th32csSnapProcess              = 0x00000002
	invalidHandleValue             = ^uintptr(0)
	processQueryLimitedInformation = 0x1000
	processQueryInformation        = 0x0400
	processVMRead                  = 0x0010
)

// processEntry32 is PROCESSENTRY32W.
type processEntry32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// processSampler remembers how much processor time each program had used at
// the last sample: a process's CPU share is a rate, and a rate needs two reads
// of the same counter, a known time apart.
type processSampler struct {
	prev  map[uint32]uint64
	at    time.Time
	cores float64
}

// sample lists the programs running now with what each is costing. A program
// that cannot be opened is left out rather than listed at zero: Windows
// refuses to open protected and elevated processes, and a row saying "System
// 0 MB" would be a lie about a program that is very much using memory.
func (p *processSampler) sample(now time.Time) []core.Process {
	ids, names, ok := snapshotProcesses()
	if !ok || len(ids) == 0 {
		return nil
	}
	elapsed := now.Sub(p.at).Seconds()
	// A sample taken milliseconds after the last one turns the smallest jitter
	// in a process's clock into a huge percentage, so the share is only
	// measured over an interval worth dividing by: the Refresh button can be
	// pressed twice in a row.
	measurable := elapsed >= 0.1 && p.cores > 0
	out := make([]core.Process, 0, len(ids))
	next := make(map[uint32]uint64, len(ids))
	for i, pid := range ids {
		handle, err := openProcess(pid)
		if err != nil {
			continue
		}
		memory, hasMemory := processMemory(handle)
		cpuTime, hasCPU := processCPUTime(handle)
		closeHandle(handle)
		if !hasMemory || !hasCPU {
			continue
		}
		next[pid] = cpuTime
		proc := core.Process{Name: names[i], Mem: memory}
		if previous, seen := p.prev[pid]; seen && measurable {
			// GetProcessTimes counts in 100 nanosecond units, and the share
			// is of the whole machine: a program on one core of sixteen
			// busy ones is at 6%, not 100%.
			used := float64(cpuTime-previous) / 1e7
			proc.CPU = clampPercent(100 * used / elapsed / p.cores)
		}
		out = append(out, proc)
	}
	p.prev, p.at = next, now
	return out
}

// snapshotProcesses lists every process the machine has, with the name it
// runs under.
func snapshotProcesses() (ids []uint32, names []string, ok bool) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == 0 || snapshot == invalidHandleValue {
		return nil, nil, false
	}
	defer procCloseHandle.Call(snapshot)

	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	first, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if first == 0 {
		return nil, nil, false
	}
	for {
		// The idle pseudo-process is not a program and cannot be opened.
		if entry.ProcessID != 0 {
			ids = append(ids, entry.ProcessID)
			names = append(names, syscall.UTF16ToString(entry.ExeFile[:]))
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		next, _, _ := procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if next == 0 {
			break
		}
	}
	return ids, names, true
}

// openProcess opens a process with as little access as the two queries need:
// the limited access right reaches more of a machine's programs, and the fuller
// one is the fallback for systems that do not have it.
func openProcess(pid uint32) (uintptr, error) {
	handle, _, err := procOpenProcess.Call(processQueryLimitedInformation|processVMRead, 0, uintptr(pid))
	if handle != 0 {
		return handle, nil
	}
	handle, _, err = procOpenProcess.Call(processQueryInformation|processVMRead, 0, uintptr(pid))
	if handle != 0 {
		return handle, nil
	}
	return 0, err
}

// processMemory reads a process's working set: the memory it currently holds,
// which is the number Task Manager's Processes tab sorts by.
func processMemory(handle uintptr) (float64, bool) {
	var counters processMemoryCounters
	counters.Cb = uint32(unsafe.Sizeof(counters))
	if ok, _, _ := procGetProcessMemoryInfo.Call(handle,
		uintptr(unsafe.Pointer(&counters)), uintptr(counters.Cb)); ok == 0 {
		return 0, false
	}
	return float64(counters.WorkingSetSize), true
}

// processCPUTime reads how much processor time a process has used since it
// started, kernel and user together, in 100 nanosecond units.
func processCPUTime(handle uintptr) (uint64, bool) {
	var created, exited, kernel, user syscall.Filetime
	if ok, _, _ := procGetProcessTimes.Call(handle,
		uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); ok == 0 {
		return 0, false
	}
	return filetimeTicks(kernel) + filetimeTicks(user), true
}

func filetimeTicks(ft syscall.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

func closeHandle(handle uintptr) {
	procCloseHandle.Call(handle)
}
