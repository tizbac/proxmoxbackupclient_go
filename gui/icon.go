package main

import (
	_ "embed"
)

// TrayIconData embeds the actual icon.ico file used by the application
// This ensures we use the same, properly formatted icon for the system tray
//
//go:embed build/windows/icon.ico
var TrayIconData []byte
