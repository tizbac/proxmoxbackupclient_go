package main

import "embed"

// TrayIconData embeds the default icon.ico file used by the application.
// It is the ultimate fallback; the tray normally shows the brand icon.
//
//go:embed build/windows/icon.ico
var TrayIconData []byte

// brandTrayIconDir is where the per-brand tray icons live inside the binary.
const brandTrayIconDir = "build/windows/brands"

// brandTrayIcons embeds the per-brand tray icons (regenerate them with
// installer/icons/generate.sh). The active brand is resolved at runtime from
// the executable's base name — the MSI ships one binary renamed per brand —
// so the tray icon has to be selected at runtime from this embedded set.
//
//go:embed build/windows/brands
var brandTrayIcons embed.FS

// TrayIconForBrand returns the embedded tray icon for a brand. When the brand
// has no icon of its own it falls back to the default brand's icon, and
// ultimately to TrayIconData.
func TrayIconForBrand(b Brand) []byte {
	key := normalizeBrandKey(b.Name)
	if key == "" {
		key = defaultBrandKey
	}
	if data, err := brandTrayIcons.ReadFile(brandTrayIconDir + "/" + key + ".ico"); err == nil && len(data) > 0 {
		return data
	}
	if data, err := brandTrayIcons.ReadFile(brandTrayIconDir + "/" + defaultBrandKey + ".ico"); err == nil && len(data) > 0 {
		return data
	}
	return TrayIconData
}
