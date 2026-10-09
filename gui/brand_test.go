package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The installed GUI executable's base name is what selects the brand at
// runtime (see BrandFromExecutable), so a branded installer must name an exe
// the catalog actually knows — otherwise the product installs "Etitech Backup"
// everywhere except in the app itself, which silently renders with the default
// Proxmox identity. That regression shipped once: an exe built before the
// etitech entry existed was still a perfectly valid PE, so the build script
// happily packaged it as EtitechBackup.exe.
func TestWiXExeNameResolvesToItsOwnBrand(t *testing.T) {
	for file, entry := range wixEntries(t) {
		productName := entry[1]
		content := mustRead(t, filepath.Join("..", "installer", "wix", wixFileFor(file)))
		exeRe := wxsExeRe.FindStringSubmatch(content)
		if exeRe == nil {
			continue
		}
		exeName := exeRe[1]

		key := normalizeBrandKey(exeName)
		brand, ok := brandCatalog[key]
		if !ok {
			t.Errorf("%s.wxs installs %s.exe, but brandCatalog has no %q entry — "+
				"the installed app would fall back to the default Proxmox branding",
				file, exeName, key)
			continue
		}
		if key != defaultBrandKey && brand.IsDefault {
			t.Errorf("%s.wxs: ResolveBrand(%q) reports IsDefault=true for a branded key", file, exeName)
		}
		if brand.Title != productName {
			t.Errorf("%s.wxs ProductName=%q but the app would title itself %q",
				file, productName, brand.Title)
		}
	}
}

// Logos are referenced by URL ("/brands/x.svg") and must be served from
// frontend/public — anything placed under frontend/src/assets/ is not reachable
// at that URL, so the About tab would render a broken (hidden) logo.
func TestBrandLogosAreShipped(t *testing.T) {
	for key, b := range brandCatalog {
		if b.Logo == "" {
			continue // default brand uses the bundled logo
		}
		if !strings.HasPrefix(b.Logo, "/brands/") {
			t.Errorf("brand %q: Logo %q must be an absolute /brands/ path", key, b.Logo)
			continue
		}
		p := filepath.Join("frontend", "public", filepath.FromSlash(strings.TrimPrefix(b.Logo, "/")))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("brand %q: Logo %q is not served: %s does not exist (%v)", key, b.Logo, p, err)
		}
	}
}

func TestBrandCatalogEntriesAreComplete(t *testing.T) {
	for key, b := range brandCatalog {
		if b.Name != key {
			t.Errorf("brand %q: Name=%q, must match its catalog key", key, b.Name)
		}
		if b.Title == "" {
			t.Errorf("brand %q: empty Title", key)
		}
		for name, colour := range map[string]string{"Accent": b.Accent, "AccentHover": b.AccentHover} {
			if len(colour) != 7 || !strings.HasPrefix(colour, "#") {
				t.Errorf("brand %q: %s=%q, expected a #rrggbb hex colour", key, name, colour)
			}
		}
		if b.BrandURL == "" {
			t.Errorf("brand %q: empty BrandURL", key)
		}
		for _, section := range []string{"about", "help", "updates", "contact"} {
			if b.Urls[section] == "" {
				t.Errorf("brand %q: missing Urls[%q]", key, section)
			}
		}
	}
}

func TestResolveBrandNormalizesExeNames(t *testing.T) {
	cases := map[string]string{
		"EtitechBackup.exe": "etitechbackup",
		"ETITECHBACKUP.EXE": "etitechbackup",
		" AcmeBackup.exe ":  "acmebackup",
		"nimbusbackup":      "nimbusbackup",
		"EtitechBackup":     "etitechbackup",
		// Unknown names fall back to the default identity, never to a partial
		// or mis-matched brand.
		"EtitechBackup (1).exe": defaultBrandKey,
		"UnrelatedTool.exe":     defaultBrandKey,
	}
	for exe, wantKey := range cases {
		got := ResolveBrand(exe)
		want := brandCatalog[wantKey]
		want.IsDefault = wantKey == defaultBrandKey
		if got.Name != want.Name || got.Title != want.Title || got.IsDefault != want.IsDefault {
			t.Errorf("ResolveBrand(%q) = {%s %q default=%v}, want {%s %q default=%v}",
				exe, got.Name, got.Title, got.IsDefault, want.Name, want.Title, want.IsDefault)
		}
	}
}
