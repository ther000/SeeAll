package system

import (
	"testing"

	"seeall/internal/core"
)

// TestTopProcessesSortsByMemoryAndKeepsFive checks the dashboard's list: the
// five programs holding the most memory, biggest first, with the extension the
// row has no room for trimmed off.
func TestTopProcessesSortsByMemoryAndKeepsFive(t *testing.T) {
	list := []core.Process{
		{Name: "Everything.exe", Mem: 90 << 20},
		{Name: "chrome.exe", Mem: 1200 << 20, CPU: 12},
		{Name: "Code.exe", Mem: 700 << 20},
		{Name: "QQ.exe", Mem: 400 << 20},
		{Name: "explorer.exe", Mem: 150 << 20},
		{Name: "WeChat.exe", Mem: 300 << 20},
		{Name: "msedge.exe", Mem: 800 << 20},
	}
	got := topProcesses(list, 5)
	if len(got) != 5 {
		t.Fatalf("the list has %d rows, want 5: %v", len(got), got)
	}
	want := []string{"chrome", "msedge", "Code", "QQ", "WeChat"}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("row %d is %q, want %q", i, got[i].Name, name)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i].Mem > got[i-1].Mem {
			t.Errorf("the rows are not in order: %v", got)
		}
	}
	if got[0].Mem != 1200<<20 || got[0].CPU != 12 {
		t.Errorf("the first row lost its numbers: %+v", got[0])
	}
	// The caller's list is the sampler's, in the order Windows handed it over:
	// sorting it in place would change what the next sample compares against.
	if list[0].Name != "Everything.exe" {
		t.Errorf("the sample's list was reordered: %v", list)
	}
}

// TestTopProcessesBreaksTiesByProgramName checks two programs holding the same
// memory keep a stable order between samples, or the list would shuffle while
// nothing moved.
func TestTopProcessesBreaksTiesByProgramName(t *testing.T) {
	got := topProcesses([]core.Process{
		{Name: "b.exe", Mem: 100},
		{Name: "a.exe", Mem: 100},
	}, 5)
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("a tie reads %v", got)
	}
}

// TestProgramNameTrimsOnlyTheExtension checks the row shows "Everything" and
// not "Everything.exe", without making a hash of a program that has no
// extension or is called something else.
func TestProgramNameTrimsOnlyTheExtension(t *testing.T) {
	cases := map[string]string{
		"Everything.exe": "Everything",
		"chrome.EXE":     "chrome",
		" Code.exe ":     "Code",
		"systemd":        "systemd",
		"":               "",
	}
	for in, want := range cases {
		if got := programName(in); got != want {
			t.Errorf("programName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTopProcessesGroupsOneProgramIntoOneRow checks a browser's dozen
// processes read as the program they belong to: five rows of chrome would fill
// the card and say less than one row of what chrome costs together.
func TestTopProcessesGroupsOneProgramIntoOneRow(t *testing.T) {
	got := topProcesses([]core.Process{
		{Name: "chrome.exe", Mem: 500 << 20, CPU: 4},
		{Name: "chrome.exe", Mem: 300 << 20, CPU: 2.5},
		{Name: "Code.exe", Mem: 600 << 20, CPU: 1},
	}, 5)
	if len(got) != 2 {
		t.Fatalf("the list is %v, want one row per program", got)
	}
	if got[0].Name != "chrome" || got[0].Mem != 800<<20 || got[0].CPU != 6.5 {
		t.Errorf("chrome reads %+v, want 800 MB and 6.5%% together", got[0])
	}
	if got[1].Name != "Code" {
		t.Errorf("the second row is %q", got[1].Name)
	}
}

// TestTopProcessesOfNothing checks the dashboard's card does not appear when
// the sampler has no programs to name: an empty list is what the panel reads as
// "there is no such card".
func TestTopProcessesOfNothing(t *testing.T) {
	if got := topProcesses(nil, 5); got != nil {
		t.Errorf("an empty sample reads %v", got)
	}
	if got := topProcesses([]core.Process{{Name: "a.exe", Mem: 1}}, 0); got != nil {
		t.Errorf("a list of room for no rows reads %v", got)
	}
	if got := topProcesses([]core.Process{{Mem: 1}}, 5); got != nil {
		t.Errorf("a nameless process was listed as %v", got)
	}
}
