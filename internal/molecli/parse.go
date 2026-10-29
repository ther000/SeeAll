package molecli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Summary lines. mo wraps the size in ANSI colour codes, so the colour comes
// off first and these ASCII lines are matched afterwards; the Chinese on the
// same screen turns into question marks under a GBK console, and nothing here
// relies on it.
var (
	rePotentialSpace = regexp.MustCompile(`(?m)^Potential space:[ \t]*(\S+)[ \t]*\|[ \t]*Items:[ \t]*(\d+)[ \t]*$`)
	reTrackedCleanup = regexp.MustCompile(`(?m)^Tracked cleanup:[ \t]*(\S+)[ \t]*\|[ \t]*Items cleaned:[ \t]*(\d+)[ \t]*$`)
	reFreeSpace      = regexp.MustCompile(`(?m)^Free space:[ \t]*(\S+)[ \t]*$`)
	reDetailedList   = regexp.MustCompile(`(?m)^Detailed file list:[ \t]*(.+?)[ \t]*$`)
	reWouldApply     = regexp.MustCompile(`(?m)^Would apply[ \t]+(\d+)[ \t]+optimization`)
	reAppliedOpt     = regexp.MustCompile(`(?m)^Optimizations applied:[ \t]*(\d+)`)
	reIssuesFixed    = regexp.MustCompile(`(?m)^Issues fixed:[ \t]*(\d+)`)
	reIssuesFound    = regexp.MustCompile(`(?m)^Issues found:[ \t]*(\d+)`)
)

// The two sentences mo writes when there is nothing to clean, one for a preview
// and one for a real run. Both are accepted: which one appears depends on
// whether there was anything to clean, not on how the command was called.
const (
	noSpaceDry  = "No significant reclaimable space detected."
	noSpaceReal = "System was already clean; no additional space freed."
	// With only the tools' own caches to clear, mo reports no numbers and
	// writes just this line.
	ownerCachesDry  = "Tool caches are listed for review."
	ownerCachesReal = "Tool cache cleanup completed."
)

// parseClean parses the output of mo clean. dry says whether it came from
// --dry-run or from a real run.
//
// listPath is the fallback location of the path list when the output does not
// name one; a "Detailed file list: ..." line wins over it. A list that cannot
// be read, or is empty, leaves Groups empty, which is no error: the numbers are
// still there to show.
func parseClean(text string, listPath string, dry bool) (Clean, error) {
	plain := plainText(text)
	clean := Clean{DryRun: dry}

	summary := rePotentialSpace
	if !dry {
		summary = reTrackedCleanup
	}
	if m := summary.FindStringSubmatch(plain); m != nil {
		clean.Size = m[1]
		clean.Bytes = parseSize(m[1])
		clean.Items, _ = strconv.Atoi(m[2])
	} else if !strings.Contains(plain, noSpaceDry) &&
		!strings.Contains(plain, noSpaceReal) &&
		!strings.Contains(plain, ownerCachesDry) &&
		!strings.Contains(plain, ownerCachesReal) {
		// Not even "nothing to clean" was said, so this is not mo clean's
		// output (a wrong command, or something cut it short). That is not
		// zero bytes, that is output this package cannot read: the caller gets
		// the error.
		return Clean{}, fmt.Errorf("读不懂 mo clean 的输出：%q", tailText(text))
	}

	if m := reFreeSpace.FindStringSubmatch(plain); m != nil {
		clean.Free = m[1]
	}

	// Only a preview writes the path list. After a real run the file is still
	// there, but it is the previous preview's, and reading it would report
	// those old paths as what this run cleaned. A real run never reads it.
	if dry {
		path := listPath
		if m := reDetailedList.FindStringSubmatch(plain); m != nil {
			path = strings.TrimSpace(m[1])
		}
		if path != "" {
			if data, err := os.ReadFile(path); err == nil {
				clean.Groups = parseCleanList(string(data))
			}
		}
	}
	return clean, nil
}

