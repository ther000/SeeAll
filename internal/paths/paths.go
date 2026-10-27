// Package paths says where SeeAll keeps the files it writes.
//
// Everything goes beside the executable: seeall.json and logs/ land in the same
// folder the user unzipped, so the whole app is one directory that can be
// copied, backed up or thrown away together. The installer puts it in
// %LOCALAPPDATA%\Programs\SeeAll, a folder the user owns, so writing there
// works; a build run from a checkout writes into that checkout.
//
// The fallback matters for the one case that breaks: an executable run from a
// read-only place, such as a network share. There the app falls back to the
// per-user data folder rather than failing to start.
package paths

import (
	"os"
	"path/filepath"
)

// dir is where the app writes, worked out once.
var dir string

// Data returns the folder the app keeps its settings and log in, creating it if
// it is not there.
func Data() (string, error) {
	if dir != "" {
		return dir, nil
	}
	next := besideExecutable()
	if err := os.MkdirAll(next, 0o755); err != nil {
		// The executable's folder is not writable. A per-user folder always is.
		if fallback, err := os.UserConfigDir(); err == nil {
			next = filepath.Join(fallback, "SeeAll")
			if err := os.MkdirAll(next, 0o755); err == nil {
				dir = next
				return dir, nil
			}
		}
		return "", err
	}
	dir = next
	return dir, nil
}

// besideExecutable is the folder holding the running executable, which is where
// a portable app keeps its things.
func besideExecutable() string {
	exe, err := executable()
	if err != nil {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// Settings returns the settings file's path.
func Settings() (string, error) {
	d, err := Data()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "seeall.json"), nil
}

// Log returns the log file's path, making its folder.
func Log() (string, error) {
	d, err := Data()
	if err != nil {
		return "", err
	}
	logs := filepath.Join(d, "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(logs, "seeall.log"), nil
}

// executable is os.Executable, replaceable by tests that need a folder the
// machine cannot write.
var executable = os.Executable

// resetForTest forgets the folder worked out so far, so that a test can make
// the package work it out again.
func resetForTest() { dir = "" }
