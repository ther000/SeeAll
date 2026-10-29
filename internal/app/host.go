// Package app runs the plugins, keeps what they report and remembers the
// user's setup on disk.
package app

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// Monitor returns the plugin's own instance, which is what a page the plugin
// draws for itself reads from: the dashboard only needs snapshots, but the
// Mole page asks its instance what it has run.
func (in *Instance) Monitor() plugin.Instance { return in.monitor }

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
	// adoptLegacy lets Load take over a pre-move settings file from the
	// per-user folder. It is off unless the app asks: a host pointed at any
	// other folder — a test's temporary one — must see only what is there.
	adoptLegacy bool

	// OnChange runs after a snapshot or a setting changes, on the goroutine
	// that changed it. The UI sets it to repaint.
	OnChange func()
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

// Load reads the settings file and starts every plugin. There is nothing to
// add and nothing to remove: the panel always shows everything the app can
// watch, so the file carries the settings each plugin was given rather than a
// list of which ones to run.
func (h *Host) Load() error {
	state, err := h.readState()
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.saved = state
	h.mu.Unlock()

	for _, p := range h.Plugins() {
		m := p.Manifest()
		h.start(p, m.DefaultInstance, settingsOf(state, m.ID))
	}
	return nil
}

// settingsOf returns the settings a plugin was saved with, or nil for one the
// file has never seen.
func settingsOf(state fileState, pluginID string) map[string]string {
	for _, saved := range state.Instances {
		if saved.PluginID == pluginID {
			return saved.Settings
		}
	}
	return nil
}

// start makes and runs one plugin's instance. There is one per plugin: a
// monitor is part of the app rather than something the user arranges, so its
// name comes from the manifest and its settings from the file.
func (h *Host) start(p plugin.Plugin, name string, values map[string]string) *Instance {
	m := p.Manifest()
	if name == "" {
		name = m.DefaultInstance
	}
	settings := plugin.NewSettings(m.Settings)
	if values != nil {
		settings.Load(values)
	}
	env := plugin.NewEnv(m.ID, m, settings)
	in := &Instance{ID: m.ID, PluginID: m.ID, Name: name, Env: env, history: map[string]*core.Ring{}}
	env.SetLog(h.Logf)
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
	h.byID[m.ID] = in
	h.mu.Unlock()

	if in.monitor != nil {
		in.monitor.Start()
	}
	h.save()
	h.OnChange()
	return in
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
	if in.history == nil {
		in.history = map[string]*core.Ring{}
	}
	for _, r := range snap.Readings {
		// A reading made of words has no series to keep — "无线" does not
		// move. One that carries both a number and its wording does: the
		// network's rate is a number of bytes per second written as "1.7
		// KB/s", and the card's chart has nothing to draw without it.
		if r.Text != "" && r.Value == 0 {
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

// now is time.Now, replaceable by tests.
var now = time.Now

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
	// Opacity is how solid the panel is, from MinPanelOpacity to 1; nil on a
	// file written before the slider existed, which means opaque.
	Opacity *float64 `json:"opacity,omitempty"`
}

func (h *Host) readState() (fileState, error) {
	var state fileState
	data, err := os.ReadFile(h.storePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return state, err
		}
		if h.adoptLegacy {
			return h.adoptLegacyState()
		}
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		// A corrupt file is renamed rather than thrown away, so the user
		// can look at it.
		_ = os.Rename(h.storePath, h.storePath+".broken")
		return fileState{}, nil
	}
	return state, nil
}

// userConfigDir is os.UserConfigDir, replaceable by tests that must not touch
// the machine's real one.
var userConfigDir = os.UserConfigDir

// AdoptLegacySettings turns on the one-time move of a settings file written by
// a build that kept its files in the per-user folder. Only the real app calls
// it: a host made for a test must not read the machine's own settings.
func (h *Host) AdoptLegacySettings() { h.adoptLegacy = true }

// adoptLegacyState reads the settings a build that kept its files in the
// per-user folder left there, and marks that file as taken.
//
// It runs only when the app's own file is missing, and it moves the old one
// aside as it reads it. Marking it is what makes this a move rather than a
// fallback: a user who empties their panel on purpose must not have the old
// file's monitors come back at the next launch, and only a file that has not
// been taken yet can be a first-run migration.
func (h *Host) adoptLegacyState() (fileState, error) {
	var state fileState
	legacy := h.legacyStorePath()
	if legacy == "" {
		return state, nil
	}
	data, err := os.ReadFile(legacy)
	if err != nil {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fileState{}, nil
	}
	// Taken. A file that cannot be moved is left alone rather than read
	// again next launch: the settings already came across.
	_ = os.Rename(legacy, legacy+".migrated")
	return state, nil
}

// legacyStorePath is where a build that kept its files in the per-user folder
// wrote the settings, or "" when that is the same place the app uses now.
func (h *Host) legacyStorePath() string {
	userConfig, err := userConfigDir()
	if err != nil {
		return ""
	}
	old := filepath.Join(userConfig, "SeeAll", "seeall.json")
	if samePath(old, h.storePath) {
		return ""
	}
	return old
}

// samePath reports whether two paths name the same file, allowing for the
// slashes and letter case Windows treats as the same.
func samePath(a, b string) bool {
	clean := func(p string) string {
		return strings.ToLower(filepath.ToSlash(filepath.Clean(p)))
	}
	return clean(a) == clean(b)
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

// MinPanelOpacity is how faint the panel may get. Below it the text stops
// being readable, and a panel the user cannot read is worse than one they
// cannot see through.
const MinPanelOpacity = 0.4

// PanelOpacity returns how solid the panel is, from MinPanelOpacity to 1.
func (h *Host) PanelOpacity() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.saved.UI.Opacity == nil {
		return 1
	}
	return clampOpacity(*h.saved.UI.Opacity)
}

// SetPanelOpacity remembers how solid the panel is.
func (h *Host) SetPanelOpacity(v float64) {
	h.mu.Lock()
	value := clampOpacity(v)
	if h.saved.UI.Opacity != nil && *h.saved.UI.Opacity == value {
		h.mu.Unlock()
		return
	}
	h.saved.UI.Opacity = &value
	h.mu.Unlock()
	h.save()
	h.OnChange()
}

// ClampOpacityForTest is clampOpacity, for the panel's tests to check the
// floor a slider cannot cross.
func ClampOpacityForTest(v float64) float64 { return clampOpacity(v) }

// clampOpacity keeps a panel solid enough to read.
func clampOpacity(v float64) float64 {
	if v < MinPanelOpacity {
		return MinPanelOpacity
	}
	if v > 1 {
		return 1
	}
	return v
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
