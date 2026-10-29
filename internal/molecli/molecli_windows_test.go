//go:build windows

package molecli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// fakeCLI lays out a fake mole in a temporary folder: a mo.cmd plus a mole.ps1,
// the same shape as a real install. mo.cmd deliberately answers only with
// ROUTE=cmd, so one run says which route was taken.
//
// Passing every argument through is the point of this file: mo's command line
// is all positional, and a dropped --limit or --dry-run makes it useless.
func fakeCLI(t *testing.T, script string) *Client {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mo.cmd"),
		[]byte("@echo off\r\necho ROUTE=cmd ARGS=%*\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		if err := os.WriteFile(filepath.Join(dir, "mole.ps1"), []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := New()
	c.look = func() (string, bool) { return filepath.Join(dir, "mo.cmd"), true }
	return c
}

// TestRunsTheScriptNextToTheLauncher pins "skip the cmd.exe layer when there is
// a mole.ps1".
func TestRunsTheScriptNextToTheLauncher(t *testing.T) {
	c := fakeCLI(t, "Write-Output (\"ROUTE=ps1 ARGS=\" + ($args -join ' '))\n")
	out, err := c.run(context.Background(), "clean", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, "ROUTE=ps1 ARGS=clean --dry-run") {
		t.Errorf("ran as %q, want the mole.ps1 route with clean --dry-run", got)
	}
}

// TestAllArgumentsReachTheCLI pins argument fidelity: spaces, equals signs and
// backslashes must all survive the trip.
func TestAllArgumentsReachTheCLI(t *testing.T) {
	c := fakeCLI(t, "Write-Output ($args -join '|')\n")
	out, err := c.run(context.Background(), "history", "--json", "--limit", "200")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(out)), "history|--json|--limit|200"; got != want {
		t.Errorf("the arguments arrived as %q, want %q", got, want)
	}
}

// TestFallsBackToCmdWhenThereIsNoScript pins the fallback: with only mo.cmd the
// cmd.exe /c route runs, and the arguments still go through.
func TestFallsBackToCmdWhenThereIsNoScript(t *testing.T) {
	c := fakeCLI(t, "")
	out, err := c.run(context.Background(), "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, "ROUTE=cmd ARGS=status --json") {
		t.Errorf("ran as %q, want the cmd.exe route with status --json", got)
	}
}

// TestFailureCarriesTheOutputTail pins the failure path: on a non-zero exit the
// error has to carry the last thing mo said (with the ANSI stripped), not just
// "exit status 3".
func TestFailureCarriesTheOutputTail(t *testing.T) {
	c := fakeCLI(t, `Write-Output "$([char]27)[31mboom: cache locked by another process$([char]27)[0m"
Write-Output "Try again later"
exit 3
`)
	_, err := c.PreviewClean(context.Background())
	if err == nil {
		t.Fatal("the fake mo exited 3 and this reported no error")
	}
	text := err.Error()
	for _, want := range []string{"boom: cache locked by another process", "Try again later", "exit status 3"} {
		if !strings.Contains(text, want) {
			t.Errorf("the error does not carry %q: %v", want, err)
		}
	}
	if strings.Contains(text, "\x1b") {
		t.Errorf("the error still carries ANSI: %q", text)
	}
}

// TestUnreadableOutputIsAnError pins that "output I cannot read" and "the
// command failed" are two different things: exit code 0 with something else on
// stdout must be an error, not a cleanup of zero bytes.
func TestUnreadableOutputIsAnError(t *testing.T) {
	c := fakeCLI(t, "Write-Output 'hello from somewhere else'\n")
	_, err := c.PreviewClean(context.Background())
	if err == nil {
		t.Fatal("output that cannot be read must be an error")
	}
	if !strings.Contains(err.Error(), "mo clean") {
		t.Errorf("the error does not say which command failed: %v", err)
	}
}

// TestCancelKillsTheWholeTree pins cancellation: once ctx is done, powershell
// and the children it started have to go too. Killing only the parent leaves
// orphans behind, which is exactly what this package exists to prevent.
func TestCancelKillsTheWholeTree(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "pids.txt")
	script := fmt.Sprintf(`$log = %q
Set-Content -Path $log -Value $PID
$child = Start-Process -FilePath "powershell.exe" -ArgumentList @('-NoProfile','-Command','Start-Sleep -Seconds 60') -PassThru -WindowStyle Hidden
Add-Content -Path $log -Value $child.Id
Start-Sleep -Seconds 60
`, logPath)
	c := fakeCLI(t, script)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.run(ctx, "analyze", "--json")
		done <- err
	}()

	pids := waitForPIDs(t, logPath, 2, 20*time.Second)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("after the cancel the error is %v, want context.Canceled", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("still running 15s after the cancel; the tree probably survived")
	}

	for _, pid := range pids {
		if !waitForExit(pid, 10*time.Second) {
			t.Errorf("process %d is still alive; the cancel did not kill the tree", pid)
		}
	}
}

// waitForPIDs waits until the file holds want PIDs. The fake mo writes them
// slowly, so sleeping a fixed while and hoping would be flaky.
func waitForPIDs(t *testing.T, path string, want int, timeout time.Duration) []int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			lines := strings.Fields(string(data))
			if len(lines) >= want {
				pids := make([]int, 0, len(lines))
				for _, line := range lines {
					pid, err := strconv.Atoi(line)
					if err != nil {
						t.Fatalf("the PID file holds %q: %v", line, err)
					}
					pids = append(pids, pid)
				}
				return pids
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("waited %v and the fake mo never wrote %d PIDs", timeout, want)
	return nil
}

// waitForExit waits until a process is gone. taskkill returning does not mean
// the process has exited yet, so it has to be polled.
func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !processAlive(pid)
}

// processAlive says whether that PID is still there. No handle means it is
// gone.
func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	return true
}
