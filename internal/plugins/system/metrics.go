package system

import "time"

// metrics is one sample of the machine.
type metrics struct {
	Hostname string
	Platform string

	CPUPercent     float64
	MemTotal       float64
	MemUsed        float64
	MemUsedPercent float64

	DiskTotal float64
	DiskFree  float64

	HasBattery     bool
	BatteryPercent float64
	Charging       bool
	// BatteryMinutes is how long the battery lasts, or -1 when unknown.
	BatteryMinutes int

	Uptime time.Duration
}
