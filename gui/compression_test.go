//go:build !service

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The setter saves the level, runs take it, and the window reads it back.
// Empty, and anything not one of the five (a hand-edited config.json), is
// default; a level named for one run wins over the setting.
func TestSetCompressionLevel(t *testing.T) {
	strictHome(t)
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}

	if got := a.effectiveCompression(""); got != "default" {
		t.Errorf("nothing set: %q, want default", got)
	}
	if err := a.SetCompressionLevel("best"); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(); c.CompressionLevel != "best" {
		t.Errorf("saved: %q", c.CompressionLevel)
	}
	if got := a.effectiveCompression(""); got != "best" {
		t.Errorf("a run: %q, want best", got)
	}
	if got := a.effectiveCompression("fastest"); got != "fastest" {
		t.Errorf("a run asking for fastest: %q", got)
	}
	if m := a.GetConfigWithHostname(); m["compression_level"] != "best" {
		t.Errorf("GetConfigWithHostname: %v", m["compression_level"])
	}

	// "none" is a level like the others: saved, and what a run takes.
	if err := a.SetCompressionLevel("none"); err != nil {
		t.Fatal(err)
	}
	if got := a.effectiveCompression(""); got != "none" {
		t.Errorf("a run with none set: %q", got)
	}
	if got := a.effectiveCompression("off"); got != "none" {
		t.Errorf("a run asking for off: %q, want none", got)
	}
	if err := a.SetCompressionLevel("best"); err != nil {
		t.Fatal(err)
	}

	for _, bad := range []string{"fast", "max", "9", "zstd"} {
		if err := a.SetCompressionLevel(bad); err == nil {
			t.Errorf("SetCompressionLevel(%q) accepted", bad)
		}
	}
	if a.config.CompressionLevel != "best" {
		t.Errorf("a refused call changed the setting: %q", a.config.CompressionLevel)
	}

	a.config.CompressionLevel = "ultra" // hand-edited
	if got := a.effectiveCompression(""); got != "default" {
		t.Errorf("an unknown stored level: %q, want default", got)
	}
	if err := a.SetCompressionLevel(""); err != nil || a.effectiveCompression("") != "default" {
		t.Errorf("clearing: %v, %q", err, a.effectiveCompression(""))
	}
}

// The service runs SetCompressionLevel for the app, with a typed argument.
func TestSetCompressionLevelInServiceCalls(t *testing.T) {
	if _, ok := apiCalls["SetCompressionLevel"]; !ok {
		t.Fatal("SetCompressionLevel is not on the service's list")
	}
	svc := strictServiceApp(t)
	for raw, want := range map[string]string{`[]`: "argument 1 missing", `[3]`: "argument 1", `["fast"]`: "fastest, default, better or best"} {
		if out, err := svc.CallForAPI("SetCompressionLevel", json.RawMessage(raw)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("SetCompressionLevel(%s) = %s, %v; want an error with %q", raw, out, err, want)
		}
	}
	if _, err := svc.CallForAPI("SetCompressionLevel", json.RawMessage(`["better"]`)); err != nil {
		t.Fatal(err)
	}
	if svc.config.CompressionLevel != "better" || svc.effectiveCompression("") != "better" {
		t.Errorf("service setting: %q", svc.config.CompressionLevel)
	}
}

// An app working through the service hands the change to the service and
// reads the service's settings back.
func TestSetCompressionLevelThroughService(t *testing.T) {
	p := newStrictPair(t)
	if err := p.app.SetCompressionLevel("fastest"); err != nil {
		t.Fatal(err)
	}
	p.service(func(s *App) {
		if s.config.CompressionLevel != "fastest" {
			t.Errorf("service setting: %q", s.config.CompressionLevel)
		}
	})
	if got := compressionSetting(p.app.config); got != "fastest" {
		t.Errorf("app's copy: %q", got)
	}
	p.saw(t, "POST /call/SetCompressionLevel", "GET /config")
}
