// Command preview renders the panel to PNG files without a window, so that
// its layout can be checked on machines without a desktop session and
// compared between changes.
//
//	go run ./cmd/preview -out docs
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
	"seeall/internal/panel"
)

func main() {
	out := flag.String("out", "docs", "directory for the PNG files")
	scale := flag.Float64("scale", 2, "pixel scale")
	width := flag.Int("width", 0, "panel width in DIPs; 0 takes the display's")
	height := flag.Int("height", 0, "panel height in DIPs; 0 takes the display's")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	// The preview draws the panel this machine gets: a 2560×1440 display
	// at 150% scaling, so the work area is 1706×912 DIPs.
	fitted := panel.Fit(1706, 912)
	w, h := *width, *height
	if w == 0 {
		w = fitted.Width
	}
	if h == 0 {
		h = fitted.Height
	}

	shots := []struct {
		name string
		row  string
	}{
		{"panel-mouse", "mouse"},
		{"panel-system", "system"},
		{"panel-server", "server:example.com"},
		{"panel-agent", "agent"},
		{"panel-settings", panel.SettingsPage},
	}
	for _, shot := range shots {
		_, p := fixture(fitted)
		p.SetSelected(shot.row)
		img := ui.Render(p.View, w, h, float32(*scale))
		path := filepath.Join(*out, shot.name+".png")
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		fmt.Printf("wrote %s (%d×%d)\n", path, w, h)
	}
}

// fixture builds a host with the built-in plugins and fills it with the data
// a real machine would show, so the preview is deterministic.
func fixture(size panel.Metrics) (*app.Host, *panel.App) {
	dir, err := os.MkdirTemp("", "seeall-preview")
	if err != nil {
		log.Fatal(err)
	}
	host := app.NewHost(dir)
	if err := host.Load(); err != nil {
		log.Fatal(err)
	}
	// Stop the plugins' own polling: the preview shows what it injects.
	host.Stop()
	host.OnChange = func() {}
	host.ClearHistory()

	now := time.Now()
	inject := func(id string, snap core.Snapshot, history map[string][]float64) {
		if !host.Inject(id, snap) {
			log.Fatalf("预览：没有实例 %q", id)
		}
		for reading, values := range history {
			host.InjectHistory(id, reading, values)
		}
	}

	inject("mouse", core.Snapshot{
		At: now.Add(-2 * time.Second), Status: core.StatusOK,
		Title: "雷柏 2.4G 鼠标", Subtitle: "无线 · 72%", Primary: "battery",
		Readings: []core.Reading{
			{ID: "battery", Label: "电量", Value: 72, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "2 秒前"},
			{ID: "link", Label: "连接", Text: "无线", Severity: core.SeverityOK},
			{ID: "dpi", Label: "DPI", Value: 1200, Severity: core.SeverityNone, Detail: "来自状态帧"},
		},
	}, map[string][]float64{"battery": wave(72, 40, 8, 3)})

	inject("keyboard", core.Snapshot{
		At: now.Add(-9 * time.Second), Status: core.StatusWarn,
		Title: "Rapoo Gaming Device", Subtitle: "24% · 该充电了", Primary: "battery",
		Readings: []core.Reading{
			{ID: "battery", Label: "电量", Value: 24, Unit: core.UnitPercent,
				Severity: core.SeverityWarn, Detail: "9 秒前"},
			{ID: "source", Label: "来源", Text: "电池字段", Severity: core.SeverityNone,
				Detail: "Rapoo Gaming Device"},
		},
	}, nil)

	inject("system", core.Snapshot{
		At: now.Add(-1 * time.Second), Status: core.StatusOK,
		Title: "DESKTOP-4K1", Subtitle: "Windows · windows/amd64", Primary: "cpu",
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 23.4, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "16 核"},
			{ID: "mem", Label: "内存", Value: 61, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "19.4 GB / 31.8 GB"},
			{ID: "disk", Label: "磁盘 C:", Value: 78, Unit: core.UnitPercent,
				Severity: core.SeverityWarn, Detail: "212 GB 可用 / 953 GB"},
			{ID: "battery", Label: "电池", Value: 84, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "3 小时后续航"},
			{ID: "uptime", Label: "开机", Text: "2.4 h", Severity: core.SeverityNone},
		},
	}, map[string][]float64{"cpu": wave(23, 18, 9, 6), "mem": wave(61, 18, 4, 11)})

	inject("server:example.com", core.Snapshot{
		At: now.Add(-4 * time.Second), Status: core.StatusOK,
		Title: "example.com", Subtitle: "example.com", Primary: "latency",
		Readings: []core.Reading{
			{ID: "status", Label: "状态码", Value: 200, Severity: core.SeverityOK},
			{ID: "latency", Label: "延迟", Value: 142, Unit: core.UnitMillis,
				Severity: core.SeverityOK, Detail: "延迟平稳"},
			{ID: "cert", Label: "证书", Value: 68, Unit: core.UnitDay, Severity: core.SeverityOK,
				Detail: "到期前"},
			{ID: "uptime", Label: "上次成功", Text: "刚好", Severity: core.SeverityNone},
		},
	}, map[string][]float64{"latency": wave(142, 24, 40, 5)})

	inject("agent", core.Snapshot{
		At: now.Add(-1 * time.Second), Status: core.StatusOK,
		Title: "Agent", Subtitle: "Claude Code · Codex", Primary: "count",
		Readings: []core.Reading{
			{ID: "count", Label: "运行中", Value: 3, Severity: core.SeverityOK},
			{ID: "busy", Label: "在计算", Value: 2, Severity: core.SeverityNone,
				Detail: "刚用掉 CPU 就算"},
			{ID: "agent-1", Label: "Claude Code", Text: "claude --resume",
				Severity: core.SeverityNone, Detail: "PID 8124 · 42 分钟前"},
			{ID: "agent-2", Label: "Codex", Text: "codex exec",
				Severity: core.SeverityNone, Detail: "PID 9032 · 6 分钟前"},
			{ID: "cpu", Label: "占用 CPU", Value: 1.8, Unit: core.UnitSeconds,
				Severity: core.SeverityNone, Detail: "最近一次采样"},
		},
	}, map[string][]float64{"cpu": wave(1.8, 12, 1.2, 9)})

	p := panel.New(host)
	p.SetSize(size)
	return host, p
}

// wave makes a plausible history of count samples that ends at v.
func wave(v float64, count int, wobble, seed float64) []float64 {
	out := make([]float64, 0, count)
	for i := 0; i < count; i++ {
		phase := float64(i)/3 + seed
		drift := wobble * float64(i) / float64(count)
		out = append(out, math.Max(0, v-drift+wobble*0.6*math.Sin(phase)))
	}
	return out
}
