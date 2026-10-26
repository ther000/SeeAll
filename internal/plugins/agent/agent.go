// Package agent watches AI coding agents running on this computer: how many
// are up, what they are doing and how long they have been busy.
//
// The plugin looks for the agent processes that name themselves on the
// command line (Claude Code, Codex, Aider, Gemini CLI and the like), groups
// them by kind, and reports the newest one. It reads no conversation and no
// file the agents write: the process table is all it needs.
package agent

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// known maps the name of a process's executable, or a word of its command
// line, to how the panel names the agent.
var known = []struct {
	Match string
	Name  string
}{
	{"claude", "Claude Code"},
	{"codex", "Codex"},
	{"aider", "Aider"},
	{"gemini", "Gemini CLI"},
	{"cursor-agent", "Cursor Agent"},
	{"copilot", "Copilot CLI"},
	{"opencode", "opencode"},
	{"cline", "Cline"},
	{"amp", "Amp"},
	{"goose", "Goose"},
}

// Process is one running process this plugin keeps.
type Process struct {
	PID         int
	Command     string
	Agent       string
	CPUSeconds  float64
	Started     time.Time
	MemoryBytes uint64
}

// Manifest describes the plugin to the app.
func Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "agent",
		Name:            "Agent",
		Summary:         "本机运行的 AI 编码代理：数量、活动与耗时",
		Kind:            plugin.KindAgent,
		Icon:            "terminal",
		DefaultInstance: "Agent",
		Interval:        3 * time.Second,
		Settings: []plugin.Setting{
			{Key: "extra", Label: "额外进程名", Type: plugin.SettingText, Default: "",
				Placeholder: "my-agent,another",
				Description: "用逗号隔开，除了内置的那几个还要盯的名字"},
			{Key: "hide_idle", Label: "隐藏空闲的", Type: plugin.SettingBool, Default: "false"},
		},
	}
}

// Plugin is the agent monitor.
type Plugin struct{}

// Manifest implements plugin.Plugin.
func (Plugin) Manifest() plugin.Manifest { return Manifest() }

// New implements plugin.Plugin.
func (Plugin) New(env *plugin.Env) plugin.Instance { return &instance{env: env} }

type instance struct {
	env *plugin.Env

	mu       sync.Mutex
	prev     map[int]Process
	activity map[int]float64
	seen     map[int]time.Time
	started  bool
	quit     chan struct{}
}

// Start implements plugin.Instance.
func (in *instance) Start() {
	in.mu.Lock()
	if in.started {
		in.mu.Unlock()
		return
	}
	in.started = true
	in.quit = make(chan struct{})
	if in.prev == nil {
		in.prev = map[int]Process{}
		in.activity = map[int]float64{}
		in.seen = map[int]time.Time{}
	}
	quit := in.quit
	in.mu.Unlock()

	in.collect()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				in.collect()
			}
		}
	}()
}

// Stop implements plugin.Instance.
func (in *instance) Stop() {
	in.mu.Lock()
	quit := in.quit
	in.started = false
	in.quit = nil
	in.mu.Unlock()
	if quit != nil {
		close(quit)
	}
}

// Poll implements plugin.Instance.
func (in *instance) Poll() { in.collect() }

