//go:build !windows

package molecli

import (
	"os"
	"os/exec"
	"syscall"
)

// sysProcAttr has nothing to set on other systems.
//
// Unix-only fields such as Setpgid are deliberately left out, so the package
// also builds on js/wasm and plan9; SeeAll only ever runs on Windows, and this
// file is here so that ./... builds elsewhere.
func sysProcAttr() *syscall.SysProcAttr { return nil }

// locate looks for mo in PATH on other systems (the macOS edition is a script).
func locate() (string, bool) {
	path, err := exec.LookPath("mo")
	return path, err == nil
}

// fileExists says whether the path is a file that exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// commandFor runs mo directly on other systems.
func commandFor(launcher string, args []string) (string, []string) {
	return launcher, args
}

// killTree can only kill the direct child on other systems: a process tree
// needs platform support, and this file does not provide it.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
