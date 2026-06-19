package main

// Version injected at build time via ldflags (-X main.appVersion=x.y.z)
// Source of truth: gui/wails.json productVersion
var appVersion = "dev" // Default for local dev without ldflags
