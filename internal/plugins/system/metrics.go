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

	// NetRxRate and NetTxRate are the bytes per second the machine is
	// receiving and sending, over every interface that is not a loopback.
	NetRxRate float64
	NetTxRate float64

	// HasGPU is false on a machine whose graphics cannot be counted, a
	// session without a GPU process, or a system That reports no counters.
	HasGPU bool
	// GPUPercent is how busy the graphics processor is: the sum of its
	// engines' utilization, as Task Manager shows it.
	GPUPercent float64
	// VRAMUsed and VRAMTotal are the graphics memory in use and the one the
	// adapter has.
	VRAMUsed  float64
	VRAMTotal float64

	Uptime time.Duration
}
