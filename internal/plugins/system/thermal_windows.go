//go:build windows

package system

// The processor's temperature and its current clock come out of the same
// performance counter query as the network rates: one read per sample covers
// them all.

const (
	// cpuClockPath is how fast the processor runs against the clock it booted
	// at, in percent. The registry knows the nominal clock; this knows the
	// real one, boost included.
	cpuClockPath = `\Processor Information(_Total)\% Processor Performance`
	// thermalPath is each ACPI thermal zone's temperature. Windows writes it
	// in kelvin, whatever the performance monitor's localized label says.
	thermalPath = `\Thermal Zone Information(*)\Temperature`
)

// sampleClock fills the clock the processor is running at now.
func (s *sampler) sampleClock(m *metrics) {
	if s.cpuClock == nil || s.inv.BaseMHz <= 0 {
		return
	}
	performance, ok := s.perf.value(s.cpuClock)
	if !ok || performance <= 0 {
		return
	}
	// A machine that idles down does report under 100%, which is the point:
	// "2.24 GHz" on a 3.2 GHz processor is what Task Manager shows too.
	mhz := s.inv.BaseMHz * performance / 100
	if mhz < 100 || mhz > 12000 {
		return
	}
	m.CPUClockMHz = mhz
}

// sampleTemp fills the machine's temperature with the hottest thermal zone.
func (s *sampler) sampleTemp(m *metrics) {
	if s.thermal == nil {
		return
	}
	for _, zone := range s.perf.instanceValues(s.thermal) {
		celsius := zone.Value - 273.15
		// A zone that is not really measured reports zero, or a kelvin value
		// no computer runs at: the firmware's placeholder is not a reading.
		if celsius < 0 || celsius > 125 {
			continue
		}
		if !m.HasTemp || celsius > m.TempC {
			m.HasTemp, m.TempC = true, celsius
		}
	}
	if m.HasTemp {
		s.sayClear("temp")
		return
	}
	s.sayOnce("temp", "system: 温度计数器没有值，温度读数不会出现")
}

// A graphics processor's own temperature is not in any Windows API: it needs
// the vendor's library (NVAPI, ADL) or a WMI class most drivers do not publish.
// metrics.HasGPUTemp therefore stays false, and the temperature card says what
// the thermal zones said instead of inventing the other half.