// parseCleanList parses the path list a preview exports: after the header, a
// "# group name" line opens a group and every line below it is a path. Blank
// lines, the header comments and paths with no group above them are skipped.
func parseCleanList(text string) []Group {
	lines := strings.Split(plainText(text), "\n")
	var groups []Group
	inHeader := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if i == 0 {
			line = strings.TrimPrefix(line, "\ufeff")
			// The list opens with an explanation mo writes, and the header runs
			// to the first blank line. A file without that explanation (a
			// truncated one, say) is read from the first line on.
			if strings.HasPrefix(line, "# Mole Cleanup Preview") {
				inHeader = true
				continue
			}
		}
		if inHeader {
			if line == "" {
				inHeader = false
			}
			continue
		}
		if line == "" {
			continue
		}
		if name, ok := strings.CutPrefix(line, "#"); ok {
			groups = append(groups, Group{Name: strings.TrimSpace(name)})
			continue
		}
		if len(groups) == 0 {
			continue
		}
		groups[len(groups)-1].Paths = append(groups[len(groups)-1].Paths, line)
	}

	// A group with a name but no paths, and a group with no name, should not
	// take a row on the panel.
	kept := groups[:0]
	for _, g := range groups {
		if g.Name != "" && len(g.Paths) > 0 {
			kept = append(kept, g)
		}
	}
	return kept
}

// analyzeJSON is the object mo analyze --json prints, with only the fields this
// package uses.
type analyzeJSON struct {
	ScanStatus string `json:"scan_status"`
	TotalSize  int64  `json:"total_size"`
	Entries    []struct {
		ScanStatus string `json:"scan_status"`
		Name       string `json:"name"`
		Path       string `json:"path"`
		Size       int64  `json:"size"`
		IsDir      bool   `json:"is_dir"`
		Cleanable  bool   `json:"cleanable"`
		Insight    bool   `json:"insight"`
	} `json:"entries"`
}

// parseAnalyze parses mo analyze --json. The output is UTF-8 JSON and goes
// straight to the decoder.
func parseAnalyze(text string) (Analyze, error) {
	var doc analyzeJSON
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return Analyze{}, fmt.Errorf("读不懂 mo analyze 的 JSON（%v）：%q", err, tailText(text))
	}
	analyze := Analyze{Total: doc.TotalSize, Partial: doc.ScanStatus != "complete"}
	for _, entry := range doc.Entries {
		if entry.ScanStatus != "complete" {
			// A partial entry's size is a lower bound, so the whole result
			// cannot be reported as a total.
			analyze.Partial = true
		}
		analyze.Entries = append(analyze.Entries, Entry{
			Name:      entry.Name,
			Path:      entry.Path,
			Bytes:     entry.Size,
			Dir:       entry.IsDir,
			Cleanable: entry.Cleanable,
			Insight:   entry.Insight,
		})
	}
	return analyze, nil
}

// historyJSON is the object mo history --json prints. PowerShell's ConvertTo-Json
// writes lower-case keys with underscores; fields this package does not know are
// ignored.
type historyJSON struct {
	Sessions []struct {
		Command        string         `json:"command"`
		RunID          string         `json:"run_id"`
		StartedAt      string         `json:"started_at"`
		OperationCount int            `json:"operation_count"`
		Actions        map[string]int `json:"actions"`
	} `json:"sessions"`
	Operations []struct {
		Timestamp string `json:"timestamp"`
		Command   string `json:"command"`
		RunID     string `json:"run_id"`
		Action    string `json:"action"`
		Path      string `json:"path"`
		Detail    string `json:"detail"`
	} `json:"operations"`
}

// stampLayouts are the two shapes mo writes a time in: local time in the log,
// and occasionally RFC 3339 on the JSON surface. An unreadable stamp becomes the
// zero time instead of taking the whole history down with it.
var stampLayouts = []string{
	"2006-01-02 15:04:05",
	time.RFC3339,
}

// stamp reads one timestamp and returns the zero time when it cannot.
func stamp(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range stampLayouts {
		if at, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return at
		}
	}
	return time.Time{}
}

// parseHistory parses mo history --json. An empty history and bad JSON are two
// different things: the first returns an empty struct, the second an error.
func parseHistory(text string) (History, error) {
	var doc historyJSON
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return History{}, fmt.Errorf("读不懂 mo history 的 JSON（%v）：%q", err, tailText(text))
	}
	var history History
	for _, session := range doc.Sessions {
		history.Sessions = append(history.Sessions, Session{
			Command: session.Command,
			Started: stamp(session.StartedAt),
			Count:   session.OperationCount,
			Actions: session.Actions,
		})
	}
	for _, operation := range doc.Operations {
		history.Operations = append(history.Operations, Operation{
			At:      stamp(operation.Timestamp),
			Command: operation.Command,
			Action:  operation.Action,
			Path:    operation.Path,
			Detail:  operation.Detail,
		})
	}
	return history, nil
}

