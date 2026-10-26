//go:build windows

package agent

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// listProcesses reads the process table with the Toolhelp snapshot API. It
// runs on a goroutine of the plugin's own, never on the UI thread.
func listProcesses() ([]Process, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("快照进程失败：%w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}

	now := time.Now()
	var out []Process
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if p, ok := describeProcess(entry.ProcessID, name, now); ok {
			out = append(out, p)
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return out, nil
}

// describeProcess reads the command line and CPU time of one process, and
// drops the ones that exited while the snapshot was open.
func describeProcess(pid uint32, name string, now time.Time) (Process, bool) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return Process{}, false
	}
	defer windows.CloseHandle(handle)

	command := name
	if line, err := commandLine(handle); err == nil && line != "" {
		command = line
	}
	p := Process{PID: int(pid), Command: command}

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err == nil {
		k := filetimeSeconds(kernel)
		u := filetimeSeconds(user)
		p.CPUSeconds = k + u
		if created := filetimeTime(creation); !created.IsZero() {
			p.Started = created
		}
	}
	if p.Started.IsZero() {
		p.Started = now
	}
	return p, true
}

func filetimeSeconds(ft windows.Filetime) float64 {
	return float64(uint64(ft.HighDateTime)<<32|uint64(ft.LowDateTime)) / 1e7
}

func filetimeTime(ft windows.Filetime) time.Time {
	ticks := uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
	if ticks == 0 {
		return time.Time{}
	}
	// Windows counts 100ns ticks from 1601; Unix from 1970.
	const unixEpochOffset = int64(116444736000000000)
	return time.Unix(0, int64(ticks/10)-unixEpochOffset*10)
}

// commandLine reads a process's command line with NtQueryInformationProcess
// and the PEB, which needs no elevation for the user's own processes.
func commandLine(handle windows.Handle) (string, error) {
	var info pebInfo
	status, _, callErr := ntQueryInformationProcess.Call(
		uintptr(handle), uintptr(processCommandLineInformation),
		uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Sizeof(info)), 0,
	)
	if status != 0 || info.Length == 0 || info.Buffer == 0 {
		return "", fmt.Errorf("读不到命令行（%v）", callErr)
	}
	buf := make([]uint16, info.Length/2)
	if err := windows.ReadProcessMemory(windows.CurrentProcess(), uintptr(info.Buffer),
		(*byte)(unsafe.Pointer(&buf[0])), uintptr(info.Length), nil); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}

// pebInfo mirrors PROCESS_COMMAND_LINE_INFORMATION.
type pebInfo struct {
	Length uint16
	_      uint16
	Buffer uint64
}

const processCommandLineInformation = 60

var ntdll = windows.NewLazySystemDLL("ntdll.dll")
var ntQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
