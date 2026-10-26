//go:build !windows

package agent

// listProcesses is what a port implements: reading another system's process
// table. Until then the plugin reports no agents.
func listProcesses() ([]Process, error) { return nil, nil }
