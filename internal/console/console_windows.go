//go:build windows

// Package console keeps SeeAll from leaving a terminal window behind.
//
// A Go program builds as a console application unless the linker is given
// -H=windowsgui, which mygo build sets and a bare "go build" does not. So a
// binary built the quick way opens a black window next to the tray icon.
// Hiding it here fixes that for any build, while leaving a console alone when
// it is not ours: run from cmd, PowerShell or "go run", the window belongs to
// the shell the developer is working in.
package console

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	user32                    = syscall.NewLazyDLL("user32.dll")
	procShowWindow            = user32.NewProc("ShowWindow")
)

const swHide = 0

// HideIfOwned hides the console window when this process is the only one
// using it, which is what a double click on a console-subsystem build does.
// It does nothing when the console is shared with a shell.
func HideIfOwned() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}

	// GetConsoleProcessList fills the buffer with the processes attached to
	// the console and returns how many there are, or 0 when there is none.
	var pids [8]uint32
	count, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)),
	)
	if count != 1 {
		return
	}
	procShowWindow.Call(hwnd, swHide)
}
