package system

import "testing"

// TestHealthScoreOfAnIdleMachine checks the number the dashboard's header
// leads with on the reference screen: a processor at 5%, memory half full, the
// graphics card asleep, 54°C and a volume with room to spare reads 90 很好.
// The temperature is what keeps a machine sitting still from reading 100.
func TestHealthScoreOfAnIdleMachine(t *testing.T) {
	in := healthInput{
		CPUPercent:      5,
		MemPercent:      45,
		HasGPU:          true,
		GPUPercent:      3,
		HasTemp:         true,
		TempC:           54,
		HasDisk:         true,
		DiskUsedPercent: 60,
		HasBattery:      true,
		BatteryPercent:  100,
		Charging:        true,
	}
	score := healthScore(in)
	if score != 90 {
		t.Errorf("an idle machine scores %d, want 90", score)
	}
	if note := healthNote(score); note != "很好" {
		t.Errorf("90 is %q, want 很好", note)
	}
}

// TestHealthScoreIsNotPenalisedForWhatTheMachineCannotSay checks the other half
// of "no fake data": a desktop with no battery and no readable thermometer is
// not deducted for their absence, and scores higher than the same load with a
// hot processor.
func TestHealthScoreIsNotPenalisedForWhatTheMachineCannotSay(t *testing.T) {
	quiet := healthInput{CPUPercent: 5, MemPercent: 45}
	if score := healthScore(quiet); score != 100 {
		t.Errorf("a quiet machine with nothing to measure scores %d, want 100", score)
	}
	quiet.HasTemp, quiet.TempC = true, 54
	if score := healthScore(quiet); score >= 100 {
		t.Errorf("the same machine at 54°C still scores %d", score)
	}
}

// TestHealthScoreFallsWithLoad checks that each dimension costs points: a
// machine at 100% on every count is not "很好" with a temperature added on.
func TestHealthScoreFallsWithLoad(t *testing.T) {
	loaded := healthInput{
		CPUPercent: 100, MemPercent: 95,
		HasGPU: true, GPUPercent: 100,
		HasTemp: true, TempC: 92,
		HasDisk: true, DiskUsedPercent: 99,
		HasBattery: true, BatteryPercent: 5,
	}
	score := healthScore(loaded)
	if score > 15 {
		t.Errorf("a machine maxed out scores %d, want a handful of points", score)
	}
	if note := healthNote(score); note != "需要处理" {
		t.Errorf("a maxed-out machine is %q, want 需要处理", note)
	}
}

// TestHealthScoreStaysAboveZero checks the promise core makes about 0: it means
// "this source does not score itself", and the dashboard then draws the
// machine's name where the number would be. A machine in trouble must not be
// mistaken for a source with nothing to say.
func TestHealthScoreStaysAboveZero(t *testing.T) {
	worst := healthInput{
		CPUPercent: 100, MemPercent: 100,
		HasGPU: true, GPUPercent: 100,
		HasTemp: true, TempC: 120,
		HasDisk: true, DiskUsedPercent: 100,
		HasBattery: true, BatteryPercent: 0,
	}
	if score := healthScore(worst); score < 1 {
		t.Errorf("the worst machine scores %d, want at least 1", score)
	}
}

// TestBatteryOnlyCostsWhileItIsDraining checks the battery dimension: a laptop
// on the cable is not unhealthy at 5%, one running on a nearly empty battery is.
func TestBatteryOnlyCostsWhileItIsDraining(t *testing.T) {
	base := healthInput{CPUPercent: 5, MemPercent: 45}
	charging := base
	charging.HasBattery, charging.BatteryPercent, charging.Charging = true, 5, true
	if score := healthScore(charging); score != 100 {
		t.Errorf("a machine charging at 5%% scores %d, want 100", score)
	}
	draining := base
	draining.HasBattery, draining.BatteryPercent = true, 5
	if score := healthScore(draining); score != 90 {
		t.Errorf("a machine draining at 5%% scores %d, want 90", score)
	}
}

// TestHealthNoteBands pins the four words and the boundaries between them: the
// header writes the word, so an off-by-one at 90 says "良好" about a machine
// the reference screen calls 很好.
func TestHealthNoteBands(t *testing.T) {
	cases := map[int]string{
		100: "很好", 90: "很好", 89: "良好", 75: "良好", 74: "一般",
		60: "一般", 59: "需要处理", 1: "需要处理", 0: "需要处理",
	}
	for score, want := range cases {
		if got := healthNote(score); got != want {
			t.Errorf("healthNote(%d) = %q, want %q", score, got, want)
		}
	}
}

// TestHealthOfASample checks the score is taken from the sample the page shows,
// not from something else: a machine with a readable 54°C thermal zone lands in
// the same place as the same numbers handed over directly.
func TestHealthOfASample(t *testing.T) {
	m := metrics{
		CPUPercent: 5, MemUsedPercent: 45,
		HasGPU: true, GPUPercent: 3,
		HasTemp: true, TempC: 54,
		// 128 of 512 GB free is 75% used, under the 80% the disk may start to
		// cost: the sample has room without being empty.
		HasDisk: true, DiskTotal: 512 << 30, DiskFree: 128 << 30,
		HasBattery: true, BatteryPercent: 100, Charging: true,
	}
	score, note := healthOf(m)
	if score != 90 || note != "很好" {
		t.Errorf("the sample scores %d %q, want 90 很好", score, note)
	}
}
