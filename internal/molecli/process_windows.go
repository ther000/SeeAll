//go:build windows

package molecli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

const (
	// createNoWindow gives the child no console of its own. SeeAll is built
	// with -H=windowsgui, so nothing hands the child a console, and without
	// this flag a black window flashes.
	createNoWindow = 0x08000000
	// createNewProcessGroup puts powershell/cmd in a group of its own, so a
	// cancel can take the group down at once.
	createNewProcessGroup = 0x00000200
)

// sysProcAttr is the start-up attribute every child gets: no window, a group of
// its own.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow | createNewProcessGroup,
	}
}

// locate finds a file that can start mo. It only looks, never runs.
//
// The order is mo / mo.cmd in PATH, the install folder, then the scoop shims.
// PATH comes first because the user may have installed it elsewhere; the
// install folder is where the official script puts it.
func locate() (string, bool) {
	for _, name := range []string{"mo", "mo.cmd"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		path := filepath.Join(local, "Programs", "mole-cli", "mo.cmd")
		if fileExists(path) {
			return path, true
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, "scoop", "shims", "mo.cmd")
		if fileExists(path) {
			return path, true
		}
	}
	return "", false
}

// fileExists says whether the path is a file that exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// commandFor expands mo's launcher into the command that really runs.
//
// mo.cmd is only a cmd shell that calls powershell in turn. With mole.ps1 next
// to it, powershell runs the script directly, which saves the cmd.exe layer and
// the quoting that layer needs. Every argument is passed through untouched.
// Without the .ps1 it falls back to cmd.exe /c.
func commandFor(launcher string, args []string) (string, []string) {
	script := filepath.Join(filepath.Dir(launcher), "mole.ps1")
	if fileExists(script) {
		argv := []string{
			"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-File", script,
		}
		return "powershell.exe", append(argv, args...)
	}
	return "cmd.exe", append([]string{"/c", launcher}, args...)
}

// killTree kills the whole tree cmd started.
//
// powershell starts children of its own (analyze/status call the exe under bin),
// and killing only the parent leaves them behind as orphans. taskkill /T is the
// dependable way on Windows; when policy blocks it, it falls back to a single
// Kill. taskkill gets a hidden window too, or a black box flashes on cancel.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = sysProcAttr()
	if err := kill.Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
