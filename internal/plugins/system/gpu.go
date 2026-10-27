package system

import (
	"strconv"
	"strings"
)

// The parts of the graphics card choice that do not need Windows: reading an
// adapter out of a counter's instance name, and naming a card in a row the
// panel has room for.

// parseLuid reads the adapter ID out of a performance counter's instance name,
// such as "pid_12772_luid_0x00000000_0x0000E3CD_phys_0_eng_0_engtype_3D": the
// counters of every card are in one place, told apart by that ID.
func parseLuid(name string) (uint64, bool) {
	const marker = "luid_0x"
	i := strings.Index(name, marker)
	if i < 0 {
		return 0, false
	}
	rest := name[i+len(marker):]
	// The marker takes the "0x" of the high half with it: eight hex digits,
	// then the second half's "0x" and eight more.
	if len(rest) < 19 || rest[8] != '_' || rest[9:11] != "0x" {
		return 0, false
	}
	high, err := strconv.ParseUint(rest[0:8], 16, 32)
	if err != nil {
		return 0, false
	}
	low, err := strconv.ParseUint(rest[11:19], 16, 32)
	if err != nil {
		return 0, false
	}
	return high<<32 | low, true
}

// shortGPUName drops what a narrow row does not need: the trademark marks and
// the vendor word in front of the model.
func shortGPUName(name string) string {
	name = strings.NewReplacer("(TM)", "", "(R)", "", "(tm)", "", "(r)", "").Replace(name)
	for _, prefix := range []string{"NVIDIA ", "AMD ", "Intel "} {
		name = strings.TrimPrefix(name, prefix)
	}
	return strings.TrimSpace(name)
}
