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

	// A file that was written but holds no instance is a user who removed
	// them all, and their choice stands across a restart. Only a machine that
	// never ran SeeAll — no file, so no version — starts with every plugin's
	// default monitor.
	if state.Version == 0 {
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
//
// A plugin the user removed earlier comes back with the settings it had: nil
// values take those the host saved when the monitor went away, so that
// removing one is not a way to forget how it was set up.
func (h *Host) Add(pluginID, name string, values map[string]string) *Instance {
	p := h.Plugin(pluginID)
	if p == nil {
		return nil
	}
	m := p.Manifest()
	if name == "" {
		name = m.DefaultInstance
	}
	restoring := false
	if values == nil {
		h.mu.RLock()
		values = h.saved.Removed[pluginID]
		restoring = values != nil
		h.mu.RUnlock()
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
	env := plugin.NewEnv(id, m, settings)
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
	// The monitor runs again, so the settings kept for it are no longer
	// waiting to be restored.
	if restoring {
		delete(h.saved.Removed, pluginID)
	}
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
	// Its cards are gone with it, so an order for them arranges nothing: a
	// monitor removed and added back starts from the plugin's own order.
	delete(h.saved.Order, id)
	// Keep how it was set up, so that adding it back restores the monitor
	// rather than a fresh one.
	if len(in.Manifest().Settings) > 0 {
		if h.saved.Removed == nil {
			h.saved.Removed = map[string]map[string]string{}
		}
		h.saved.Removed[in.PluginID] = in.Settings().All()
	}
	h.mu.Unlock()

	if in.monitor != nil {
		in.monitor.Stop()
	}
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

// Notifications reports whether the app may post system notifications. It is
// the switch the settings page shows, over the per-monitor ones: a monitor's
// own "提醒" setting says when it would notify, this says whether anything may
// appear at all.
func (h *Host) Notifications() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.saved.UI.Notify == nil {
		return true
	}
	return *h.saved.UI.Notify
}

// SetNotifications remembers whether notifications may be posted.
func (h *Host) SetNotifications(on bool) {
	h.mu.Lock()
	if h.saved.UI.Notify != nil && *h.saved.UI.Notify == on {
		h.mu.Unlock()
		return
	}
	h.saved.UI.Notify = &on
	h.mu.Unlock()
	h.save()
	h.OnChange()
}

// Notify posts a system notification, unless the user turned them off.
func (h *Host) Notify(title, body string) {
	if !h.Notifications() {
		return
	}
	h.OnNotify(title, body)
}

func (h *Host) notifyInstance(title, body string) { h.Notify(title, body) }

// now is time.Now, replaceable by tests.
var now = time.Now

// ---- settings file ----

type fileState struct {
	Version   int          `json:"version"`
	Instances []savedEntry `json:"instances"`
	UI        savedUI      `json:"ui"`
	// Removed holds the settings of the monitors the user took away, by
	// plugin: adding one back goes through Add, which reads them.
	Removed map[string]map[string]string `json:"removed,omitempty"`
	// Order holds the order a page's cards were dragged into, by instance:
	// the reading IDs as the user arranged them. A reading a plugin stops
	// reporting is simply absent, and one it starts reporting is appended.
	Order map[string][]string `json:"order,omitempty"`
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
	// Notify is the master switch for system notifications; nil on a file
	// written before it existed, which means on.
	Notify *bool `json:"notify,omitempty"`
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
	state.Removed = h.saved.Removed
	state.Order = h.saved.Order
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

// CardOrder returns the order the cards of an instance's page were dragged
// into: reading IDs, most wanted first. An instance the user has not arranged
// has none, and its page keeps the plugin's own order.
func (h *Host) CardOrder(instanceID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), h.saved.Order[instanceID]...)
}

// SetCardOrder remembers the order of an instance's cards.
func (h *Host) SetCardOrder(instanceID string, order []string) {
	h.mu.Lock()
	if h.saved.Order == nil {
		h.saved.Order = map[string][]string{}
	}
	h.saved.Order[instanceID] = append([]string(nil), order...)
	h.mu.Unlock()
	h.save()
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
