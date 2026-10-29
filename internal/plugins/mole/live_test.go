package mole

import (
	"os"
	"testing"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// TestLiveReadOnlyCommands drives the plugin the way the page does — the
// instance, the command, the parse, the state the view reads — against the
// machine's real Mole CLI. It runs only the two reading commands, so it deletes
// nothing; the two that remove files are refused by the same code this test
// goes through, and are covered by the unit tests.
//
// It is skipped by default: it starts a few PowerShell processes and takes
// several seconds.
//
//	SEEALL_MO_LIVE=1 go test ./internal/plugins/mole -run TestLive -v
func TestLiveReadOnlyCommands(t *testing.T) {
	if os.Getenv("SEEALL_MO_LIVE") != "1" {
		t.Skip("real CLI smoke test: set SEEALL_MO_LIVE=1 to run it")
	}
	env := plugin.NewEnv("mole", Plugin{}.Manifest(), plugin.NewSettings(nil))
	env.SetPublishHook(func(string, core.Snapshot) {})
	inst, ok := Plugin{}.New(env).(*Instance)
	if !ok {
		t.Fatal("the plugin did not make its own instance")
	}
	if st := inst.State(); !st.Ready {
		t.Fatalf("Mole is not installed here")
	}

	// wait runs one command and waits for it to finish, the way the page's
	// two-second repaint would.
	wait := func(t *testing.T, act Action) Data {
		t.Helper()
		if err := inst.Run(act); err != nil {
			t.Fatalf("Run(%s): %v", act, err)
		}
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			st := inst.State()
			if st.Running == nil {
				if st.Last == nil || st.Last.Err {
					t.Fatalf("%s failed: %+v", act, st.Last)
				}
				return st
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("%s did not finish in three minutes", act)
		return Data{}
	}

	start := time.Now()
	st := wait(t, ScanDisk)
	if st.Disk == nil || len(st.Disk.Entries) == 0 {
		t.Fatalf("mo analyze returned no entries: %+v", st.Disk)
	}
	t.Logf("analyze: %d entries, %s, %v", len(st.Disk.Entries),
		core.FormatBytes(float64(st.Disk.Total)), time.Since(start).Round(time.Millisecond))

	start = time.Now()
	st = wait(t, ScanHistory)
	if st.History == nil {
		t.Fatal("mo history returned nothing")
	}
	t.Logf("history: %d sessions, %v", len(st.History.Sessions),
		time.Since(start).Round(time.Millisecond))

	// The page's safety rule holds on the real instance too: nothing has been
	// previewed, so a cleanup is refused without the CLI being started.
	if err := inst.Run(Clean); err == nil {
		t.Fatal("Run(Clean) was allowed without a preview")
	}
}
