package mouse

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

// Manifest describes the plugin to the app.
func Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "mouse",
		Name:            "鼠标",
		Summary:         "无线鼠标的电量、连接方式与 DPI",
		Kind:            plugin.KindDevice,
		Icon:            "mouse",
		DefaultInstance: "鼠标",
		// Battery, link and DPI are one mouse's readings: its page shows them
		// in one card.
		Combined: true,
		Settings: []plugin.Setting{
			{
				Key: "vid", Label: "USB 厂商号", Type: plugin.SettingText, Default: "24AE",
				Placeholder: "24AE",
				Description: "雷柏是 24AE，换别的品牌就填它的 VID（十六进制）",
			},
			{Key: "notify", Label: "低电量提醒", Type: plugin.SettingBool, Default: "true"},
		},
	}
}

// Plugin is the mouse monitor.
type Plugin struct{}

// Manifest implements plugin.Plugin.
func (Plugin) Manifest() plugin.Manifest { return Manifest() }

// New implements plugin.Plugin.
func (Plugin) New(env *plugin.Env) plugin.Instance { return &instance{env: env} }

// instance listens to every receiver plugged in. A mouse broadcasts only on
// the channel it is connected to, and one machine can have several receivers
// (a standard one and a 4K one), so every battery channel is watched at once.
type instance struct {
	env *plugin.Env

	mu       sync.Mutex
	readers  []*hid.Reader
	paths    []string
	device   string
	frame    Frame
	hasFrame bool
	lastAt   time.Time
	problem  string
	notified bool
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
	in.closeReaders()
}

// Poll implements plugin.Instance: the host's timer republishes the last
// frame, which keeps "2 秒前" honest even while the mouse is asleep.
func (in *instance) Poll() { in.publish() }

// vendorID reads the VID to watch from the settings.
func (in *instance) vendorID() uint16 {
	text := strings.TrimSpace(strings.ToLower(in.env.Settings().Get("vid")))
	text = strings.TrimPrefix(text, "0x")
	if text == "" {
		return rapooVendorID
	}
	v, err := strconv.ParseUint(text, 16, 16)
	if err != nil {
		return rapooVendorID
	}
	return uint16(v)
}

func (in *instance) loop(quit <-chan struct{}) {
	defer in.wg.Done()
	in.ensureReaders()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			in.ensureReaders()
		}
	}
}

// ensureReaders finds the battery channels and starts one goroutine per
// channel that is not being read yet.
func (in *instance) ensureReaders() {
	vendor := in.vendorID()
	intfs, err := hid.Enumerate(func(i hid.Interface) bool {
		return i.VendorID == vendor && i.UsagePage == batteryUsagePage && i.Usage == batteryUsage
	})
	if err != nil {
		in.fail(fmt.Sprintf("枚举 HID 设备失败：%v", err))
		return
	}

	in.mu.Lock()
	known := make(map[string]bool, len(in.readers))
	for _, r := range in.readers {
		known[r.Path()] = true
	}
	in.mu.Unlock()

	added := 0
	for _, intf := range intfs {
		if known[intf.Path] {
			continue
		}
		reader, err := hid.OpenReader(intf)
		if err != nil {
			in.fail(fmt.Sprintf("打开 %s 失败：%v", intf.Describe(), err))
			continue
		}
		in.mu.Lock()
		in.readers = append(in.readers, reader)
		in.paths = append(in.paths, intf.Describe())
		// The receiver names the mouse it belongs to; the panel shows that
		// name rather than one guessed from the vendor ID.
		if name := strings.TrimSpace(intf.Product); name != "" {
			in.device = name
		}
		problemCleared := in.problem
		in.problem = ""
		in.mu.Unlock()
		added++
		in.wg.Add(1)
		go in.read(reader)
		if problemCleared != "" {
			in.publish()
		}
	}

	in.mu.Lock()
	count := len(in.readers)
	in.mu.Unlock()
	if count == 0 && added == 0 {
		in.mu.Lock()
		in.hasFrame = false
		in.mu.Unlock()
		in.fail("没找到电量通道，确认接收器插好了")
	}
}

