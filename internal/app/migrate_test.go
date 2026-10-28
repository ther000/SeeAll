package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSettingsMoveFromTheOldFolder checks the one thing a change of location
// can get wrong: an upgrade loses the settings the user gave a monitor, because
// the new folder is empty and nothing looked in the old one.
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
			{PluginID: "mouse", Name: "鼠标", Settings: map[string]string{"vid": "1234"}},
		},
	}
	legacy.UI.Opacity = &faint

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

	if got := h.PanelOpacity(); got != faint {
		t.Errorf("panel opacity = %v, want the old file's %v", got, faint)
	}
	if got := h.Instance("mouse").Settings().Get("vid"); got != "1234" {
		t.Errorf("the mouse's vendor id = %q, want the old file's 1234", got)
	}
	if _, err := os.Stat(old + ".migrated"); err != nil {
		t.Errorf("the old file was not moved aside: %v", err)
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
		{PluginID: "mouse", Name: "鼠标", Settings: map[string]string{"vid": "1234"}},
	}}
	data, _ := json.Marshal(stale)
	if err := os.WriteFile(old, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// The new place already has a file of its own, which is the only thing
	// the app should read.
	dir := t.TempDir()
	current := fileState{Version: 1, Instances: []savedEntry{
		{PluginID: "mouse", Name: "鼠标", Settings: map[string]string{"vid": "9999"}},
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

	if got := h.Instance("mouse").Settings().Get("vid"); got != "9999" {
		t.Errorf("the mouse's vendor id = %q, want the new file's 9999", got)
	}
}

// TestEveryPluginRunsWhateverTheFileSays is what "these are found by
// themselves" means: a file that lists one monitor, or none at all, still gets
// every plugin the app has. The file is where the settings live, not a list of
// what to run — a machine whose file predates a plugin must gain it, not miss
// it forever.
func TestEveryPluginRunsWhateverTheFileSays(t *testing.T) {
	files := []fileState{
		{Version: 1, Instances: []savedEntry{{PluginID: "mouse", Name: "鼠标"}}},
		{Version: 1},
	}
	for i, state := range files {
		dir := t.TempDir()
		data, err := json.Marshal(state)
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
		for _, id := range []string{"mouse", "system"} {
			if h.Instance(id) == nil {
				t.Errorf("file %d: %q does not run: %v", i, id, h.Instances())
			}
		}
	}
}
