package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dataDir calls Data and fails the test when it reports an error.
func dataDir(t *testing.T) string {
	t.Helper()
	dir, err := Data()
	if err != nil {
		t.Fatalf("Data(): %v", err)
	}
	return dir
}

// fakeExec puts the executable in dir for the rest of the test.
func fakeExec(t *testing.T, dir string) {
	t.Helper()
	fakeExecIn(t, filepath.Join(dir, "SeeAll.exe"))
}

// fakeExecIn puts the executable exactly where path says, which is how a test
// sets up a folder that cannot be written.
func fakeExecIn(t *testing.T, path string) {
	t.Helper()
	previous := executable
	executable = func() (string, error) { return path, nil }
	resetForTest()
	t.Cleanup(func() {
		executable = previous
		resetForTest()
	})
}

// TestDataIsBesideTheExecutable is the whole point of the package: the folder
// the app writes to is the folder the executable is in, so the app is one
// directory that can be moved or copied as a unit.
func TestDataIsBesideTheExecutable(t *testing.T) {
	dir := t.TempDir()
	fakeExec(t, dir)

	if got := dataDir(t); got != dir {
		t.Errorf("Data() = %q, want the executable's folder %q", got, dir)
	}
}

// TestSettingsAndLogAreInThatFolder checks the two files land where Data says,
// and that the log's own folder is made rather than assumed.
func TestSettingsAndLogAreInThatFolder(t *testing.T) {
	dir := t.TempDir()
	fakeExec(t, dir)

	settings, err := Settings()
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Dir(settings); got != dir {
		t.Errorf("settings live in %q, want %q", got, dir)
	}
	if base := filepath.Base(settings); base != "seeall.json" {
		t.Errorf("settings file is %q", base)
	}

	logPath, err := Log()
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Dir(logPath); got != filepath.Join(dir, "logs") {
		t.Errorf("the log is in %q, want %q", got, filepath.Join(dir, "logs"))
	}
	if info, err := os.Stat(filepath.Dir(logPath)); err != nil || !info.IsDir() {
		t.Errorf("the log's folder was not made: %v", err)
	}
}

// TestDataIsWorkedOutOnce keeps a caller from asking the filesystem about the
// executable on every call.
func TestDataIsWorkedOutOnce(t *testing.T) {
	dir := t.TempDir()
	fakeExec(t, dir)

	first := dataDir(t)
	// Move the executable out from under the cached answer without resetting
	// it: a folder worked out once does not follow.
	executable = func() (string, error) {
		return filepath.Join(t.TempDir(), "SeeAll.exe"), nil
	}
	if second := dataDir(t); second != first {
		t.Errorf("Data() changed after a second call: %q then %q", first, second)
	}
}

// TestAFolderThatCannotBeWrittenFallsBack is the one case a
// beside-the-executable layout has to survive: a copy run from somewhere
// read-only. The app still starts, writing to the per-user folder.
func TestAFolderThatCannotBeWrittenFallsBack(t *testing.T) {
	parent := t.TempDir()
	// A file where a folder has to be, and the executable "inside" it: the
	// folder it sits in cannot be created, so MkdirAll must fail.
	blocker := filepath.Join(parent, "blocker")
	if err := os.WriteFile(blocker, []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeExecIn(t, filepath.Join(blocker, "SeeAll", "SeeAll.exe"))

	dir, err := Data()
	if err != nil {
		t.Fatalf("Data() failed instead of falling back: %v", err)
	}
	if strings.HasPrefix(dir, blocker) {
		t.Fatalf("Data() used a folder under a plain file: %q", dir)
	}
	userConfig, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("no per-user folder on this machine: %v", err)
	}
	if !strings.HasPrefix(dir, userConfig) {
		t.Errorf("the fallback is %q, want something under %q", dir, userConfig)
	}
}
