package main

import (
	"path/filepath"
	"testing"
)

// Each user has their own theme; a user who has not chosen one gets the
// machine-wide theme earlier versions saved, and with none of either the
// frontend uses the brand's colours.
func TestThemeIsPerUser(t *testing.T) {
	dir := t.TempDir()
	user := "alice"
	old := themeFilePath
	themeFilePath = func() (string, error) { return filepath.Join(dir, user, "theme.json"), nil }
	t.Cleanup(func() { themeFilePath = old })

	a := &App{config: &Config{}}
	if got := a.GetTheme(); got.Preset != "" {
		t.Fatalf("nothing saved: preset %q, want none", got.Preset)
	}

	a.config.Theme = &ThemeSettings{Preset: "red"} // saved by an earlier version
	if got := a.GetTheme(); got.Preset != "red" {
		t.Errorf("before choosing: preset %q, want the earlier machine-wide red", got.Preset)
	}

	if err := a.SaveTheme(ThemeSettings{Preset: "blue"}); err != nil {
		t.Fatal(err)
	}
	if got := a.GetTheme(); got.Preset != "blue" {
		t.Errorf("alice: preset %q, want her blue", got.Preset)
	}
	if a.config.Theme.Preset != "red" {
		t.Errorf("saving changed the machine-wide theme to %q", a.config.Theme.Preset)
	}

	user = "bob"
	if got := a.GetTheme(); got.Preset != "red" {
		t.Errorf("bob: preset %q, want the machine-wide red, not alice's choice", got.Preset)
	}
}
