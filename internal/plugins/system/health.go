package system

import "math"

// healthInput is what the health score looks at: the same numbers the cards
// show, and which of them the machine could actually be asked about. A machine
// with no battery or no readable temperature is not penalised for the gap.
type healthInput struct {
	CPUPercent float64
	MemPercent float64
	HasGPU     bool
	GPUPercent float64

	HasTemp bool
	TempC   float64

	HasDisk         bool
	DiskUsedPercent float64

	HasBattery     bool
	BatteryPercent float64
	Charging       bool
}

// healthOf grades one sample.
func healthOf(m metrics) (int, string) {
	in := healthInput{
		CPUPercent:     m.CPUPercent,
		MemPercent:     m.MemUsedPercent,
		HasGPU:         m.HasGPU,
		GPUPercent:     m.GPUPercent,
		HasTemp:        m.HasTemp,
		TempC:          m.TempC,
		HasDisk:        m.HasDisk && m.DiskTotal > 0,
		HasBattery:     m.HasBattery,
		BatteryPercent: m.BatteryPercent,
		Charging:       m.Charging,
	}
	if in.HasDisk {
		in.DiskUsedPercent = 100 * (m.DiskTotal - m.DiskFree) / m.DiskTotal
	}
	score := healthScore(in)
	return score, healthNote(score)
}

// healthScore is how well the machine is doing, from 1 to 100. Each dimension
// may cost a number of points, and only past a level that is already worth
// remarking on: a processor at 15% and a volume at 40% are both free.
//
// An idle laptop still lands around 90 rather than 100, because its own
// temperature costs something: 54°C is ten of the twenty-five points the
// temperature can take, which is the number the dashboard's reference screen
// shows. The floor is 1, not 0: a snapshot with no health of its own says 0 and
// the panel then draws the machine's name instead of a score.
func healthScore(in healthInput) int {
	score := 100.0
	score -= over(in.CPUPercent, 20, 100, 25)
	score -= over(in.MemPercent, 50, 100, 20)
	if in.HasGPU {
		score -= over(in.GPUPercent, 30, 100, 15)
	}
	if in.HasTemp {
		score -= over(in.TempC, 30, 90, 25)
	}
	if in.HasDisk {
		score -= over(in.DiskUsedPercent, 80, 98, 15)
	}
	if in.HasBattery && !in.Charging {
		score -= under(in.BatteryPercent, 50, 5, 10)
	}
	switch {
	case score < 1:
		return 1
	case score > 100:
		return 100
	}
	return int(math.Round(score))
}

// healthNote is what a score means in a word, in the bands the dashboard's
// header uses.
func healthNote(score int) string {
	switch {
	case score >= 90:
		return "很好"
	case score >= 75:
		return "良好"
	case score >= 60:
		return "一般"
	}
	return "需要处理"
}

// over is how many of max points a value above start costs, reaching all of
// them at full.
func over(v, start, full, max float64) float64 {
	if v <= start {
		return 0
	}
	if v >= full {
		return max
	}
	return max * (v - start) / (full - start)
}

// under is over for a value below start: a battery at floor costs all of max.
func under(v, start, floor, max float64) float64 {
	if v >= start {
		return 0
	}
	if v <= floor {
		return max
	}
	return max * (start - v) / (start - floor)
}
