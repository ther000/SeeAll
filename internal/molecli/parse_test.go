package molecli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// read returns one testdata file. These are what the tools really printed on
// this machine (the two hand-written ones say so where they are used), so the
// parser has to recognise them as they are.
func read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestParseSize pins size parsing: plain numbers, fractions, letter case, a
// space, and a thousands comma all work; an empty or unknown one is 0 rather
// than an error.
func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"123B", 123},
		{"123b", 123},
		{"0B", 0},
		{"1.5KB", 1536},
		{"1.5 KB", 1536},
		{"132.7MB", 139146035},
		{"132.7 mb", 139146035},
		{"64.51GB", 69267085066},
		{"2TB", 2199023255552},
		{"2 TB", 2199023255552},
		{"1,234KB", 1263616},
		{"12,345B", 12345},
		{"888KB", 909312},
		{" 132.7MB ", 139146035},
		{"\x1b[0;32m132.7MB\x1b[0m", 139146035},
		{"", 0},
		{"  ", 0},
		{"unknown", 0},
		{"MB", 0},
		{"-3MB", 0},
		{"1.2.3MB", 0},
	}
	for _, c := range cases {
		if got := parseSize(c.in); got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestStripANSI pins colour removal: colour, clear-line and set-title sequences
// all go, and every other byte stays.
func TestStripANSI(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain text", "plain text"},
		{"\x1b[0;32m132.7MB\x1b[0m", "132.7MB"},
		{"\x1b[1;35mClean Your Windows\x1b[0m", "Clean Your Windows"},
		{"\x1b[2K\rLoading", "\rLoading"},
		{"\x1b[?25lhidden", "hidden"},
		{"\x1b]0;mo - clean\x07after", "after"},
		{"两个\x1b[33m中文\x1b[0m", "两个中文"},
	}
	for _, c := range cases {
		if got := stripANSI(c.in); got != c.want {
			t.Errorf("stripANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestTailText pins the error tail: ANSI gone, only the last few hundred bytes,
// and starting at a whole line.
func TestTailText(t *testing.T) {
	long := strings.Repeat("old progress line\n", 200) + "\x1b[31mlast real error\x1b[0m\n"
	got := tailText(long)
	if strings.Contains(got, "\x1b") {
		t.Errorf("the tail still carries ANSI: %q", got)
	}
	if !strings.HasSuffix(got, "last real error") {
		t.Errorf("the tail lost the last line: %q", got)
	}
	if len(got) > tailBytes {
		t.Errorf("the tail is %d bytes, over the %d byte cut", len(got), tailBytes)
	}
	if strings.HasPrefix(got, "rogress line") {
		t.Errorf("the tail starts mid-line: %q", got)
	}

	if got := tailText("  \n\n"); got != "" {
		t.Errorf("empty output should give an empty tail, got %q", got)
	}
}

// withListPath replaces the path on the "Detailed file list: ..." line of a
// preview, or drops the line when path is empty. The file captured on this
// machine names the path it had at the time; pointing it at local testdata is
// what makes the case replayable on another machine.
func withListPath(t *testing.T, text, path string) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		idx := strings.Index(line, "Detailed file list:")
		switch {
		case idx < 0:
			kept = append(kept, line)
		case path != "":
			kept = append(kept, line[:idx]+"Detailed file list: "+path)
		}
	}
	return strings.Join(kept, "\n")
}

// TestParseClean is the table for the cleanup summary: a preview captured here,
// nothing to clean, a real run summary, only tool caches, and output that
// cannot be read.
func TestParseClean(t *testing.T) {
	dryRun := read(t, "clean-dry.txt")
	list := filepath.Join("testdata", "clean-list.txt")
	missing := filepath.Join("testdata", "does-not-exist.txt")
	cases := []struct {
		name     string
		text     string
		listPath string
		dry      bool
		want     Clean
		wantErr  bool
	}{
		{
			// The preview captured here: the numbers sit inside ANSI and the
			// path list has four groups. The fallback path does not exist, which
			// proves the one named in the output is the one read.
			name:     "preview with ANSI and a path list",
			text:     withListPath(t, dryRun, list),
			listPath: missing,
			dry:      true,
			want: Clean{
				DryRun: true,
				Bytes:  139146035,
				Size:   "132.7MB",
				Items:  3,
				Free:   "64.50GB",
				Groups: []Group{
					{Name: "npm cache", Paths: []string{`C:\Users\Administrator\AppData\Local\npm-cache`}},
					{Name: "Bun cache", Paths: []string{`C:\Users\Administrator\.bun\install\cache\@t@`}},
					{Name: "Go build cache", Paths: []string{`C:\Users\Administrator\AppData\Local\go-build`}},
					{Name: "Go module cache", Paths: []string{
						`C:\Users\Administrator\go\pkg\mod\cache\download`,
						`C:\Users\Administrator\go\pkg\mod\cache\lock`,
					}},
				},
			},
		},
		{
			// The list the output names is gone (deleted after the preview).
			// The numbers still show, the groups fall back to empty, and none
			// of that is an error.
			name:     "the named list does not exist",
			text:     withListPath(t, dryRun, missing),
			listPath: list,
			dry:      true,
			want: Clean{
				DryRun: true,
				Bytes:  139146035,
				Size:   "132.7MB",
				Items:  3,
				Free:   "64.50GB",
			},
		},
		{
			// An older mo does not write the list path, so the fallback is read.
			name:     "no list path in the output falls back to the default",
			text:     withListPath(t, dryRun, ""),
			listPath: list,
			dry:      true,
			want: Clean{
				DryRun: true,
				Bytes:  139146035,
				Size:   "132.7MB",
				Items:  3,
				Free:   "64.50GB",
				Groups: []Group{
					{Name: "npm cache", Paths: []string{`C:\Users\Administrator\AppData\Local\npm-cache`}},
					{Name: "Bun cache", Paths: []string{`C:\Users\Administrator\.bun\install\cache\@t@`}},
					{Name: "Go build cache", Paths: []string{`C:\Users\Administrator\AppData\Local\go-build`}},
					{Name: "Go module cache", Paths: []string{
						`C:\Users\Administrator\go\pkg\mod\cache\download`,
						`C:\Users\Administrator\go\pkg\mod\cache\lock`,
					}},
				},
			},
		},
		{
			// The list holds only its header: nothing to list, not an error.
			name:     "the list is header only",
			text:     withListPath(t, dryRun, filepath.Join("testdata", "clean-list-header-only.txt")),
			listPath: list,
			dry:      true,
			want: Clean{
				DryRun: true,
				Bytes:  139146035,
				Size:   "132.7MB",
				Items:  3,
				Free:   "64.50GB",
			},
		},
		{
			// Hand-written from clean.ps1's Show-CleanupSummary: a real run.
			name:     "real run summary",
			text:     read(t, "clean-done.txt"),
			listPath: list,
			dry:      false,
			want: Clean{
				DryRun: false,
				Bytes:  139146035,
				Size:   "132.7MB",
				Items:  3,
				Free:   "64.65GB",
			},
		},
		{
			// After a real run the list file is still there, but it is the one
			// the last preview left behind and must not be read as what this run
			// cleaned.
			name:     "stale list left by an earlier preview",
			text:     "Tracked cleanup: 888KB | Items cleaned: 1\nFree space: 64.65GB\n",
			listPath: list,
			dry:      false,
			want: Clean{
				DryRun: false,
				Bytes:  909312,
				Size:   "888KB",
				Items:  1,
				Free:   "64.65GB",
			},
		},
		{
			// Hand-written preview: with nothing to clean mo writes only this
			// line, with no numbers.
			name:    "preview with nothing to clean",
			text:    read(t, "clean-nothing.txt"),
			dry:     true,
			want:    Clean{DryRun: true, Free: "64.50GB"},
			wantErr: false,
		},
		{
			name: "real run with nothing to clean",
			text: "System was already clean; no additional space freed.\nFree space: 64.65GB\n",
			dry:  false,
			want: Clean{Free: "64.65GB"},
		},
		{
			name: "only tool caches, no numbers",
			text: "Dry run complete - no changes made\nTool caches are listed for review.\nFree space: 64.50GB\n",
			dry:  true,
			want: Clean{DryRun: true, Free: "64.50GB"},
		},
		{
			name: "a single item",
			text: "\x1b[0;32m888KB\x1b[0m\nPotential space: 888KB | Items: 1\nFree space: 64.50GB\n",
			dry:  true,
			want: Clean{DryRun: true, Bytes: 909312, Size: "888KB", Items: 1, Free: "64.50GB"},
		},
		{
			// PowerShell's N0 format puts a comma into a four-digit number.
			name: "thousands comma",
			text: "Potential space: 1,234KB | Items: 2\nFree space: 64.50GB\n",
			dry:  true,
			want: Clean{DryRun: true, Bytes: 1263616, Size: "1,234KB", Items: 2, Free: "64.50GB"},
		},
		{
			// Another host writing CRLF endings must still match the summary
			// lines.
			name: "CRLF line endings",
			text: strings.ReplaceAll(withListPath(t, dryRun, ""), "\n", "\r\n"),
			dry:  true,
			want: Clean{DryRun: true, Bytes: 139146035, Size: "132.7MB", Items: 3, Free: "64.50GB"},
		},
		{
			name:    "output that cannot be read",
			text:    "mo: command not found\nsomething else entirely\n",
			dry:     true,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseClean(c.text, c.listPath, c.dry)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseClean got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

// TestParseCleanList pins the grouping of the list: the header is not a group,
// a blank line breaks one, and a group with no paths is dropped.
func TestParseCleanList(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []Group
	}{
		{
			name: "list captured on this machine",
			text: read(t, "clean-list.txt"),
			want: []Group{
				{Name: "npm cache", Paths: []string{`C:\Users\Administrator\AppData\Local\npm-cache`}},
				{Name: "Bun cache", Paths: []string{`C:\Users\Administrator\.bun\install\cache\@t@`}},
				{Name: "Go build cache", Paths: []string{`C:\Users\Administrator\AppData\Local\go-build`}},
				{Name: "Go module cache", Paths: []string{
					`C:\Users\Administrator\go\pkg\mod\cache\download`,
					`C:\Users\Administrator\go\pkg\mod\cache\lock`,
				}},
			},
		},
		{
			name: "header only",
			text: read(t, "clean-list-header-only.txt"),
		},
		{
			name: "empty file",
			text: "",
		},
		{
			name: "list without a header",
			text: "# npm cache\nC:\\cache\\npm\n",
			want: []Group{{Name: "npm cache", Paths: []string{`C:\cache\npm`}}},
		},
		{
			name: "a group with no paths is dropped",
			text: "# Mole Cleanup Preview - 2026-10-29\n#\n\n# empty group\n\n# npm cache\nC:\\cache\\npm\n",
			want: []Group{{Name: "npm cache", Paths: []string{`C:\cache\npm`}}},
		},
		{
			name: "header with a BOM",
			text: "\ufeff# Mole Cleanup Preview - 2026-10-29\n#\n\n# npm cache\nC:\\cache\\npm\n",
			want: []Group{{Name: "npm cache", Paths: []string{`C:\cache\npm`}}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseCleanList(c.text); !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseCleanList got %+v\nwant %+v", got, c.want)
			}
		})
	}
}

// TestParseAnalyze pins disk analysis: a complete scan, a partial one (with an
// unavailable entry), and JSON that is broken.
func TestParseAnalyze(t *testing.T) {
	t.Run("a complete scan", func(t *testing.T) {
		got, err := parseAnalyze(read(t, "analyze.json"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Partial {
			t.Error("this scan is complete and must not be marked partial")
		}
		if got.Total != 825590013952 {
			t.Errorf("total is %d, want 825590013952", got.Total)
		}
		if len(got.Entries) != 10 {
			t.Fatalf("%d entries, want 10", len(got.Entries))
		}
		first := got.Entries[0]
		if first.Name != "游戏 (F:)" || first.Path != `F:\` || first.Bytes != 456126095360 || !first.Dir {
			t.Errorf("the first entry parsed as %+v", first)
		}
		last := got.Entries[9]
		if last.Name != "npm Cache" || !last.Insight || last.Cleanable {
			t.Errorf("the last entry parsed as %+v", last)
		}
	})

	t.Run("a partial scan", func(t *testing.T) {
		got, err := parseAnalyze(read(t, "analyze-partial.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !got.Partial {
			t.Error("one entry is unavailable, so the whole result must be partial")
		}
		if len(got.Entries) != 4 {
			t.Fatalf("%d entries, want 4", len(got.Entries))
		}
		if !got.Entries[2].Cleanable || got.Entries[2].Bytes != 0 {
			t.Errorf("the unavailable entry parsed as %+v", got.Entries[2])
		}
	})

	t.Run("truncated JSON", func(t *testing.T) {
		if _, err := parseAnalyze(read(t, "analyze-cut.json")); err == nil {
			t.Fatal("truncated JSON must be an error")
		} else if !strings.Contains(err.Error(), "JSON") {
			t.Errorf("the error does not say the JSON is unreadable: %v", err)
		}
	})

	t.Run("empty output", func(t *testing.T) {
		if _, err := parseAnalyze(""); err == nil {
			t.Fatal("empty output must be an error")
		}
	})
}

// TestParseHistory pins history: an empty log, three sessions with three
// operations, and bad JSON.
func TestParseHistory(t *testing.T) {
	t.Run("no records at all", func(t *testing.T) {
		got, err := parseHistory(read(t, "history-empty.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Sessions) != 0 || len(got.Operations) != 0 {
			t.Errorf("an empty history parsed as %+v", got)
		}
	})

	t.Run("three sessions", func(t *testing.T) {
		got, err := parseHistory(read(t, "history.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Sessions) != 3 || len(got.Operations) != 3 {
			t.Fatalf("parsed %d sessions and %d operations, want 3 and 3",
				len(got.Sessions), len(got.Operations))
		}
		first := got.Sessions[0]
		if first.Command != "clean" || first.Count != 2 {
			t.Errorf("the first session parsed as %+v", first)
		}
		want := time.Date(2026, 10, 28, 9, 14, 2, 0, time.Local)
		if !first.Started.Equal(want) {
			t.Errorf("the session started at %v, want %v", first.Started, want)
		}
		if first.Actions["removed"] != 1 || first.Actions["recycled"] != 1 {
			t.Errorf("the session actions parsed as %+v", first.Actions)
		}
		second := got.Operations[1]
		if second.Action != "recycled" || second.Path != `C:\Users\Administrator\.bun\install\cache` {
			t.Errorf("the second operation parsed as %+v", second)
		}
		if want := time.Date(2026, 10, 28, 9, 14, 11, 0, time.Local); !second.At.Equal(want) {
			t.Errorf("the operation is stamped %v, want %v", second.At, want)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		if _, err := parseHistory(`{"sessions": [`); err == nil {
			t.Fatal("bad JSON must be an error")
		}
	})

	t.Run("an unreadable stamp does not sink the history", func(t *testing.T) {
		got, err := parseHistory(`{"sessions":[{"command":"clean","started_at":"昨天下午"}],
			"operations":[{"timestamp":"2026-10-28 09:14:07","command":"clean","action":"removed",
			"path":"C:\\\\x","detail":"1MB"}]}`)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Sessions[0].Started.IsZero() {
			t.Errorf("an unreadable stamp should stay zero, got %v", got.Sessions[0].Started)
		}
		if got.Operations[0].At.IsZero() {
			t.Error("a readable stamp in the same document must not be lost")
		}
	})
}

// TestParseOptimize pins the optimization summary: a preview, the singular
// form, a real run with issue counts, and output that cannot be read.
func TestParseOptimize(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		dry     bool
		want    Optimize
		wantErr bool
	}{
		{
			name: "preview captured on this machine",
			text: read(t, "optimize-dry.txt"),
			dry:  true,
			want: Optimize{DryRun: true, Applied: 3},
		},
		{
			name: "singular",
			text: "Would apply 1 optimization\n",
			dry:  true,
			want: Optimize{DryRun: true, Applied: 1},
		},
		{
			// Hand-written from optimize.ps1's Show-OptimizeSummary: a real run.
			name: "real run with issue counts",
			text: read(t, "optimize-done.txt"),
			dry:  false,
			want: Optimize{Applied: 3, Fixed: 2, Issues: 1},
		},
		{
			name: "real run with no issues reports the count only",
			text: "======================================================================\nOptimization complete\nOptimizations applied: 0\n======================================================================\n",
			dry:  false,
			want: Optimize{},
		},
		{
			name:    "output that cannot be read",
			text:    "Optimize and Maintain\nnothing to see\n",
			dry:     true,
			wantErr: true,
		},
		{
			// A preview line must not be taken as a real run's result, or the
			// other way round.
			name:    "preview output fed to the real-run parser",
			text:    read(t, "optimize-dry.txt"),
			dry:     false,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseOptimize(c.text, c.dry)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("parseOptimize got %+v, want %+v", got, c.want)
			}
		})
	}
}
