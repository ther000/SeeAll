// Package app runs the plugins, keeps what they report and remembers the
// user's setup on disk.
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// Instance is one running monitor the app keeps: the mouse, the system.
type Instance struct {
	// ID is unique across the app: "mouse", "system".
	ID string
	// PluginID names the plugin that made it.
	PluginID string
	// Name is what the sidebar shows.
	Name string
	// Env is the instance's connection to the app.
	Env *plugin.Env

	mu      sync.RWMutex
	monitor plugin.Instance
	snap    core.Snapshot
	history map[string]*core.Ring
	lastErr string
}

// Snapshot returns the instance's newest snapshot.
func (in *Instance) Snapshot() core.Snapshot {
	in.mu.RLock()
	defer in.mu.RUnlock()
	return in.snap
}

// History returns the values kept for one reading.
func (in *Instance) History(readingID string) []float64 {
	in.mu.RLock()
	ring := in.history[readingID]
	in.mu.RUnlock()
	if ring == nil {
		return nil
	}
	return ring.Values()
}

// Manifest returns the plugin's manifest.
func (in *Instance) Manifest() plugin.Manifest { return in.Env.Manifest() }

// Settings returns the instance's settings.
func (in *Instance) Settings() *plugin.Settings { return in.Env.Settings() }

// Host owns the plugins, the instances and the settings file.
type Host struct {
	mu        sync.RWMutex
	plugins   []plugin.Plugin
	instances []*Instance
	byID      map[string]*Instance
	storePath string
	saved     fileState

	// OnChange runs after a snapshot or a setting changes, on the goroutine
	// that changed it. The UI sets it to repaint.
	OnChange func()
	// OnNotify posts a system notification; the app sets it once it is
	// ready, because notifications need the desktop.
	OnNotify func(title, body string)
	// Logf writes a line to the log file.
	Logf func(format string, args ...any)
}

// NewHost makes a host with the built-in plugins, reading the settings file
// in dir.
func NewHost(dir string) *Host {
	h := &Host{
		byID:      map[string]*Instance{},
		storePath: filepath.Join(dir, "seeall.json"),
		OnChange:  func() {},
		OnNotify:  func(string, string) {},
		Logf:      func(string, ...any) {},
	}
	h.plugins = Builtin()
	return h
}

// Plugins returns the plugins the app knows.
func (h *Host) Plugins() []plugin.Plugin {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]plugin.Plugin, len(h.plugins))
	copy(out, h.plugins)
	return out
}

// Plugin finds a plugin by ID.
func (h *Host) Plugin(id string) plugin.Plugin {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, p := range h.plugins {
		if p.Manifest().ID == id {
			return p
		}
	}
	return nil
}

// Instances returns the running instances, in sidebar order.
func (h *Host) Instances() []*Instance {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Instance, len(h.instances))
	copy(out, h.instances)
	return out
}

// Instance finds one by ID.
func (h *Host) Instance(id string) *Instance {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.byID[id]
}

// Load reads the settings file and starts every instance in it, or the
// plugins' default instances on a machine that has none yet.
func (h *Host) Load() error {
	state, err := h.readState()
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.saved = state
	h.mu.Unlock()

	h.OnNotify = func(title, body string) { h.Notify(title, body) }

	if len(state.Instances) == 0 {
		for _, p := range h.Plugins() {
			m := p.Manifest()
			h.Add(m.ID, m.DefaultInstance, nil)
		}
	} else {
		for _, saved := range state.Instances {
			p := h.Plugin(saved.PluginID)
			if p == nil {
				continue
			}
			h.Add(saved.PluginID, saved.Name, saved.Settings)
		}
	}
	return nil
}

