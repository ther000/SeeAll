// Package keyboard watches the battery of a wireless keyboard.
//
// Unlike the mouse, whose vendor protocol is known, keyboards report their
// charge in more than one way. This plugin reads the two common ones:
//
//   - a Generic Device Controls collection (usage page 0x0006, usage 0x0020
//     Battery Strength), whose first input report byte is the level;
//   - a Rapoo state broadcast on the vendor page, the same frame the mouse
//     plugin parses.
//
// The plugin is marked experimental: the paths are implemented and run, but
// only the Rapoo frame is verified against hardware.
package keyboard

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"seeall/internal/core"
	"seeall/internal/hid"
	"seeall/internal/plugin"
)

const (
	// batteryUsagePage is Generic Device Controls, where HID puts battery
	// strength.
	batteryUsagePage uint16 = 0x0006
	// batteryUsage is the Battery Strength usage.
	batteryUsage uint16 = 0x0020
	// vendorUsagePage is the Rapoo vendor page.
	vendorUsagePage uint16 = 0xFF00
	// vendorUsage is the Rapoo state-broadcast usage.
	vendorUsage uint16 = 0x0002
)

// Manifest describes the plugin to the app.
func Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "keyboard",
		Name:            "键盘",
		Summary:         "无线键盘的电量，按 HID 电池字段读取",
		Kind:            plugin.KindDevice,
		Icon:            "keyboard",
		DefaultInstance: "键盘",
		Interval:        5 * time.Second,
		Experimental:    true,
		Settings: []plugin.Setting{
			{
				Key: "vid", Label: "USB 厂商号", Type: plugin.SettingText, Default: "",
				Placeholder: "24AE",
				Description: "留空就监听所有键盘；填了就只认这家厂商",
			},
		},
	}
}

// Plugin is the keyboard monitor.
type Plugin struct{}

// Manifest implements plugin.Plugin.
func (Plugin) Manifest() plugin.Manifest { return Manifest() }

// New implements plugin.Plugin.
func (Plugin) New(env *plugin.Env) plugin.Instance { return &instance{env: env} }

type channel struct {
	reader *hid.Reader
	kind   string // "battery" or "vendor"
	path   string
	name   string
}

type instance struct {
	env *plugin.Env

	mu       sync.Mutex
	chans    []*channel
	level    int
	hasLevel bool
	device   string
	source   string
	lastAt   time.Time
	problem  string
	started  bool
	quit     chan struct{}
	wg       sync.WaitGroup
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
	in.mu.Unlock()
	in.wg.Add(1)
	go in.loop(in.quit)
}

// Stop implements plugin.Instance.
func (in *instance) Stop() {
	in.mu.Lock()
	quit := in.quit
	in.started = false
	in.quit = nil
	in.mu.Unlock()
	if quit == nil {
		return
	}
	close(quit)
	in.wg.Wait()
	in.closeAll()
}

// Poll implements plugin.Instance.
func (in *instance) Poll() { in.publish() }

func (in *instance) loop(quit <-chan struct{}) {
	defer in.wg.Done()
	in.discover()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			in.discover()
		}
	}
}

// discover opens the keyboard collections it finds. Keyboards the system
// owns cannot be opened; those are skipped, and the plugin says so once.
func (in *instance) discover() {
	vendor := in.vendorFilter()
	intfs, _ := hid.Enumerate(func(i hid.Interface) bool {
		if vendor != 0 && i.VendorID != vendor {
			return false
		}
		if i.UsagePage == batteryUsagePage && i.Usage == batteryUsage {
			return true
		}
		if i.UsagePage == vendorUsagePage && i.Usage == vendorUsage && i.InputReportByteLength >= 7 {
			return true
		}
		return false
	})

	in.mu.Lock()
	known := make(map[string]bool, len(in.chans))
	for _, ch := range in.chans {
		known[ch.path] = true
	}
	in.mu.Unlock()

	for _, intf := range intfs {
		if known[intf.Path] {
			continue
		}
		reader, err := hid.OpenReader(intf)
		if err != nil {
			in.setProblem(fmt.Sprintf("打不开 %s（%v）", intf.Product, err))
			continue
		}
		kind := "battery"
		if intf.UsagePage == vendorUsagePage {
			kind = "vendor"
		}
		name := strings.TrimSpace(intf.Product)
		if name == "" {
			name = "无线键盘"
		}
		ch := &channel{reader: reader, kind: kind, path: intf.Path, name: name}
		in.mu.Lock()
		in.chans = append(in.chans, ch)
		in.mu.Unlock()
		in.wg.Add(1)
		go in.read(ch)
	}

	in.mu.Lock()
	count := len(in.chans)
	in.mu.Unlock()
	if count == 0 {
		in.setProblem("没找到键盘的电池通道")
	}
}

