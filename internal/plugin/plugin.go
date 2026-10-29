// Package plugin defines what a SeeAll plugin is and how the app runs it.
//
// A plugin is a Go package that declares a manifest and makes instances: a
// monitor of one thing, such as a mouse's battery or the load on this
// computer. SeeAll ships two of them (mouse, system), and a new one only has
// to implement these interfaces to show up in the app.
package plugin

import (
	"sync"
	"time"

	"seeall/internal/core"
)

// Kind groups plugins the way the sidebar does.
type Kind string

const (
	// KindDevice watches hardware the user carries: mice, dongles.
	KindDevice Kind = "device"
	// KindSystem watches the computer SeeAll runs on.
	KindSystem Kind = "system"
	// KindNetwork watches machines and services elsewhere.
	KindNetwork Kind = "network"
)

// Order sorts the kinds the way the sidebar does: the hardware first, then
// the machine it runs on, then anything else a plugin adds.
func (k Kind) Order() int {
	switch k {
	case KindDevice:
		return 0
	case KindSystem:
		return 1
	}
	return 2
}

// String names the kind for people.
func (k Kind) String() string {
	switch k {
	case KindDevice:
		return "设备"
	case KindSystem:
		return "本机"
	case KindNetwork:
		return "服务"
	}
	return string(k)
}

// Manifest describes a plugin to the app and to the settings page.
type Manifest struct {
	// ID is the plugin's stable name, such as "mouse".
	ID string
	// Name is what the sidebar shows: "鼠标".
	Name string
	// Summary says what it watches, in one line.
	Summary string
	// Kind groups it.
	Kind Kind
	// Icon names an SVG in the app's icon set, drawn in the sidebar.
	Icon string
	// Settings are the fields the settings page offers for an instance.
	Settings []Setting
	// DefaultInstance is the name of the instance every install starts
	// with: "鼠标", "本机".
	DefaultInstance string
	// Combined reads a plugin's page as one card: the readings of one device
	// belong together, so they go in the hero card under the value rather
	// than as cards of their own beside it.
	Combined bool
	// Standalone says the instance is a page of its own rather than a card
	// on the dashboard: it does things — Mole's cleanup — instead of watching
	// something, so it opens from the tray and the header. It keeps its line
	// in the tray menu like any other monitor; it has no settings card because
	// it declares no settings.
	Standalone bool
}

// Plugin creates instances of one kind of monitor.
type Plugin interface {
	// Manifest describes the plugin.
	Manifest() Manifest
	// New makes a fresh instance. It must not start watching yet; the host
	// calls Start on the main thread once the instance exists.
	New(env *Env) Instance
}

// Instance is one monitor the app runs: the mouse, the system.
type Instance interface {
	// Start begins watching. It returns at once and keeps working in
	// goroutines until Stop, reporting through Env.
	Start()
	// Stop ends watching; it returns once the instance's goroutines are
	// done.
	Stop()
	// Poll asks for a fresh snapshot now, as the Refresh button and the
	// host's timer do. It may run on any goroutine and must not block on
	// the UI.
	Poll()
}

// Env is everything an instance gets from the app: its settings, the way
// to publish snapshots and to reach the user.
type Env struct {
	// InstanceID is unique across the app.
	InstanceID string

	manifest Manifest
	settings *Settings

	mu     sync.RWMutex
	snap   core.Snapshot
	logf   func(string, ...any)
	onSnap func(instanceID string, snap core.Snapshot)
}

// NewEnv makes an environment for one instance.
func NewEnv(instanceID string, m Manifest, settings *Settings) *Env {
	return &Env{
		InstanceID: instanceID,
		manifest:   m,
		settings:   settings,
		logf:       func(string, ...any) {},
		onSnap:     func(string, core.Snapshot) {},
	}
}

// Manifest returns the plugin's manifest.
func (e *Env) Manifest() Manifest { return e.manifest }

// Settings returns the instance's settings.
func (e *Env) Settings() *Settings { return e.settings }

// Logf writes a line to SeeAll's log.
func (e *Env) Logf(format string, args ...any) { e.logf(format, args...) }

// Publish replaces the instance's snapshot and repaints the panel. It is
// safe from any goroutine and is what a plugin's polling loop calls.
func (e *Env) Publish(snap core.Snapshot) {
	if snap.SourceID == "" {
		snap.SourceID = e.InstanceID
	}
	if snap.At.IsZero() {
		snap.At = time.Now()
	}
	e.mu.Lock()
	e.snap = snap
	e.mu.Unlock()
	e.onSnap(e.InstanceID, snap)
}

// Snapshot returns the last published snapshot.
func (e *Env) Snapshot() core.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.snap
}

// SetLog sets where Logf writes.
func (e *Env) SetLog(fn func(string, ...any)) {
	if fn != nil {
		e.logf = fn
	}
}

// SetPublishHook sets what runs when a snapshot arrives.
func (e *Env) SetPublishHook(fn func(string, core.Snapshot)) {
	if fn != nil {
		e.onSnap = fn
	}
}
