// Package system watches the computer SeeAll runs on: processor load,
// memory, the drives, the battery and how long it has been up.
package system

import (
	"fmt"
	"runtime"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// Manifest describes the plugin to the app.
func Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "system",
		Name:            "本机",
		Summary:         "CPU、内存、磁盘、电池与开机时长",
		Kind:            plugin.KindSystem,
		Icon:            "cpu",
		DefaultInstance: "本机",
		Interval:        2 * time.Second,
		Settings: []plugin.Setting{
			{Key: "disk", Label: "磁盘", Type: plugin.SettingText, Default: "C:",
				Placeholder: "C:", Description: "要看剩余空间的盘符"},
			{Key: "notify", Label: "电池提醒", Type: plugin.SettingBool, Default: "true"},
		},
	}
}

// Plugin is the system monitor.
type Plugin struct{}

// Manifest implements plugin.Plugin.
func (Plugin) Manifest() plugin.Manifest { return Manifest() }

// New implements plugin.Plugin.
func (Plugin) New(env *plugin.Env) plugin.Instance { return &instance{env: env} }

type instance struct {
	env      *plugin.Env
	sampler  sampler
	started  bool
	quit     chan struct{}
	notified bool
}

// Start implements plugin.Instance.
func (in *instance) Start() {
	if in.started {
		return
	}
	in.started = true
	in.quit = make(chan struct{})
	in.sampler.init()
	in.collect()
	go func(quit <-chan struct{}) {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				in.collect()
			}
		}
	}(in.quit)
}

// Stop implements plugin.Instance.
func (in *instance) Stop() {
	if !in.started {
		return
	}
	in.started = false
	close(in.quit)
	in.quit = nil
}

// Poll implements plugin.Instance.
func (in *instance) Poll() { in.collect() }

// collect samples the machine and publishes what it found.
func (in *instance) collect() {
	now := time.Now()
	disk := in.env.Settings().Get("disk")
	if disk == "" {
		disk = "C:"
	}
	m := in.sampler.sample(disk)

	status := core.StatusOK
	readings := []core.Reading{
		{ID: "cpu", Label: "CPU", Value: m.CPUPercent, Unit: core.UnitPercent,
			Severity: loadSeverity(m.CPUPercent), Detail: fmt.Sprintf("%d 核", runtime.NumCPU())},
		{ID: "mem", Label: "内存", Value: m.MemUsedPercent, Unit: core.UnitPercent,
			Severity: loadSeverity(m.MemUsedPercent),
			Detail:   core.FormatBytes(m.MemUsed) + " / " + core.FormatBytes(m.MemTotal)},
	}
	if m.DiskTotal > 0 {
		used := 100 * (1 - m.DiskFree/m.DiskTotal)
		readings = append(readings, core.Reading{
			ID: "disk", Label: "磁盘 " + disk, Value: used, Unit: core.UnitPercent,
			Severity: loadSeverity(used),
			Detail:   core.FormatBytes(m.DiskFree) + " 可用 / " + core.FormatBytes(m.DiskTotal),
		})
	}
	if m.HasBattery {
		severity := core.SeverityOK
		switch {
		case m.BatteryPercent <= 10:
			severity = core.SeverityCritical
		case m.BatteryPercent <= 30:
			severity = core.SeverityWarn
		}
		detail := ""
		if m.Charging {
			detail = "充电中"
		} else if m.BatteryMinutes >= 0 {
			detail = core.FormatDuration(time.Duration(m.BatteryMinutes)*time.Minute) + "续航"
		}
		readings = append(readings, core.Reading{
			ID: "battery", Label: "电池", Value: m.BatteryPercent, Unit: core.UnitPercent,
			Severity: severity, Detail: detail,
		})
		if m.BatteryPercent <= 10 && !m.Charging {
			in.notifyBattery(m.BatteryPercent)
		} else if m.BatteryPercent > 20 || m.Charging {
			in.notified = false
		}
	}
	readings = append(readings, core.Reading{
		ID: "uptime", Label: "开机", Text: core.FormatMillis(float64(m.Uptime / time.Millisecond)),
		Severity: core.SeverityNone,
	})

	if m.CPUPercent >= 90 || m.MemUsedPercent >= 92 {
		status = core.StatusWarn
	}

	in.env.Publish(core.Snapshot{
		At:       now,
		Status:   status,
		Title:    m.Hostname,
		Subtitle: m.Platform,
		Primary:  "cpu",
		Readings: readings,
	})
}

// loadSeverity colors a percentage where high is bad.
func loadSeverity(p float64) core.Severity {
	switch {
	case p >= 92:
		return core.SeverityCritical
	case p >= 75:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

func (in *instance) notifyBattery(level float64) {
	if in.notified || !in.env.Settings().Bool("notify", true) {
		return
	}
	in.notified = true
	in.env.Notify("电量低", fmt.Sprintf("这台电脑只剩 %s，插上电源。", core.FormatNumber(level)+"%"))
}
