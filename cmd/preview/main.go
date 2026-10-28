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

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"seeall/internal/app"
	"seeall/internal/core"
	"seeall/internal/panel"
	"seeall/internal/tray"
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

	// The preview draws the panel this machine gets: a 2560×1440 display at
	// 150% scaling, so the work area is 1706×912 DIPs, less the margin that
	// keeps the panel off the screen edge and above the taskbar. The design is
	// in pixels, so the scale decides how many DIPs the same pixels take.
	const displayScale = 1.5
	area := tray.Inset(mygo.Rectangle{Width: 1706, Height: 912},
		panel.MarginDIP(displayScale))
	fitted := panel.Fit(area.Width, area.Height, displayScale)

	// The dashboard is the whole of a machine's page — its score, its cards,
	// the battery, the programs it runs and the bar at the foot — and that
	// page is composed for the design's own canvas. The classic pages are the
	// panel as it opens on this display.
	shots := []struct {
		name  string
		row   string
		theme string
		// design draws the page at the panel's design size rather than at the
		// size this display gives it.
		design bool
	}{
		{"panel-dashboard", "system", app.ThemeDashboard, true},
		{"panel-dashboard-mouse", "mouse", app.ThemeDashboard, true},
		{"panel-dashboard-settings", panel.SettingsPage, app.ThemeDashboard, true},
		{"panel-mouse", "mouse", app.ThemeClassic, false},
		{"panel-system", "system", app.ThemeClassic, false},
		{"panel-settings", panel.SettingsPage, app.ThemeClassic, false},
	}
	for _, shot := range shots {
		_, p := fixture(fitted, shot.theme)
		p.SetSelected(shot.row)
		w, h := fitted.Width, fitted.Height
		if *width != 0 || *height != 0 {
			if *width != 0 {
				w = *width
			}
			if *height != 0 {
				h = *height
			}
		} else if shot.design {
			w, h = panel.DesignWidth, panel.DesignHeight
		}
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

// fixture builds a host with the built-in plugins, in the given look, and
// fills it with the data a real machine would show, so the preview is
// deterministic.
func fixture(size panel.Metrics, theme string) (*app.Host, *panel.App) {
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
	host.SetTheme(theme)
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
		Title: "Rapoo Gaming Device", Subtitle: "无线 · 72%", Primary: "battery",
		Health: 72, HealthNote: "电量充足",
		Facts: []core.Fact{
			{Label: "连接", Value: "无线"},
			{Label: "DPI", Value: "1200"},
		},
		Readings: []core.Reading{
			{ID: "battery", Label: "电量", Value: 72, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "2 秒前"},
			{ID: "link", Label: "连接", Text: "无线", Severity: core.SeverityOK},
			{ID: "dpi", Label: "DPI", Value: 1200, Severity: core.SeverityNone, Detail: "来自状态帧"},
		},
	}, map[string][]float64{"battery": wave(72, 40, 8, 3)})

	// The machine, as the dashboard describes it: a score, the four facts
	// across the top, the six parts of it, and the programs behind the
	// numbers.
	inject("system", core.Snapshot{
		At: now.Add(-1 * time.Second), Status: core.StatusOK,
		Title: "DESKTOP-4K1", Subtitle: "Windows 11 26H2 · windows/amd64", Primary: "cpu",
		Health: 90, HealthNote: "很好",
		Facts: []core.Fact{
			{Value: "Ryzen 7 6800H"},
			{Value: "13.7 GB"},
			{Value: "Windows 11 26H2"},
			{Label: "已运行", Value: "2天2小时"},
		},
		Processes: []core.Process{
			{Name: "QQ", Mem: 391 * 1024 * 1024, CPU: 0},
			{Name: "DeepSeek Harness", Mem: 254 * 1024 * 1024, CPU: 0},
			{Name: "Windows 服务主进程", Mem: 222 * 1024 * 1024, CPU: 3.3},
			{Name: "Everything", Mem: 143 * 1024 * 1024, CPU: 0},
			{Name: "Quicker", Mem: 128 * 1024 * 1024, CPU: 0},
		},
		Readings: []core.Reading{
			{ID: "cpu", Label: "CPU", Value: 5, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "2.24 GHz", Detail: "16 线程"},
			{ID: "gpu", Label: "GPU", Value: 4, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "50°C", Detail: "GeForce RTX 3060 Laptop GPU"},
			{ID: "mem", Label: "内存", Value: 46, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "压力正常", Detail: "6.39 GB / 13.7 GB"},
			{ID: "disk", Label: "可用空间", Value: 63 * 1024 * 1024 * 1024, Unit: core.UnitBytes,
				Severity: core.SeverityOK, Badge: "C: · 159 GB", Detail: "已用 96 GB · 60%"},
			{ID: "net", Label: "网络", Text: "<1 KB/s",
				Severity: core.SeverityOK, Badge: "Wi-Fi", Detail: "上传 <1 KB/s"},
			{ID: "temp", Label: "温度", Value: 54, Text: "54°C",
				Severity: core.SeverityOK, Badge: "正常", Detail: "系统 54° · GPU 50°"},
			{ID: "vram", Label: "显存", Value: 36, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Detail: "2.1 GB / 5.9 GB"},
			{ID: "battery", Label: "电池", Value: 100, Unit: core.UnitPercent,
				Severity: core.SeverityOK, Badge: "健康 78%", Detail: "已充满"},
			{ID: "uptime", Label: "开机", Text: "2天2小时", Severity: core.SeverityNone},
		},
	}, map[string][]float64{
		// A processor at rest draws a dashed line, a busy one a shape.
		"cpu": flat(5, 60),
		"gpu": wave(4, 60, 3, 4),
		"mem": wave(46, 60, 3, 11),
		"net": {900, 1400, 3000, 9000, 22000, 41000, 52000},
	})

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

// flat makes a history of one value: a reading that is not moving.
func flat(v float64, count int) []float64 {
	out := make([]float64, count)
	for i := range out {
		out[i] = v
	}
	return out
}
