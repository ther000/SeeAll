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

// gpuPlaceholderName stands in when a machine's graphics can be counted but no
// card could be enumerated. It is not a model, so it never becomes a badge.
const gpuPlaceholderName = "显卡"

// gpuModelMax is how much of a model the GPU card's badge holds before the
// panel would cut it in the middle of a word.
const gpuModelMax = 18

// gpuBrandWord is a word of a graphics card's name that says who made it
// rather than which one it is.
var gpuBrandWord = map[string]bool{
	"geforce": true, "nvidia": true,
	"radeon": true, "amd": true,
	"intel": true,
	// "Graphics" and "GPU" end half the drivers' names: "UHD Graphics 630",
	// "RTX 3060 Laptop GPU".
	"graphics": true, "gpu": true,
}

// gpuModel is the badge on the GPU card: the model, without the brand words a
// narrow badge has no room for. "GeForce RTX 3060 Laptop GPU" reads
// "RTX 3060 Laptop".
func gpuModel(name string) string {
	if name == "" || name == gpuPlaceholderName {
		return ""
	}
	fields := strings.Fields(shortGPUName(name))
	kept := make([]string, 0, len(fields))
	for _, word := range fields {
		if gpuBrandWord[strings.ToLower(word)] {
			continue
		}
		kept = append(kept, word)
	}
	// A name that was nothing but brand words — an integrated card called
	// "Radeon Graphics" — still has to say something.
	model := strings.Join(kept, " ")
	if model == "" {
		model = strings.Join(fields, " ")
	}
	if model == "" {
		return ""
	}
	return clip(model, gpuModelMax)
}

// clip shortens a label to at most max characters, at a word boundary when
// there is one, so that a badge does not end in half a word.
func clip(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	for i := max - 1; i > 0; i-- {
		if runes[i] == ' ' {
			return string(runes[:i])
		}
	}
	return string(runes[:max])
}
