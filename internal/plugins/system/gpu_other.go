//go:build !windows

package system

import "seeall/internal/plugin"

// Graphics cards are read through DXGI and counted through the performance
// counters, neither of which this build has: there is no card to choose and
// the page reports no GPU.

const gpuAuto = "auto"

type gpuAdapter struct {
	Key    string
	Name   string
	Luid   uint64
	Memory float64
}

func graphicsAdapters() []gpuAdapter { return nil }

func gpuChoices() []plugin.Option {
	return []plugin.Option{{Value: gpuAuto, Label: "自动"}}
}
