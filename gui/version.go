package main

import (
	"strings"

	"pbscommon"
)

// Version injected at build time via ldflags (-X main.appVersion=x.y.z)
// Source of truth: gui/wails.json productVersion
var appVersion = "dev" // Default for local dev without ldflags

// The snapshots this program writes name its version in their manifest.
func init() { pbscommon.SetClientVersion(appVersion) }

// versionLabel is the version as shown to people: "v0.7.0" for a release,
// and a development build as is ("dev-49d96e4", not "vdev-49d96e4").
func versionLabel() string {
	if appVersion == "" || strings.HasPrefix(appVersion, "dev") {
		return appVersion
	}
	return "v" + appVersion
}