// parseOptimize parses the output of mo optimize. A preview reports "Would apply
// N optimizations"; a real run reports "Optimizations applied: N", possibly
// followed by the counts of problems fixed and found.
func parseOptimize(text string, dry bool) (Optimize, error) {
	plain := plainText(text)
	optimize := Optimize{DryRun: dry}
	found := false
	if dry {
		if m := reWouldApply.FindStringSubmatch(plain); m != nil {
			optimize.Applied, _ = strconv.Atoi(m[1])
			found = true
		}
	} else {
		if m := reAppliedOpt.FindStringSubmatch(plain); m != nil {
			optimize.Applied, _ = strconv.Atoi(m[1])
			found = true
		}
		if m := reIssuesFixed.FindStringSubmatch(plain); m != nil {
			optimize.Fixed, _ = strconv.Atoi(m[1])
		}
		if m := reIssuesFound.FindStringSubmatch(plain); m != nil {
			optimize.Issues, _ = strconv.Atoi(m[1])
		}
	}
	if !found {
		return Optimize{}, fmt.Errorf("读不懂 mo optimize 的输出：%q", tailText(text))
	}
	return optimize, nil
}

// parseSize reads a size mo wrote as a byte count: "123B", "1.5KB", "132.7MB",
// "64.51GB", "2TB".
//
// Upper and lower case both work, as does a space between the number and the
// unit, and a thousands comma -- PowerShell's {0:N0} writes a four-digit KB
// count as "1,234KB". An empty string, or anything unrecognised, is 0 rather
// than an error: a broken size should not turn a whole preview into a failure,
// and showing 0 beats popping an error.
func parseSize(s string) int64 {
	s = strings.TrimSpace(stripANSI(s))
	if s == "" {
		return 0
	}
	s = strings.ToUpper(strings.ReplaceAll(s, ",", ""))
	mult := int64(1)
	for _, unit := range []struct {
		suffix string
		size   int64
	}{
		{"TB", 1 << 40},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"B", 1},
	} {
		if after, ok := strings.CutSuffix(s, unit.suffix); ok {
			mult = unit.size
			s = strings.TrimSpace(after)
			break
		}
	}
	if s == "" {
		return 0
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || value < 0 {
		return 0
	}
	return int64(value * float64(mult))
}

// ansiEscape matches CSI (colour, clear line) and OSC (set title), the two
// kinds of escape sequence mo uses.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// stripANSI drops terminal escape sequences and leaves text that can be read.
func stripANSI(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	return ansiEscape.ReplaceAllString(s, "")
}

// plainText is the step before parsing: strip the ANSI, then fold CRLF into LF.
//
// mo writes LF into a pipe today, but its output is put together by PowerShell's
// Write-Host and another host can switch it to CRLF. A regex that only accepted
// LF would match nothing on such a machine, so the line endings are folded
// first.
func plainText(s string) string {
	return strings.ReplaceAll(stripANSI(s), "\r\n", "\n")
}

// tailBytes is how much output an error message carries: enough to see the last
// failure, not enough to push a whole progress screen into the log.
const tailBytes = 400

// tailText folds output into an error tail: ANSI gone, blank ends gone, invalid
// UTF-8 dropped, and past 400 bytes only the end is kept, starting at a whole
// line so nothing is cut in half.
//
// The UTF-8 pass is not only for the cut: mo writes through a console code page
// (CP936 here), so a Chinese path in a failure message arrives as bytes that are
// not UTF-8 at all, and they would reach the panel as replacement glyphs.
func tailText(s string) string {
	s = strings.ToValidUTF8(strings.TrimSpace(stripANSI(s)), "")
	if len(s) <= tailBytes {
		return s
	}
	cut := len(s) - tailBytes
	if at := strings.IndexByte(s[cut:], '\n'); at >= 0 {
		cut += at + 1
	}
	return strings.ToValidUTF8(s[cut:], "")
}

// defaultCleanList is where a preview exports its path list, used when the
// output does not name one.
func defaultCleanList() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mole", "clean-list.txt")
}
