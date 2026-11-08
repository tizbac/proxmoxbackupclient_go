//go:build !service

package main

import (
	"encoding/json"
	"testing"
	"time"

	"pbscommon"
)

// Preferences > Backup Options > Chunk timeout: saved, applied to pbscommon at
// once, read back by the window; out of range refused.
func TestSetChunkTimeoutPersistsAndApplies(t *testing.T) {
	strictHome(t)
	noSpeedLimits(t)
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}

	if err := a.SetChunkTimeout(300); err != nil {
		t.Fatal(err)
	}
	if got := pbscommon.ChunkStallTimeout(); got != 300*time.Second {
		t.Errorf("pbscommon chunk timeout %v, want 5m0s", got)
	}
	if c := LoadConfig(); c.ChunkTimeoutSeconds != 300 {
		t.Errorf("saved: %d", c.ChunkTimeoutSeconds)
	}
	if m := a.GetConfigWithHostname(); m["chunk_timeout_seconds"] != 300 {
		t.Errorf("GetConfigWithHostname: %v", m["chunk_timeout_seconds"])
	}

	for _, bad := range []int{-1, 29, 1801} {
		if err := a.SetChunkTimeout(bad); err == nil {
			t.Errorf("SetChunkTimeout(%d) accepted", bad)
		}
	}
	if a.config.ChunkTimeoutSeconds != 300 {
		t.Errorf("a refused call changed the setting: %d", a.config.ChunkTimeoutSeconds)
	}

	if err := a.SetChunkTimeout(0); err != nil {
		t.Fatal(err)
	}
	if got := pbscommon.ChunkStallTimeout(); got != 0 {
		t.Errorf("default: pbscommon chunk timeout %v, want 0 (default)", got)
	}
}

// Reloading the settings applies the chunk timeout; a hand-edited value out
// of range counts as the default.
func TestReloadConfigAppliesChunkTimeout(t *testing.T) {
	strictHome(t)
	noSpeedLimits(t)
	for _, c := range []struct {
		seconds int
		want    time.Duration
	}{{120, 2 * time.Minute}, {5, 0}} {
		cfg := &Config{PBSServers: map[string]*PBSServer{}, ChunkTimeoutSeconds: c.seconds}
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}
		a.ReloadConfig()
		if got := pbscommon.ChunkStallTimeout(); got != c.want {
			t.Errorf("%d s in config.json: pbscommon chunk timeout %v, want %v", c.seconds, got, c.want)
		}
	}
}

// The service runs SetChunkTimeout for the app; an app working through the
// service hands the change to it and takes the setting back.
func TestSetChunkTimeoutThroughService(t *testing.T) {
	if _, ok := apiCalls["SetChunkTimeout"]; !ok {
		t.Fatal("SetChunkTimeout is not on the service's list")
	}
	noSpeedLimits(t)
	svc := strictServiceApp(t)
	if _, err := svc.CallForAPI("SetChunkTimeout", json.RawMessage(`["60"]`)); err == nil {
		t.Error("SetChunkTimeout with a string accepted")
	}
	if _, err := svc.CallForAPI("SetChunkTimeout", json.RawMessage(`[60]`)); err != nil {
		t.Fatal(err)
	}
	if svc.config.ChunkTimeoutSeconds != 60 {
		t.Errorf("service setting: %d", svc.config.ChunkTimeoutSeconds)
	}

	p := newStrictPair(t)
	if err := p.app.SetChunkTimeout(600); err != nil {
		t.Fatal(err)
	}
	p.service(func(s *App) {
		if s.config.ChunkTimeoutSeconds != 600 {
			t.Errorf("service setting: %d", s.config.ChunkTimeoutSeconds)
		}
	})
	if p.app.config.ChunkTimeoutSeconds != 600 {
		t.Errorf("app's copy: %d", p.app.config.ChunkTimeoutSeconds)
	}
	p.saw(t, "POST /call/SetChunkTimeout", "GET /config")
}
