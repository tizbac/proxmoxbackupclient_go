package main

import (
	"embed"
)

// TrayIconData embeds the default icon.ico, the same icon as the executable.
// It is the last fallback; the tray normally shows the brand's own icon.
//
//go:embed build/windows/icon.ico
var TrayIconData []byte

// brandTrayIconDir is where the per-brand tray icons live in the binary.
const brandTrayIconDir = "build/windows/brands"

// brandTrayIcons holds one tray icon per brand, named by brand key
// (regenerate them with installer/icons/generate.sh). One binary ships under
// each brand's executable name, so the icon is picked at runtime.
//
//go:embed build/windows/brands
var brandTrayIcons embed.FS

// TrayIconForBrand returns the brand's tray icon, else the default brand's,
// else TrayIconData.
func TrayIconForBrand(b Brand) []byte {
	key := normalizeBrandKey(b.Name)
	if key == "" {
		key = defaultBrandKey
	}
	for _, k := range []string{key, defaultBrandKey} {
		if data, err := brandTrayIcons.ReadFile(brandTrayIconDir + "/" + k + ".ico"); err == nil && len(data) > 0 {
			return data
		}
	}
	return TrayIconData
}
