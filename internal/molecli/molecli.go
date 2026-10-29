// Package molecli drives the Mole CLI (mo) installed on this machine: disk
// analysis, cleanup preview and execution, optimization, and history, as Go
// values a panel can draw.
//
// It does one thing: run mo the way Windows needs it run (no console window, a
// cancellable process, the whole process tree killed on cancel) and turn the
// output into structs. It does not decide anything for the caller. mo clean and
// mo optimize start deleting files the moment they are invoked, so when to call
// them belongs to the UI, after the user has seen a preview and said yes. Nor
// does the package set a deadline of its own: every timeout is the ctx the
// caller hands in.
//
// Finding and running mo is split per platform in process_windows.go and
// process_other.go; parsing is plain functions, so a test can feed them
// strings.
package molecli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNotInstalled reports that mo is not installed here. Callers test it with
// errors.Is.
var ErrNotInstalled = errors.New("没有找到 Mole CLI（mo）")

// Clean is one cleanup: a preview and a real run share the struct, and DryRun
// says which one this is.
type Clean struct {
	// At is when this result was read.
	At time.Time
	// DryRun is true for a --dry-run preview, which deleted nothing.
	DryRun bool
	// Bytes is the byte count mo reported; on a real run it is the tracked
	// cleanup.
	Bytes int64
	// Size is the readable size mo wrote ("132.7MB"), shown as it stands.
	Size string
	// Items is the entry count: potential entries in a preview, cleaned ones
	// after a real run.
	Items int
	// Free is the free space mo reported afterwards ("64.50GB").
	Free string
	// Groups are the paths a preview exported for cleaning, grouped the way mo
	// grouped them ("npm cache"). A real run does not read them: only a preview
	// writes that list, so what sits there after a real run belongs to the
	// previous preview.
	Groups []Group
}

// Group is one group of paths in a cleanup preview, named by the comment line
// mo wrote above them.
type Group struct {
	// Name is the group name, such as "Go module cache".
	Name string
	// Paths are the full paths in this group.
	Paths []string
}

// Analyze is the result of mo analyze.
type Analyze struct {
	// At is when it was read.
	At time.Time
	// Total is the byte count mo reported; a lower bound while Partial is set.
	Total int64
	// Partial says the scan did not finish: some entry is partial or
	// unavailable, so these numbers are not a total.
	Partial bool
	// Entries are mo's entries in its own order, already sorted by size.
	Entries []Entry
}

// Entry is one entry of mo analyze.
type Entry struct {
	// Name is the name mo gave it ("npm Cache").
	Name string
	// Path is the real path of the entry.
	Path string
	// Bytes is its size; an unavailable entry is 0.
	Bytes int64
	// Dir says this is a directory.
	Dir bool
	// Cleanable says mo found a CACHEDIR.TAG here and the contents can be
	// cleared safely.
	Cleanable bool
	// Insight marks a place worth a look, such as Temp and stale downloads.
	Insight bool
}

// History is the result of mo history: sessions are runs, operations are the
// records of one run.
type History struct {
	// At is when it was read.
	At time.Time
	// Sessions are the runs, newest first.
	Sessions []Session
	// Operations are the individual records, newest first.
	Operations []Operation
}

// Session is one mo run.
type Session struct {
	// Command is the subcommand ("clean", "optimize").
	Command string
	// Started is when the run began; zero when the stamp cannot be read.
	Started time.Time
	// Count is how many operations this run logged.
	Count int
	// Actions counts each action name (removed, recycled, skipped, ...).
	Actions map[string]int
}

// Operation is one record in the history.
type Operation struct {
	// At is when it happened; zero when the stamp cannot be read.
	At time.Time
	// Command is the subcommand that produced it.
	Command string
	// Action is the lower-case verb ("removed", "recycled").
	Action string
	// Path is the path involved, possibly empty.
	Path string
	// Detail is the note that came with it, such as a size or a skip reason.
	Detail string
}

// Optimize is the result of mo optimize.
type Optimize struct {
	// At is when it was read.
	At time.Time
	// DryRun is true for a --dry-run preview.
	DryRun bool
	// Applied is how many optimizations would be (preview) or were (real run)
	// applied.
	Applied int
	// Issues is how many problems were found; only a real run reports it.
	Issues int
	// Fixed is how many problems were fixed; only a real run reports it.
	Fixed int
}

