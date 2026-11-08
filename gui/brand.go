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
//
// Every name the program uses on disk comes from the brand too (identity.go):
// FileBase names the programs (salvanza-gui, salvanza-svc) and Folder the
// settings, data and log folders (C:\ProgramData\Salvanza, ~/.salvanza), so
// two brands never share settings.
type Brand struct {
	Name           string `json:"name"`
	Title          string `json:"title"`            // the full product name, as shortcuts, windows and the tray show it
	ShortTitle     string `json:"short_title"`      // where space is short (notifications, e-mail subjects)
	FileBase       string `json:"file_base"`        // program names: FileBase + "-gui", "-svc"
	Folder         string `json:"folder"`           // folder names: C:\ProgramData\<Folder>, ~/.<lower-case Folder>
	Logo           string `json:"logo"`             // asset URL relative to the frontend root; "" = bundled default logo
	Accent         string `json:"accent"`           // primary button/accent color (hex)
	AccentHover    string `json:"accent_hover"`     // hover/pressed variant (hex)
	BrandURL       string `json:"brand_url"`        // brand home page; also the fallback for Urls["about"]
	BuyStorageURL  string `json:"buy_storage_url"`  // the brand sells storage here: a "Buy Backup Storage" button; "" = no button, the page offers the PBS download instead (frontend aboutBrand.js)
	BuyStorageText string `json:"buy_storage_text"` // the button's label; "" = the translated buyBackupStorage
	// Urls holds the About-tab / support links for this brand. Keys:
	// "about", "help", "updates", "contact". Any key may be omitted; the
	// frontend falls back to the upstream Proxmox destinations. These are the
	// same destinations the WiX ARP* (Programs & Features) properties carry,
	// so keep both in sync per brand (see installer/wix).
	Urls      map[string]string `json:"urls"`
	IsDefault bool              `json:"is_default"`
}

const defaultBrandKey = "salvanza"

// Where the project lives: its site, and the repository with the releases.
const (
	projectSite = "https://salvanza.org/"
	projectRepo = "https://github.com/salvanza/salvanza_client"
)

// brandCatalog maps a brand key to a brand. The key is the program's file
// name without its extension and its "-gui" or "-svc" ending (brandKeyOf), so
// the installer names the programs after the brand: salvanza-gui.exe and
// salvanza-svc.exe are "salvanza", nimbusbackup-gui.exe is "nimbusbackup".
var brandCatalog = map[string]Brand{
	defaultBrandKey: {
		Name:       defaultBrandKey,
		Title:      "Salvanza for Proxmox Backup Server",
		ShortTitle: "Salvanza",
		FileBase:   "salvanza",
		Folder:     "Salvanza",
		Logo:       "", // bundled gui/frontend/src/assets/logo.webp
		// Green, the Theme picker's "green" preset: the default until a user
		// chooses another (ThemePicker.jsx).
		Accent:      "#2e8c47",
		AccentHover: "#256f38",
		BrandURL:    projectSite,
		Urls: map[string]string{
			"about":   projectSite,
			"help":    projectRepo,
			"updates": projectRepo + "/releases",
			"contact": projectRepo + "/issues",
		},
	},
	"nimbusbackup": {
		Name:           "nimbusbackup",
		Title:          "Nimbus Backup",
		ShortTitle:     "Nimbus Backup",
		FileBase:       "nimbusbackup",
		Folder:         "NimbusBackup",
		Logo:           "/brands/nimbus.svg",
		Accent:         "#22c55e",
		AccentHover:    "#16a34a",
		BrandURL:       "https://nimbus.rdem-systems.com/en/",
		BuyStorageURL:  "https://nimbus.rdem-systems.com/en/",
		BuyStorageText: "Get Nimbus storage",
		Urls: map[string]string{
			"about":   "https://nimbus.rdem-systems.com/en/",
			"help":    "https://nimbus.rdem-systems.com/en/proxmox-backup-server/",
			"updates": "https://nimbus.rdem-systems.com/en/blog/",
			"contact": "https://nimbus.rdem-systems.com/en/contact/",
		},
	},
	"etitechbackup": {
		Name:           "etitechbackup",
		Title:          "Etitech Backup",
		ShortTitle:     "Etitech Backup",
		FileBase:       "etitechbackup",
		Folder:         "EtitechBackup",
		Logo:           "/brands/etitech.webp",
		Accent:         "#4CC1CE",
		AccentHover:    "#3aaab6",
		BrandURL:       "https://www.etitech.net/",
		BuyStorageURL:  "https://www.etitech.net/servizi.php",
		BuyStorageText: "",
		Urls: map[string]string{
			"about":   "https://www.etitech.net/about.php",
			"help":    "https://www.etitech.net/servizi.php",
			"updates": "https://www.etitech.net/",
			"contact": "https://www.etitech.net/contatti.php",
		},
	},
}

// legacyBrandKeys maps the program names of releases before the rename to
// today's brand: Proxmox Backup Client became Salvanza.
var legacyBrandKeys = map[string]string{
	"proxmoxbackupclient": defaultBrandKey,
}

// normalizeBrandKey is the brand key of a program's file name: lower case,
// without the extension and without the "-gui" or "-svc" ending (or the
// "svc" one of the names before the rename), so "salvanza-gui.exe",
// "salvanza-svc", "NimbusBackup.exe" and "NimbusBackupSVC.exe" give
// "salvanza", "salvanza", "nimbusbackup" and "nimbusbackup".
func normalizeBrandKey(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		n = n[:i]
	}
	for _, suffix := range []string{"-gui", "-svc", "svc"} {
		if strings.HasSuffix(n, suffix) && len(n) > len(suffix) {
			n = strings.TrimSuffix(n, suffix)
			break
		}
	}
	if k, ok := legacyBrandKeys[n]; ok {
		n = k
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
// default Salvanza identity when the name is unknown.
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
