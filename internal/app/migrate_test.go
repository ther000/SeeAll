package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSettingsMoveFromTheOldFolder checks the one thing a change of location
// can get wrong: an upgrade loses every monitor the user arranged, because the
// new folder is empty and nothing looked in the old one.
func TestSettingsMoveFromTheOldFolder(t *testing.T) {
	userConfig := t.TempDir()
	previous := userConfigDir
	userConfigDir = func() (string, error) { return userConfig, nil }
	t.Cleanup(func() { userConfigDir = previous })

	// A settings file as a build that wrote to the per-user folder left it.
	faint := 0.5
	legacy := fileState{
		Version: 1,
		Instances: []savedEntry{
			{PluginID: "system", Name: "本机", Settings: map[string]string{"gpu": "auto"}},
		},
	}
	legacy.UI.Opacity = &faint
	legacy.Order = map[string][]string{"system": {"gpu", "cpu"}}

	old := filepath.Join(userConfig, "SeeAll", "seeall.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// A fresh folder, as the app beside its executable would have.
	h := NewHost(t.TempDir())
	h.AdoptLegacySettings()
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()

	if h.Instance("system") == nil {
		t.Errorf("the old file's monitor was not read: %v", h.Instances())
	}
	if got := h.PanelOpacity(); got != faint {
		t.Errorf("panel opacity = %v, want the old file's %v", got, faint)
	}
	if got := h.CardOrder("system"); len(got) != 2 || got[0] != "gpu" {
		t.Errorf("card order = %v, want the old file's arrangement", got)
	}
}

// TestTheMoveHappensOnce is why the old file is marked as taken rather than
// merely read: a user who removes every monitor writes a file with none in it,
// and a migration that only looks for a missing file would put them all back at
// the next launch. It would also read the real machine's file on every run.
func TestTheMoveHappensOnce(t *testing.T) {
	userConfig := t.TempDir()
	previous := userConfigDir
	userConfigDir = func() (string, error) { return userConfig, nil }
	t.Cleanup(func() { userConfigDir = previous })

	old := filepath.Join(userConfig, "SeeAll", "seeall.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(fileState{Version: 1, Instances: []savedEntry{
		{PluginID: "system", Name: "旧的本机"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, data, 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	first := NewHost(dir)
	first.AdoptLegacySettings()
	if err := first.Load(); err != nil {
		t.Fatal(err)
	}
	first.Stop()
	if first.Instance("system") == nil {
		t.Fatal("the first launch did not take the old file's monitor")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the old file was not moved aside: %v", err)
	}

	// The user empties the panel: the file now holds no monitor.
	if err := os.Remove(filepath.Join(dir, "seeall.json")); err != nil {
		t.Fatal(err)
	}
	empty := NewHost(dir)
	empty.AdoptLegacySettings()
	if err := empty.Load(); err != nil {
		t.Fatal(err)
	}
	empty.Stop()
	for _, in := range empty.Instances() {
		empty.Remove(in.ID)
	}

	// And launches again: the removed monitors stay removed.
	again := NewHost(dir)
	again.AdoptLegacySettings()
	if err := again.Load(); err != nil {
		t.Fatal(err)
	}
	again.Stop()
	if again.Instance("system") != nil {
		t.Error("the old file's monitor came back after the panel was emptied")
	}
}

// TestANewFileWinsOverTheOldOne keeps the move from resurrecting settings the
// user has since changed in the new place.
func TestANewFileWinsOverTheOldOne(t *testing.T) {
	userConfig := t.TempDir()
	previous := userConfigDir
	userConfigDir = func() (string, error) { return userConfig, nil }
	t.Cleanup(func() { userConfigDir = previous })

	old := filepath.Join(userConfig, "SeeAll", "seeall.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := fileState{Version: 1, Instances: []savedEntry{
		{PluginID: "system", Name: "旧的本机"},
	}}
	data, _ := json.Marshal(stale)
	if err := os.WriteFile(old, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// The new place already has a file of its own, which is the only thing
	// the app should read.
	dir := t.TempDir()
	current := fileState{Version: 1, Instances: []savedEntry{
		{PluginID: "mouse", Name: "鼠标"},
	}}
	data, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seeall.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewHost(dir)
	if err := h.Load(); err != nil {
		t.Fatal(err)
	}
	h.Stop()

	if h.Instance("mouse") == nil {
		t.Errorf("the new file's monitor was not read: %v", h.Instances())
	}
	if h.Instance("system") != nil {
		t.Error("the old file was read although the new one exists")
	}
}
