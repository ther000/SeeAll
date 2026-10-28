// Package system watches the computer SeeAll runs on: processor load,
// memory, the battery, the network, the graphics processor and how long it
// has been up.
package system

import (
	"fmt"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// Manifest describes the plugin to the app.
func Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "system",
		Name:            "本机",
		Summary:         "CPU、内存、电池、网络、GPU 与开机时长",
		Kind:            plugin.KindSystem,
		Icon:            "cpu",
		DefaultInstance: "本机",
		Settings: []plugin.Setting{
			// The list is filled in when the page is drawn: which cards a
			// machine has is not known until it is running.
			{Key: "gpu", Label: "GPU", Type: plugin.SettingChoice, Default: gpuAuto,
				OptionsFor:  gpuChoices,
				Description: "只统计这一张"},
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
	// problem is the last thing written to the log, so that a condition that
	// lasts does not repeat itself on every sample.
	problem string
}

// Start implements plugin.Instance.
func (in *instance) Start() {
	if in.started {
		return
	}
	in.started = true
	in.quit = make(chan struct{})
	in.sampler.logf = in.env.Logf
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
	m := in.sampler.sample(in.env.Settings().Get("gpu"))

	// Whatever was said about the graphics being gone is over once the sample
	// stops reporting a problem — whether the card came back or the settings
	// stopped pointing at one. Otherwise the next time it goes missing there
	// would be nothing to say, because it would still equal the last line.
	if m.GPUProblem == "" {
		in.problem = ""
	} else if in.problem != m.GPUProblem {
		in.problem = m.GPUProblem
		// Said in the log too: the page explains it, but a user asking "why is
		// my GPU missing" looks at the log, and a line that recurs every few
		// seconds is noisier than the problem.
		in.env.Logf("system: %s", m.GPUProblem)
	}

	status := core.StatusOK
	diskPercent := 0.0
	if m.HasDisk && m.DiskTotal > 0 {
		diskPercent = 100 * (m.DiskTotal - m.DiskFree) / m.DiskTotal
	}
	switch {
	case m.CPUPercent >= 90, m.MemUsedPercent >= 92,
		m.HasGPU && m.GPUPercent >= 95,
		m.HasTemp && m.TempC >= 90,
		m.HasDisk && diskPercent >= 95,
		m.GPUProblem != "":
		status = core.StatusWarn
	}

	if m.HasBattery {
		if m.BatteryPercent <= 10 && !m.Charging {
			in.notifyBattery(m.BatteryPercent)
		} else if m.BatteryPercent > 20 || m.Charging {
			in.notified = false
		}
	}

	// The dashboard leads with one number and a word for it; the page's cards
	// say the same thing in more detail underneath.
	health, note := healthOf(m)
	in.env.Publish(core.Snapshot{
		At:         now,
		Status:     status,
		Title:      m.Hostname,
		Subtitle:   m.Platform,
		Primary:    "cpu",
		Readings:   readings(m),
		Health:     health,
		HealthNote: note,
		Facts:      facts(m),
		Processes:  topProcesses(m.Processes, processCount),
	})
}

func (in *instance) notifyBattery(level float64) {
	if in.notified || !in.env.Settings().Bool("notify", true) {
		return
	}
	in.notified = true
	in.env.Notify("电量低", fmt.Sprintf("这台电脑只剩 %s，插上电源。", core.FormatNumber(level)+"%"))
}