func (in *instance) vendorFilter() uint16 {
	text := strings.TrimSpace(strings.ToLower(in.env.Settings().Get("vid")))
	text = strings.TrimPrefix(text, "0x")
	if text == "" {
		return 0
	}
	v, err := strconv.ParseUint(text, 16, 16)
	if err != nil {
		return 0
	}
	return uint16(v)
}

func (in *instance) read(ch *channel) {
	defer in.wg.Done()
	for {
		in.mu.Lock()
		quit := in.quit
		in.mu.Unlock()
		if quit == nil {
			return
		}
		select {
		case <-quit:
			return
		default:
		}

		report, err := ch.reader.Read(500 * time.Millisecond)
		if err != nil {
			in.drop(ch)
			return
		}
		if report == nil {
			continue
		}
		if ch.kind == "vendor" {
			if frame, ok := parseVendor(report); ok {
				in.setLevel(frame, ch.name, "状态帧")
			}
			continue
		}
		if len(report) == 0 {
			continue
		}
		if level := int(report[0]); level >= 0 && level <= 100 {
			in.setLevel(level, ch.name, "电池字段")
		}
	}
}

func parseVendor(report []byte) (int, bool) {
	if len(report) < 7 || report[0] != 0xBB || report[1] != 0xB0 {
		return 0, false
	}
	level := int(report[6])
	if level < 0 || level > 100 {
		return 0, false
	}
	return level, true
}

func (in *instance) setLevel(level int, name, source string) {
	in.mu.Lock()
	changed := !in.hasLevel || in.level != level
	in.level = level
	in.hasLevel = true
	in.device = name
	in.source = source
	in.lastAt = time.Now()
	in.problem = ""
	in.mu.Unlock()
	if changed {
		in.publish()
	}
}

func (in *instance) setProblem(text string) {
	in.mu.Lock()
	same := in.problem == text
	in.problem = text
	hasLevel := in.hasLevel
	in.mu.Unlock()
	if !same || !hasLevel {
		in.publish()
	}
}

func (in *instance) drop(ch *channel) {
	ch.reader.Close()
	in.mu.Lock()
	defer in.mu.Unlock()
	for i, c := range in.chans {
		if c == ch {
			in.chans = append(in.chans[:i], in.chans[i+1:]...)
			return
		}
	}
}

func (in *instance) closeAll() {
	in.mu.Lock()
	chans := in.chans
	in.chans = nil
	in.mu.Unlock()
	for _, ch := range chans {
		ch.reader.Close()
	}
}

func (in *instance) publish() {
	in.mu.Lock()
	level, hasLevel, device, source, lastAt, problem := in.level, in.hasLevel, in.device, in.source, in.lastAt, in.problem
	in.mu.Unlock()

	now := time.Now()
	snap := core.Snapshot{At: now, Title: "无线键盘", Primary: "battery"}
	if !hasLevel {
		snap.Status = core.StatusOffline
		snap.Subtitle = problem
		if snap.Subtitle == "" {
			snap.Subtitle = "等待键盘上报电量"
		}
		in.env.Publish(snap)
		return
	}

	age := now.Sub(lastAt)
	stale := age > 15*time.Second
	snap.Title = device
	switch {
	case stale:
		snap.Status = core.StatusOffline
		snap.Subtitle = core.FormatDuration(age) + "没有更新"
	case level <= 30:
		snap.Status = core.StatusWarn
		snap.Subtitle = fmt.Sprintf("%d%% · 该充电了", level)
	default:
		snap.Status = core.StatusOK
		snap.Subtitle = fmt.Sprintf("%d%%", level)
	}

	severity := core.SeverityOK
	switch {
	case stale:
		severity = core.SeverityOffline
	case level <= 10:
		severity = core.SeverityCritical
	case level <= 30:
		severity = core.SeverityWarn
	}
	snap.Readings = []core.Reading{
		{ID: "battery", Label: "电量", Value: float64(level), Unit: core.UnitPercent, Severity: severity,
			Detail: core.FormatDuration(age)},
		{ID: "source", Label: "来源", Text: source, Severity: core.SeverityNone, Detail: device},
	}
	in.env.Publish(snap)
}