// collect reads the process table once and publishes what it found.
func (in *instance) collect() {
	now := time.Now()
	procs, err := listProcesses()
	if err != nil {
		in.env.Publish(core.Snapshot{
			At:       now,
			Status:   core.StatusError,
			Title:    "Agent",
			Subtitle: fmt.Sprintf("读进程列表失败：%v", err),
			Err:      err.Error(),
		})
		return
	}

	extras := in.extraNames()
	matched := make([]Process, 0, 8)
	for _, p := range procs {
		name := agentName(p.Command, extras)
		if name == "" {
			continue
		}
		p.Agent = name
		matched = append(matched, p)
	}

	in.mu.Lock()
	activity := make(map[int]float64, len(matched))
	for _, p := range matched {
		prevActivity := in.activity[p.PID]
		delta := p.CPUSeconds - prevActivity
		if delta < 0 {
			delta = 0
		}
		activity[p.PID] = delta
		if _, ok := in.seen[p.PID]; !ok {
			in.seen[p.PID] = now
		}
		p.Started = in.seen[p.PID]
	}
	known := make(map[int]bool, len(matched))
	for _, p := range matched {
		known[p.PID] = true
	}
	for pid := range in.seen {
		if !known[pid] {
			delete(in.seen, pid)
		}
	}
	in.activity = activity
	in.mu.Unlock()

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Started.Equal(matched[j].Started) {
			return matched[i].PID < matched[j].PID
		}
		return matched[i].Started.Before(matched[j].Started)
	})

	hideIdle := in.env.Settings().Bool("hide_idle", false)
	busy := make([]Process, 0, len(matched))
	for _, p := range matched {
		active := activity[p.PID] > 0.05
		p.CPUSeconds = activity[p.PID]
		if !hideIdle || active {
			busy = append(busy, p)
		}
	}

	snap := core.Snapshot{At: now, Title: "Agent", Primary: "count"}
	switch {
	case len(matched) == 0:
		snap.Status = core.StatusOffline
		snap.Subtitle = "没有在运行的代理"
	case len(busy) == 0:
		snap.Status = core.StatusOK
		snap.Subtitle = fmt.Sprintf("%d 个在跑，都在空闲", len(matched))
	default:
		snap.Status = core.StatusOK
		names := make(map[string]int, len(busy))
		for _, p := range busy {
			names[p.Agent]++
		}
		parts := make([]string, 0, len(names))
		for name, n := range names {
			if n == 1 {
				parts = append(parts, name)
			} else {
				parts = append(parts, fmt.Sprintf("%s ×%d", name, n))
			}
		}
		sort.Strings(parts)
		snap.Subtitle = strings.Join(parts, " · ")
	}

	snap.Readings = append(snap.Readings,
		core.Reading{ID: "count", Label: "运行中", Value: float64(len(matched)), Unit: core.UnitNone,
			Severity: severityForCount(len(matched))},
		core.Reading{ID: "busy", Label: "在计算", Value: float64(len(busy)), Unit: core.UnitNone,
			Severity: core.SeverityNone, Detail: "刚用掉 CPU 就算"},
	)
	n := len(busy)
	if n > 4 {
		n = 4
	}
	for i := 0; i < n; i++ {
		p := busy[i]
		snap.Readings = append(snap.Readings, core.Reading{
			ID:       fmt.Sprintf("agent-%d", i+1),
			Label:    p.Agent,
			Text:     shortCommand(p.Command),
			Severity: core.SeverityNone,
			Detail:   fmt.Sprintf("PID %d · %s", p.PID, core.FormatDuration(now.Sub(p.Started))),
		})
	}
	if len(matched) > 0 {
		var cpu float64
		for _, p := range matched {
			cpu += activity[p.PID]
		}
		snap.Readings = append(snap.Readings, core.Reading{
			ID: "cpu", Label: "占用 CPU", Value: cpu, Unit: core.UnitSeconds,
			Severity: core.SeverityNone, Detail: "最近一次采样",
		})
	}
	in.env.Publish(snap)
}

func severityForCount(n int) core.Severity {
	switch {
	case n == 0:
		return core.SeverityOffline
	case n > 12:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

// extraNames reads the names the user added.
func (in *instance) extraNames() []string {
	raw := in.env.Settings().Get("extra")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// agentName says which agent a command line belongs to, or "" when it is
// not one this plugin watches.
func agentName(command string, extras []string) string {
	lower := strings.ToLower(command)
	for _, e := range extras {
		if strings.Contains(lower, e) {
			return e
		}
	}
	for _, k := range known {
		if containsWord(lower, k.Match) {
			return k.Name
		}
	}
	return ""
}

// containsWord looks for match at the start of a word of the command line,
// so that "codex" matches "codex exec" and "/usr/bin/codex" but not an
// argument that happens to contain it.
func containsWord(haystack, match string) bool {
	for i := 0; i+len(match) <= len(haystack); i++ {
		if haystack[i:i+len(match)] != match {
			continue
		}
		if i > 0 {
			prev := haystack[i-1]
			if isWordByte(prev) {
				continue
			}
		}
		return true
	}
	return false
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_' || b == '-'
}

// shortCommand keeps a command line short enough for a row.
func shortCommand(command string) string {
	command = strings.TrimSpace(command)
	if i := strings.IndexByte(command, '\x00'); i >= 0 {
		command = command[:i]
	}
	runes := []rune(command)
	if len(runes) <= 64 {
		return command
	}
	return string(runes[:64]) + "…"
}
