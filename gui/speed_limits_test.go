//go:build !service

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"pbscommon"
)

// noSpeedLimits puts the process's budgets back to none after a test.
func noSpeedLimits(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { applySpeedLimits(nil) })
}

func wantSpeedLimits(t *testing.T, up, down, disk int64) {
	t.Helper()
	got := [3]int64{
		pbscommon.SpeedLimit(pbscommon.LimitUpload),
		pbscommon.SpeedLimit(pbscommon.LimitDownload),
		pbscommon.SpeedLimit(pbscommon.LimitDiskRead),
	}
	if got != [3]int64{up, down, disk} {
		t.Errorf("pbscommon limits = %v, want [%d %d %d]", got, up, down, disk)
	}
}

// The setter saves the limits, applies them to pbscommon at once (MB/s are
// decimal megabytes), and the window reads them back.
func TestSetSpeedLimitsPersistsAndApplies(t *testing.T) {
	strictHome(t)
	noSpeedLimits(t)
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}

	if err := a.SetSpeedLimits(10, 20, 30); err != nil {
		t.Fatal(err)
	}
	wantSpeedLimits(t, 10_000_000, 20_000_000, 30_000_000)
	if c := LoadConfig(); c.UploadLimitMBps != 10 || c.DownloadLimitMBps != 20 || c.DiskReadLimitMBps != 30 {
		t.Errorf("saved: upload %d, download %d, disk read %d", c.UploadLimitMBps, c.DownloadLimitMBps, c.DiskReadLimitMBps)
	}
	m := a.GetConfigWithHostname()
	if m["upload_limit_mbps"] != 10 || m["download_limit_mbps"] != 20 || m["disk_read_limit_mbps"] != 30 {
		t.Errorf("GetConfigWithHostname: upload %v, download %v, disk read %v",
			m["upload_limit_mbps"], m["download_limit_mbps"], m["disk_read_limit_mbps"])
	}

	if err := a.SetSpeedLimits(0, 0, 0); err != nil {
		t.Fatal(err)
	}
	wantSpeedLimits(t, 0, 0, 0)

	for _, bad := range [][3]int{{-1, 0, 0}, {0, MaxSpeedLimitMBps + 1, 0}, {0, 0, -5}} {
		if err := a.SetSpeedLimits(bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("SetSpeedLimits(%v) accepted", bad)
		}
	}
	if a.config.UploadLimitMBps != 0 || a.config.DownloadLimitMBps != 0 || a.config.DiskReadLimitMBps != 0 {
		t.Errorf("a refused call changed the settings: %+v", a.config)
	}
}

// Reloading the settings (the service does before each run) applies the
// limits in them; a value out of range counts as none.
func TestReloadConfigAppliesSpeedLimits(t *testing.T) {
	strictHome(t)
	noSpeedLimits(t)
	c := &Config{PBSServers: map[string]*PBSServer{}, UploadLimitMBps: 7, DownloadLimitMBps: MaxSpeedLimitMBps + 1, DiskReadLimitMBps: 3}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}
	a.ReloadConfig()
	wantSpeedLimits(t, 7_000_000, 0, 3_000_000)
}

// The service runs SetSpeedLimits for the app, with typed arguments.
func TestSetSpeedLimitsInServiceCalls(t *testing.T) {
	if _, ok := apiCalls["SetSpeedLimits"]; !ok {
		t.Fatal("SetSpeedLimits is not on the service's list")
	}
	noSpeedLimits(t)
	svc := strictServiceApp(t)
	for raw, want := range map[string]string{`[1,2]`: "argument 3 missing", `[1,"2",3]`: "argument 2", `[]`: "argument 1 missing"} {
		if out, err := svc.CallForAPI("SetSpeedLimits", json.RawMessage(raw)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("SetSpeedLimits(%s) = %s, %v; want an error with %q", raw, out, err, want)
		}
	}
	if _, err := svc.CallForAPI("SetSpeedLimits", json.RawMessage(`[4,5,6]`)); err != nil {
		t.Fatal(err)
	}
	if svc.config.UploadLimitMBps != 4 || svc.config.DownloadLimitMBps != 5 || svc.config.DiskReadLimitMBps != 6 {
		t.Errorf("service settings: %+v", svc.config)
	}
	wantSpeedLimits(t, 4_000_000, 5_000_000, 6_000_000)
}

// An app working through the service hands the change to the service, and
// takes the limits back with the service's settings for what it does itself
// (restores, browsing).
func TestSetSpeedLimitsThroughService(t *testing.T) {
	noSpeedLimits(t)
	p := newStrictPair(t)
	if err := p.app.SetSpeedLimits(8, 9, 0); err != nil {
		t.Fatal(err)
	}
	p.service(func(s *App) {
		if s.config.UploadLimitMBps != 8 || s.config.DownloadLimitMBps != 9 || s.config.DiskReadLimitMBps != 0 {
			t.Errorf("service settings: upload %d, download %d, disk read %d", s.config.UploadLimitMBps, s.config.DownloadLimitMBps, s.config.DiskReadLimitMBps)
		}
	})
	if c := p.app.config; c.UploadLimitMBps != 8 || c.DownloadLimitMBps != 9 {
		t.Errorf("app's copy: upload %d, download %d", c.UploadLimitMBps, c.DownloadLimitMBps)
	}
	wantSpeedLimits(t, 8_000_000, 9_000_000, 0)
	p.saw(t, "POST /call/SetSpeedLimits", "GET /config")
}