// Client is the way into mo. It looks for mo the first time it is needed,
// remembers what it found, and Forget makes it look again.
//
// One Client can be shared between goroutines: the lookup is guarded, and each
// call starts a process of its own.
type Client struct {
	mu     sync.Mutex
	probed bool
	path   string

	// now is the clock, replaceable by a test.
	now func() time.Time
	// look finds mo; nil means really look in PATH and the install folders.
	look func() (string, bool)
}

// New makes a client. It runs nothing.
func New() *Client { return &Client{} }

// Path returns the mo launcher it found, or "" when there is none.
//
// It only looks, never runs: it walks PATH, the install folder and the scoop
// shims for a file that exists, rather than probing with mo --version.
func (c *Client) Path() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.probeLocked()
	return c.path
}

// Available says whether mo can be used here. It only looks, never runs.
func (c *Client) Available() bool { return c.Path() != "" }

// Forget drops the path it found, so the next call looks again. Call it after
// mo is installed.
func (c *Client) Forget() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.probed = false
	c.path = ""
}

// probeLocked looks for mo once; the caller holds c.mu.
func (c *Client) probeLocked() {
	if c.probed {
		return
	}
	c.probed = true
	look := c.look
	if look == nil {
		look = locate
	}
	c.path, _ = look()
}

// clock returns the current time; without a clock of its own it is the real
// one.
func (c *Client) clock() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}

// PreviewClean runs mo clean --dry-run: what would go, how much, and which
// paths, with nothing deleted.
func (c *Client) PreviewClean(ctx context.Context) (Clean, error) {
	out, err := c.run(ctx, "clean", "--dry-run")
	if err != nil {
		return Clean{}, err
	}
	clean, err := parseClean(string(out), defaultCleanList(), true)
	if err != nil {
		return Clean{}, err
	}
	clean.At = c.clock()
	return clean, nil
}

// Clean runs mo clean and really deletes files. The caller must have shown the
// user PreviewClean first.
func (c *Client) Clean(ctx context.Context) (Clean, error) {
	out, err := c.run(ctx, "clean")
	if err != nil {
		return Clean{}, err
	}
	clean, err := parseClean(string(out), defaultCleanList(), false)
	if err != nil {
		return Clean{}, err
	}
	clean.At = c.clock()
	return clean, nil
}

// PreviewOptimize runs mo optimize --dry-run.
func (c *Client) PreviewOptimize(ctx context.Context) (Optimize, error) {
	out, err := c.run(ctx, "optimize", "--dry-run")
	if err != nil {
		return Optimize{}, err
	}
	optimize, err := parseOptimize(string(out), true)
	if err != nil {
		return Optimize{}, err
	}
	optimize.At = c.clock()
	return optimize, nil
}

// Optimize runs mo optimize and really touches the system. The caller must have
// shown the preview first.
func (c *Client) Optimize(ctx context.Context) (Optimize, error) {
	out, err := c.run(ctx, "optimize")
	if err != nil {
		return Optimize{}, err
	}
	optimize, err := parseOptimize(string(out), false)
	if err != nil {
		return Optimize{}, err
	}
	optimize.At = c.clock()
	return optimize, nil
}

// Analyze runs mo analyze --json for how much each drive and folder holds.
func (c *Client) Analyze(ctx context.Context) (Analyze, error) {
	out, err := c.run(ctx, "analyze", "--json")
	if err != nil {
		return Analyze{}, err
	}
	analyze, err := parseAnalyze(string(out))
	if err != nil {
		return Analyze{}, err
	}
	analyze.At = c.clock()
	return analyze, nil
}

// History runs mo history --json --limit N for the recent cleanup record.
//
// limit is clamped to 1-200; below 1 it uses mo's own default of 20.
func (c *Client) History(ctx context.Context, limit int) (History, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	out, err := c.run(ctx, "history", "--json", "--limit", strconv.Itoa(limit))
	if err != nil {
		return History{}, err
	}
	history, err := parseHistory(string(out))
	if err != nil {
		return History{}, err
	}
	history.At = c.clock()
	return history, nil
}

// run starts one mo process and returns its standard output. Every argument is
// passed through.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	path := c.Path()
	if path == "" {
		return nil, ErrNotInstalled
	}
	name, argv := commandFor(path, args)
	out, err := runCommand(ctx, name, argv)
	if err != nil {
		return nil, fmt.Errorf("mo %s：%w", strings.Join(args, " "), err)
	}
	return out, nil
}
