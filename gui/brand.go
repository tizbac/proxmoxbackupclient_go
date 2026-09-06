package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Brand describes the identity/visual configuration applied to the GUI. The
// active brand is resolved from the running executable's base name, so the same
// build can ship under different vendor identities. Relying on the exe name (and
// not the registry) also makes it work for portable installs: just rename the .exe.
type Brand struct {
	Name           string `json:"name"`
	Title          string `json:"title"`
	Logo           string `json:"logo"`             // asset URL relative to the frontend root; "" = bundled default logo
	Accent         string `json:"accent"`           // primary button/accent color (hex)
	AccentHover    string `json:"accent_hover"`     // hover/pressed variant (hex)
	BrandURL       string `json:"brand_url"`        // brand home page; also the fallback for Urls["about"]
	BuyStorageURL  string `json:"buy_storage_url"`  // "" = buy-storage CTA hidden (falls back to the PBS download link)
	BuyStorageText string `json:"buy_storage_text"` // CTA label; "" = use the i18n orderStorageCTA
	// Urls holds the About-tab / support links for this brand. Keys:
	// "about", "help", "updates", "contact". Any key may be omitted; the
	// frontend falls back to the upstream Proxmox destinations. These are the
	// same destinations the WiX ARP* (Programs & Features) properties carry,
	// so keep both in sync per brand (see installer/wix).
	Urls      map[string]string `json:"urls"`
	IsDefault bool              `json:"is_default"`
}

const defaultBrandKey = "proxmoxbackupclient"

// brandCatalog maps an executable base name (lower-cased, extension stripped) to
// a brand. THIS IS AN EXAMPLE LIST FOR TESTING — fill in the real vendors here.
// The installer is expected to name the GUI executable to match the installer, so
// "NimbusBackup.msi" -> "NimbusBackup.exe" -> key "nimbusbackup".
var brandCatalog = map[string]Brand{
	defaultBrandKey: {
		Name:        defaultBrandKey,
		Title:       "Proxmox Backup Client",
		Logo:        "", // bundled gui/frontend/src/assets/logo.webp
		Accent:      "#e87003",
		AccentHover: "#d46100",
		BrandURL:    "https://www.proxmox.com/",
		Urls: map[string]string{
			"about":   "https://www.proxmox.com/",
			"help":    "https://forum.proxmox.com/",
			"updates": "https://github.com/tizbac/proxmoxbackupclient_go/releases",
			"contact": "https://github.com/tizbac/proxmoxbackupclient_go",
		},
	},
	"nimbusbackup": {
		Name:           "nimbusbackup",
		Title:          "Nimbus Backup",
		Logo:           "/brands/nimbus.svg",
		Accent:         "#22c55e",
		AccentHover:    "#16a34a",
		BrandURL:       "https://nimbus.example/",
		BuyStorageURL:  "https://store.nimbus.example/backup",
		BuyStorageText: "Get Nimbus storage",
		Urls: map[string]string{
			"about":   "https://nimbus.example/",
			"help":    "https://nimbus.example/help",
			"updates": "https://nimbus.example/releases",
			"contact": "https://nimbus.example/contact",
		},
	},
	"acmebackup": {
		Name:           "acmebackup",
		Title:          "Acme Backup",
		Logo:           "/brands/acme.svg",
		Accent:         "#3b82f6",
		AccentHover:    "#2563eb",
		BrandURL:       "https://acme.example/",
		BuyStorageURL:  "https://store.acme.example/backup",
		BuyStorageText: "Buy Acme storage",
		Urls: map[string]string{
			"about":   "https://acme.example/",
			"help":    "https://acme.example/help",
			"updates": "https://acme.example/releases",
			"contact": "https://acme.example/contact",
		},
	},
}

// normalizeBrandKey lower-cases a file name and strips its extension so that
// "NimbusBackup.exe" and "nimbusbackup" both resolve to the same brand key.
func normalizeBrandKey(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		n = n[:i]
	}
	return n
}

// defaultBrand returns a copy of the built-in fallback identity.
func defaultBrand() Brand {
	b := brandCatalog[defaultBrandKey]
	b.IsDefault = true
	return b
}

// ResolveBrand returns the brand for an executable base name, falling back to the
// default "Proxmox Backup Client" identity when the name is unknown.
func ResolveBrand(exeBaseName string) Brand {
	key := normalizeBrandKey(exeBaseName)
	if b, ok := brandCatalog[key]; ok {
		b.IsDefault = key == defaultBrandKey
		return b
	}
	return defaultBrand()
}

// BrandFromExecutable resolves the brand from the base name of the running
// executable. On any error it falls back to the default brand.
func BrandFromExecutable() Brand {
	exePath, err := os.Executable()
	if err != nil {
		return defaultBrand()
	}
	return ResolveBrand(filepath.Base(exePath))
}

// GetBrand is bound to the frontend so the UI can apply the active brand
// (window/header title, logo, accent color and buy-storage CTA).
func (a *App) GetBrand() Brand {
	return BrandFromExecutable()
}