func (in *instance) read(reader *hid.Reader) {
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

		report, err := reader.Read(500 * time.Millisecond)
		if err != nil {
			in.fail(fmt.Sprintf("读取中断：%v", err))
			in.dropReader(reader)
			return
		}
		if report == nil {
			continue
		}
		frame, ok := ParseFrame(report)
		if !ok {
			continue
		}
		in.mu.Lock()
		in.frame = frame
		in.hasFrame = true
		in.lastAt = time.Now()
		in.problem = ""
		in.mu.Unlock()
		in.publish()
	}
}

func (in *instance) dropReader(reader *hid.Reader) {
	reader.Close()
	in.mu.Lock()
	defer in.mu.Unlock()
	for i, r := range in.readers {
		if r == reader {
			in.readers = append(in.readers[:i], in.readers[i+1:]...)
			if i < len(in.paths) {
				in.paths = append(in.paths[:i], in.paths[i+1:]...)
			}
			return
		}
	}
}

// fail records what went wrong, tells the panel and writes the log.
//
// Only a change is logged: ensureReaders runs every three seconds, so a
// receiver that is not plugged in would otherwise repeat the same line forever
// and the log would be one message with a thousand copies of itself. The panel
// keeps showing the problem while it lasts; the log records when it started.
func (in *instance) fail(text string) {
	in.mu.Lock()
	changed := in.problem != text
	in.problem = text
	in.mu.Unlock()
	if changed {
		in.env.Logf("mouse: %s", text)
	}
	in.publish()
}

func (in *instance) closeReaders() {
	in.mu.Lock()
	readers := in.readers
	in.readers = nil
	in.paths = nil
	in.mu.Unlock()
	for _, r := range readers {
		r.Close()
	}
}

// publish turns the last frame into a snapshot.
func (in *instance) publish() {
	in.mu.Lock()
	frame, hasFrame, lastAt, problem := in.frame, in.hasFrame, in.lastAt, in.problem
	device := in.device
	in.mu.Unlock()

	if device == "" {
		device = "无线鼠标"
	}
	now := time.Now()
	snap := core.Snapshot{At: now, Title: device, Primary: "battery"}
	if !hasFrame {
		snap.Status = core.StatusOffline
		snap.Subtitle = problem
		if snap.Subtitle == "" {
			snap.Subtitle = "等待接收器广播"
		}
		in.env.Publish(snap)
		return
	}

	age := now.Sub(lastAt)
	stale := age > 15*time.Second
	switch {
	case stale:
		snap.Status = core.StatusOffline
		snap.Subtitle = core.FormatDuration(age) + "没有更新"
	case frame.Battery <= 30:
		snap.Status = core.StatusWarn
	default:
		snap.Status = core.StatusOK
	}

	link := frame.LinkText()
	linkSeverity := core.SeverityOK
	if stale {
		link = "已断开"
		linkSeverity = core.SeverityOffline
		snap.Subtitle = "已断开"
	} else {
		snap.Subtitle = fmt.Sprintf("%s · %d%%", link, frame.Battery)
	}

	snap.Readings = []core.Reading{
		{
			ID: "battery", Label: "电量", Value: float64(frame.Battery), Unit: core.UnitPercent,
			Severity: batterySeverity(frame.Battery), Detail: core.FormatDuration(age),
		},
		{ID: "link", Label: "连接", Text: link, Severity: linkSeverity},
		{
			ID: "dpi", Label: "DPI", Value: float64(frame.DPI), Unit: core.UnitNone,
			Severity: core.SeverityNone, Detail: "来自状态帧",
		},
	}
	if frame.Wired() {
		snap.Readings = append(snap.Readings, core.Reading{
			ID: "charging", Label: "供电", Text: "有线", Severity: core.SeverityInfo,
		})
	}
	in.env.Publish(snap)
	in.maybeNotify(frame.Battery, stale)
}

// maybeNotify posts one notification per charge cycle when the battery falls
// to 15%.
func (in *instance) maybeNotify(level int, stale bool) {
	if !in.env.Settings().Bool("notify", true) || stale {
		return
	}
	in.mu.Lock()
	notified := in.notified
	in.mu.Unlock()

	if level <= 15 && !notified {
		in.mu.Lock()
		in.notified = true
		in.mu.Unlock()
		in.env.Notify("鼠标电量低", fmt.Sprintf("只剩 %d%%，该充电了。", level))
		return
	}
	if level > 20 && notified {
		in.mu.Lock()
		in.notified = false
		in.mu.Unlock()
	}
}
