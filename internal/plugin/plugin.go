// Package plugin defines what a SeeAll plugin is and how the app runs it.
//
// A plugin is a Go package that declares a manifest and makes instances: a
// monitor of one thing, such as a mouse's battery or the load on this
// computer. SeeAll ships two of them (mouse, system), and a new one only has
// to implement these interfaces to show up in the app.
package plugin

import (
	"context"
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
	// KindAgent watches the AI agents running on this computer.
	KindAgent Kind = "agent"
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
	case KindAgent:
		return "Agent"
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
	// Multiple lets a plugin have several instances, one per thing it
	// watches. Without it, the app keeps a single instance.
	Multiple bool
	// DefaultInstance is the name of the instance every install starts
	// with: "鼠标", "本机".
	DefaultInstance string
	// Interval is how often the host polls the instance; zero means the
	// plugin's own timer drives it.
	Interval time.Duration
	// Experimental marks a plugin whose protocol is not verified on real
	// hardware; the UI says so instead of pretending.
	Experimental bool
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
	// InstanceID is unique across the app: "mouse", "server:github".
	InstanceID string
	// Name is what the sidebar shows for the instance.
	Name string

	manifest Manifest
	settings *Settings

	mu     sync.RWMutex
	snap   core.Snapshot
	status string
	logf   func(string, ...any)
	notify func(title, body string)
	onSnap func(instanceID string, snap core.Snapshot)
	cancel context.CancelFunc
	ctx    context.Context
}

// NewEnv makes an environment for one instance.
func NewEnv(instanceID, name string, m Manifest, settings *Settings) *Env {
	ctx, cancel := context.WithCancel(context.Background())
	return &Env{
		InstanceID: instanceID,
		Name:       name,
		manifest:   m,
		settings:   settings,
		ctx:        ctx,
		cancel:     cancel,
		logf:       func(string, ...any) {},
		notify:     func(string, string) {},
		onSnap:     func(string, core.Snapshot) {},
	}
}

// Manifest returns the plugin's manifest.
func (e *Env) Manifest() Manifest { return e.manifest }

// Settings returns the instance's settings.
func (e *Env) Settings() *Settings { return e.settings }

// Context is cancelled when the instance stops, so a plugin may pass it to
// HTTP requests and its own loops.
func (e *Env) Context() context.Context { return e.ctx }

// Logf writes a line to SeeAll's log.
func (e *Env) Logf(format string, args ...any) { e.logf(format, args...) }

// Notify posts a system notification, if the user allows them.
func (e *Env) Notify(title, body string) { e.notify(title, body) }

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

// SetStatus records what the instance is doing, for the subtitle of its page
// while it has no snapshot yet.
func (e *Env) SetStatus(text string) {
	e.mu.Lock()
	e.status = text
	e.mu.Unlock()
}

// Snapshot returns the last published snapshot.
func (e *Env) Snapshot() core.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.snap
}

// Status returns the last SetStatus text.
func (e *Env) Status() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// Stop cancels the instance's context.
func (e *Env) Stop() { e.cancel() }

// SetLog sets where Logf writes.
func (e *Env) SetLog(fn func(string, ...any)) {
	if fn != nil {
		e.logf = fn
	}
}

// SetNotify sets what Notify posts.
func (e *Env) SetNotify(fn func(title, body string)) {
	if fn != nil {
		e.notify = fn
	}
}

// SetPublishHook sets what runs when a snapshot arrives.
func (e *Env) SetPublishHook(fn func(string, core.Snapshot)) {
	if fn != nil {
		e.onSnap = fn
	}
}

// Closed reports whether the instance has stopped.
func (e *Env) Closed() bool {
	select {
	case <-e.ctx.Done():
		return true
	default:
		return false
	}
}
