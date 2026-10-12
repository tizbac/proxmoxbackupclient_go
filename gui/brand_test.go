package main

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

// The installed exe does NOT serve frontend/public — it embeds frontend/dist
// (//go:embed all:frontend/dist in main.go). A logo that exists only under
// public/ means the frontend was not rebuilt after the brand asset changed,
// and the packaged WebUI would 404 on it (the About tab's onError used to hide
// the broken image, so a branded build simply showed NO icon).
func TestBrandLogosAreEmbedded(t *testing.T) {
	for key, b := range brandCatalog {
		if b.Logo == "" {
			continue
		}
		p := filepath.Join("frontend", "dist", filepath.FromSlash(strings.TrimPrefix(b.Logo, "/")))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("brand %q: Logo %q is not in the embedded WebUI: %s does not exist "+
				"(rebuild: cd frontend && npm run build): %v", key, b.Logo, p, err)
		}
	}
	// The default brand's bundled logo travels the same way.
	if _, err := os.Stat(filepath.Join("frontend", "dist", "assets", "logo.webp")); err != nil {
		t.Errorf("bundled default logo is not in the embedded WebUI "+
			"(rebuild: cd frontend && npm run build): %v", err)
	}
}

// The WiX ARP* (Programs & Features) URLs and the About-tab link list are two
// views of the same catalog — brand.go's Urls doc says they carry the same
// destinations. If they drift, the installer's links and the WebUI's links
// disagree, and only one of the two gets fixed later.
var wixArpRes = map[string]*regexp.Regexp{
	"ArpUrlAbout":      regexp.MustCompile(`<\?define\s+ArpUrlAbout\s*=\s*"([^"]+)"\s*\?>`),
	"ArpHelpLink":      regexp.MustCompile(`<\?define\s+ArpHelpLink\s*=\s*"([^"]+)"\s*\?>`),
	"ArpUrlUpdateInfo": regexp.MustCompile(`<\?define\s+ArpUrlUpdateInfo\s*=\s*"([^"]+)"\s*\?>`),
	"ArpContact":       regexp.MustCompile(`<\?define\s+ArpContact\s*=\s*"([^"]+)"\s*\?>`),
}

func TestWiXArpUrlsMatchBrandUrls(t *testing.T) {
	sectionFor := map[string]string{
		"ArpUrlAbout":      "about",
		"ArpHelpLink":      "help",
		"ArpUrlUpdateInfo": "updates",
		"ArpContact":       "contact",
	}
	for file := range wixEntries(t) {
		content := mustRead(t, filepath.Join("..", "installer", "wix", wixFileFor(file)))
		exeRe := wxsExeRe.FindStringSubmatch(content)
		if exeRe == nil {
			continue
		}
		brand, ok := brandCatalog[normalizeBrandKey(exeRe[1])]
		if !ok {
			continue // reported by TestWiXExeNameResolvesToItsOwnBrand
		}
		for define, re := range wixArpRes {
			m := re.FindStringSubmatch(content)
			if m == nil {
				t.Errorf("%s.wxs: missing <?define %s?>", file, define)
				continue
			}
			section := sectionFor[define]
			if m[1] != brand.Urls[section] {
				t.Errorf("%s.wxs %s=%q, but the WebUI's %q link opens Urls[%q]=%q — "+
					"the installer and the app disagree about brand %q",
					file, define, m[1], section, section, brand.Urls[section], brand.Name)
			}
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
		} else {
			requireAbsoluteHTTPURL(t, key, "BrandURL", b.BrandURL)
		}
		for _, section := range []string{"about", "help", "updates", "contact"} {
			if b.Urls[section] == "" {
				t.Errorf("brand %q: missing Urls[%q]", key, section)
				continue
			}
			requireAbsoluteHTTPURL(t, key, "Urls[\""+section+"\"]", b.Urls[section])
		}
		if b.BuyStorageURL != "" {
			requireAbsoluteHTTPURL(t, key, "BuyStorageURL", b.BuyStorageURL)
		}
	}
}

// requireAbsoluteHTTPURL guards the links the WebUI hands to OpenBrowser: a
// relative or scheme-less value would resolve against the embedded app's own
// origin (and do nothing useful), and typos like "proxmox.com" without a
// scheme open as a search, not a site.
func requireAbsoluteHTTPURL(t *testing.T, key, field, raw string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Errorf("brand %q: %s=%q is not a valid URL: %v", key, field, raw, err)
		return
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		t.Errorf("brand %q: %s=%q must be an absolute http(s) URL", key, field, raw)
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
