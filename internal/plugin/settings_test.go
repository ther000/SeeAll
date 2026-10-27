package plugin

import "testing"

// TestLoadDropsSettingsTheManifestNoLongerHas checks a saved value survives a
// rename only when the plugin still declares it: the app's settings file is
// rewritten from what the settings hold, so a dropped key must not come back.
func TestLoadDropsSettingsTheManifestNoLongerHas(t *testing.T) {
	s := NewSettings([]Setting{
		{Key: "gpu", Default: "auto"},
		{Key: "notify", Default: "true"},
	})
	s.Load(map[string]string{
		"gpu":    "10DE:2520#0",
		"notify": "false",
		"disk":   "C:",
	})
	if got := s.Get("gpu"); got != "10DE:2520#0" {
		t.Errorf("gpu = %q, want the saved card", got)
	}
	if got := s.Get("notify"); got != "false" {
		t.Errorf("notify = %q, want the saved switch", got)
	}
	if _, ok := s.All()["disk"]; ok {
		t.Errorf("a setting the manifest dropped is still held: %v", s.All())
	}
}