// Add makes and starts an instance.
func (h *Host) Add(pluginID, name string, values map[string]string) *Instance {
	p := h.Plugin(pluginID)
	if p == nil {
		return nil
	}
	m := p.Manifest()
	if name == "" {
		name = m.DefaultInstance
	}
	id := m.ID
	if m.Multiple {
		id = m.ID + ":" + strings.TrimSpace(name)
	}

	h.mu.RLock()
	if _, exists := h.byID[id]; exists {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	settings := plugin.NewSettings(m.Settings)
	if values != nil {
		settings.Load(values)
	}
	env := plugin.NewEnv(id, name, m, settings)
	in := &Instance{ID: id, PluginID: m.ID, Name: name, Env: env, history: map[string]*core.Ring{}}
	env.SetLog(h.Logf)
	env.SetNotify(h.notifyInstance)
	env.SetPublishHook(h.storeSnapshot)
	in.monitor = p.New(env)
	env.Settings().OnChange(func(key, value string) {
		h.save()
		if in.monitor != nil {
			in.monitor.Poll()
		}
		h.OnChange()
	})

	h.mu.Lock()
	h.instances = append(h.instances, in)
	h.byID[id] = in
	h.mu.Unlock()

	if in.monitor != nil {
		in.monitor.Start()
	}
	h.save()
	h.OnChange()
	return in
}

// Remove stops an instance and forgets it.
func (h *Host) Remove(id string) {
	h.mu.Lock()
	in := h.byID[id]
	if in == nil {
		h.mu.Unlock()
		return
	}
	delete(h.byID, id)
	for i, candidate := range h.instances {
		if candidate == in {
			h.instances = append(h.instances[:i], h.instances[i+1:]...)
			break
		}
	}
	h.mu.Unlock()

	if in.monitor != nil {
		in.monitor.Stop()
	}
	in.Env.Stop()
	h.save()
	h.OnChange()
}

// Refresh asks every instance for a snapshot now.
func (h *Host) Refresh() {
	for _, in := range h.Instances() {
		if in.monitor != nil {
			go in.monitor.Poll()
		}
	}
}

// RefreshOne asks one instance for a snapshot now.
func (h *Host) RefreshOne(id string) {
	if in := h.Instance(id); in != nil && in.monitor != nil {
		go in.monitor.Poll()
	}
}

// Stop ends every instance, as quitting does.
func (h *Host) Stop() {
	for _, in := range h.Instances() {
		if in.monitor != nil {
			in.monitor.Stop()
		}
		in.Env.Stop()
	}
}

// storeSnapshot keeps a snapshot and its history, then repaints the panel.
func (h *Host) storeSnapshot(instanceID string, snap core.Snapshot) {
	in := h.Instance(instanceID)
	if in == nil {
		return
	}
	in.mu.Lock()
	in.snap = snap
	in.lastErr = snap.Err
	if in.history == nil {
		in.history = map[string]*core.Ring{}
	}
	for _, r := range snap.Readings {
		if r.Text != "" {
			continue
		}
		ring := in.history[r.ID]
		if ring == nil {
			ring = core.NewRing(120)
			in.history[r.ID] = ring
		}
		ring.Push(r.Value)
	}
	in.mu.Unlock()
	h.OnChange()
}

// Notify posts a system notification.
func (h *Host) Notify(title, body string) { h.OnNotify(title, body) }

func (h *Host) notifyInstance(title, body string) { h.Notify(title, body) }

// now is time.Now, replaceable by tests.
var now = time.Now

// Summaries returns one line per instance, for the overview and the tray
// menu.
func (h *Host) Summaries() []Summary {
	instances := h.Instances()
	out := make([]Summary, 0, len(instances))
	for _, in := range instances {
		snap := in.Snapshot()
		out = append(out, Summary{
			ID:       in.ID,
			Name:     in.Name,
			Kind:     in.Manifest().Kind,
			Status:   snap.Status,
			Headline: headline(snap),
		})
	}
	return out
}

// Summary is one instance as the overview lists it.
type Summary struct {
	ID       string
	Name     string
	Kind     plugin.Kind
	Status   core.Status
	Headline string
}

// headline picks the one reading that says the most about an instance.
func headline(snap core.Snapshot) string {
	if len(snap.Readings) == 0 {
		if snap.Subtitle != "" {
			return snap.Subtitle
		}
		return "还没有数据"
	}
	best := snap.Readings[0]
	rank := readingRank(best)
	for _, r := range snap.Readings[1:] {
		if readingRank(r) < rank {
			best = r
			rank = readingRank(r)
		}
	}
	if best.Format() == "" {
		return snap.Subtitle
	}
	return best.Label + " " + best.Format()
}

// readingRank orders readings by how much they say: warnings first, then
// values, then words.
func readingRank(r core.Reading) int {
	rank := 10
	switch r.Severity {
	case core.SeverityCritical:
		rank = 0
	case core.SeverityWarn:
		rank = 1
	case core.SeverityOffline:
		rank = 2
	case core.SeverityOK:
		rank = 3
	case core.SeverityInfo:
		rank = 4
	}
	if r.Text != "" {
		rank += 5
	}
	return rank
}

// Kinds returns the plugin kinds in sidebar order.
func Kinds(instances []*Instance) []plugin.Kind {
	seen := map[plugin.Kind]bool{}
	var out []plugin.Kind
	for _, order := range []plugin.Kind{plugin.KindDevice, plugin.KindSystem} {
		for _, in := range instances {
			if in.Manifest().Kind == order && !seen[order] {
				seen[order] = true
				out = append(out, order)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return false })
	return out
}

// ---- settings file ----

type fileState struct {
	Version   int          `json:"version"`
	Instances []savedEntry `json:"instances"`
	UI        savedUI      `json:"ui"`
}

type savedEntry struct {
	PluginID string            `json:"plugin"`
	Name     string            `json:"name"`
	Settings map[string]string `json:"settings"`
}

type savedUI struct {
	LastInstance string `json:"lastInstance"`
	StartAtLogin *bool  `json:"startAtLogin"`
}

func (h *Host) readState() (fileState, error) {
	var state fileState
	data, err := os.ReadFile(h.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		// A corrupt file is renamed rather than thrown away, so the user
		// can look at it.
		_ = os.Rename(h.storePath, h.storePath+".broken")
		return fileState{}, nil
	}
	return state, nil
}

// save writes the settings file. It runs on whichever goroutine changed
// something, and never blocks the UI for long: the file is a few hundred
// bytes.
func (h *Host) save() {
	state := fileState{Version: 1}
	h.mu.RLock()
	state.UI = h.saved.UI
	for _, in := range h.instances {
		state.Instances = append(state.Instances, savedEntry{
			PluginID: in.PluginID,
			Name:     in.Name,
			Settings: in.Env.Settings().All(),
		})
	}
	h.mu.RUnlock()
	state.Instances = append([]savedEntry(nil), state.Instances...)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.storePath), 0o755); err != nil {
		return
	}
	tmp := h.storePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, h.storePath)
}

// StartAtLogin reports whether the app starts with Windows.
func (h *Host) StartAtLogin() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.saved.UI.StartAtLogin == nil {
		return true
	}
	return *h.saved.UI.StartAtLogin
}

// SetStartAtLogin remembers the choice.
func (h *Host) SetStartAtLogin(on bool) {
	h.mu.Lock()
	h.saved.UI.StartAtLogin = &on
	h.mu.Unlock()
	h.save()
}

// LastInstance returns the instance the panel showed last, or "".
func (h *Host) LastInstance() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.saved.UI.LastInstance
}

// SetLastInstance remembers which instance the panel showed.
func (h *Host) SetLastInstance(id string) {
	h.mu.Lock()
	if h.saved.UI.LastInstance == id {
		h.mu.Unlock()
		return
	}
	h.saved.UI.LastInstance = id
	h.mu.Unlock()
	h.save()
}

// String names a snapshot for the log.
func (in *Instance) String() string {
	snap := in.Snapshot()
	return fmt.Sprintf("%s(%s): %s", in.Name, in.ID, snap.Status)
}
