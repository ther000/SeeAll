package system

import (
	"sort"
	"strings"

	"seeall/internal/core"
)

// topProcesses is the dashboard's list: the programs holding the most memory,
// in that order, with the extension the list has no room for trimmed off.
//
// A program is one row however many processes it runs in: a browser is a dozen
// processes, and five rows of one of them would say less than one row of what
// the program costs together.
func topProcesses(list []core.Process, n int) []core.Process {
	if n <= 0 || len(list) == 0 {
		return nil
	}
	merged := make(map[string]core.Process, len(list))
	order := make([]string, 0, len(list))
	for _, p := range list {
		name := programName(p.Name)
		if name == "" {
			continue
		}
		sum, seen := merged[name]
		if !seen {
			order = append(order, name)
		}
		sum.Name = name
		sum.Mem += p.Mem
		sum.CPU += p.CPU
		merged[name] = sum
	}
	out := make([]core.Process, 0, len(order))
	for _, name := range order {
		p := merged[name]
		if p.CPU > 100 {
			p.CPU = 100
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Mem != out[j].Mem {
			return out[i].Mem > out[j].Mem
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// programName is what the list calls a program: "Everything" rather than
// "Everything.exe".
func programName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
		name = name[:len(name)-4]
	}
	return name
}
