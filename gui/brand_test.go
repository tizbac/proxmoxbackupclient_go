package main

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests keep the brand catalogue (brand.go), the tray icons and the
// installer's brand files (installer/wix/*.wxs, installer/icons) in step.
// From upstream, which shipped a branded MSI whose app fell back to the
// default brand, and branded icons that were copies of the default one.

var (
	wxsServiceRe = regexp.MustCompile(`<\?define\s+ServiceExeName\s*=\s*"([^"]+)"\s*\?>`)
	wxsProductRe = regexp.MustCompile(`<\?define\s+ProductName\s*=\s*"([^"]+)"\s*\?>`)
	wxsExeRe     = regexp.MustCompile(`<\?define\s+ExeName\s*=\s*"([^"]+)"\s*\?>`)
)

// wxsBrand is one brand's installer file.
type wxsBrand struct {
	file, exe, service, product, content string
}

// wixBrands reads every brand .wxs in installer/wix (files without an
// ExeName, ServiceExeName and ProductName are not brand files).
func wixBrands(t *testing.T) []wxsBrand {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "installer", "wix", "*.wxs"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no WiX files in ../installer/wix (%v)", err)
	}
	var out []wxsBrand
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		c := string(data)
		exe, svc, prod := wxsExeRe.FindStringSubmatch(c), wxsServiceRe.FindStringSubmatch(c), wxsProductRe.FindStringSubmatch(c)
		if exe == nil || svc == nil || prod == nil {
			continue
		}
		out = append(out, wxsBrand{file: filepath.Base(f), exe: exe[1], service: svc[1], product: prod[1], content: c})
	}
	if len(out) == 0 {
		t.Fatal("no brand .wxs files found")
	}
	return out
}

// The installed exe's name picks the brand, so each brand MSI must install
// an exe the catalogue knows, titled as the MSI's product.
func TestWiXExeNameResolvesToItsOwnBrand(t *testing.T) {
	for _, w := range wixBrands(t) {
		key := normalizeBrandKey(w.exe)
		b, ok := brandCatalog[key]
		if !ok {
			t.Errorf("%s installs %s.exe, but brandCatalog has no %q: the app would show the default brand", w.file, w.exe, key)
			continue
		}
		if b.Title != w.product {
			t.Errorf("%s: ProductName %q, but the app titles itself %q", w.file, w.product, b.Title)
		}
	}
}

var wxsFolderRe = regexp.MustCompile(`<\?define\s+Folder\s*=\s*"([^"]+)"\s*\?>`)

// Each MSI installs the names the app derives from its brand (identity.go):
// <FileBase>-gui.exe and <FileBase>-svc.exe, the service registered under
// <FileBase>-svc (service_main.go, service_control_windows.go), in
// C:\Program Files\<Folder>.
func TestWiXNamesMatchTheBrand(t *testing.T) {
	for _, w := range wixBrands(t) {
		b, ok := brandCatalog[normalizeBrandKey(w.exe)]
		if !ok {
			continue // reported by TestWiXExeNameResolvesToItsOwnBrand
		}
		if want := b.FileBase + "-gui"; w.exe != want {
			t.Errorf("%s: ExeName %q, want %q", w.file, w.exe, want)
		}
		if want := b.FileBase + "-svc"; w.service != want {
			t.Errorf("%s: ServiceExeName %q, want %q", w.file, w.service, want)
		}
		if m := wxsFolderRe.FindStringSubmatch(w.content); m == nil || m[1] != b.Folder {
			t.Errorf("%s: Folder %v, want %q", w.file, m, b.Folder)
		}
	}
}

// Every brand's names come from its FileBase and Folder, and no two brands
// share a program or settings folder name.
func TestBrandNamesAreDistinct(t *testing.T) {
	files, folders := map[string]string{}, map[string]string{}
	for key, b := range brandCatalog {
		if b.FileBase == "" || b.Folder == "" || b.ShortTitle == "" {
			t.Errorf("brand %q: FileBase %q, Folder %q, ShortTitle %q: none may be empty", key, b.FileBase, b.Folder, b.ShortTitle)
		}
		if b.FileBase != strings.ToLower(b.FileBase) || strings.ContainsAny(b.FileBase, " .\\/") {
			t.Errorf("brand %q: FileBase %q must be lower case, without spaces or dots", key, b.FileBase)
		}
		if other, dup := files[b.FileBase]; dup {
			t.Errorf("brands %q and %q share FileBase %q", key, other, b.FileBase)
		}
		files[b.FileBase] = key
		if other, dup := folders[strings.ToLower(b.Folder)]; dup {
			t.Errorf("brands %q and %q share Folder %q", key, other, b.Folder)
		}
		folders[strings.ToLower(b.Folder)] = key
		if normalizeBrandKey(b.FileBase+"-gui.exe") != key || normalizeBrandKey(b.FileBase+"-svc.exe") != key {
			t.Errorf("brand %q: its own program names do not resolve to it", key)
		}
	}
}

// The MSI's Programs and Features links and the About page's links are the
// same destinations (brand.go's Urls), so they must not drift apart.
func TestWiXArpUrlsMatchBrandUrls(t *testing.T) {
	defines := map[string]string{
		"ArpUrlAbout":      "about",
		"ArpHelpLink":      "help",
		"ArpUrlUpdateInfo": "updates",
		"ArpContact":       "contact",
	}
	for _, w := range wixBrands(t) {
		b, ok := brandCatalog[normalizeBrandKey(w.exe)]
		if !ok {
			continue // reported by TestWiXExeNameResolvesToItsOwnBrand
		}
		for define, section := range defines {
			re := regexp.MustCompile(`<\?define\s+` + define + `\s*=\s*"([^"]+)"\s*\?>`)
			m := re.FindStringSubmatch(w.content)
			if m == nil {
				t.Errorf("%s: no %s define", w.file, define)
				continue
			}
			if m[1] != b.Urls[section] {
				t.Errorf("%s: %s is %q, but brand %q's %s link is %q", w.file, define, m[1], b.Name, section, b.Urls[section])
			}
		}
	}
}

// A brand logo is a URL under /brands/, served from frontend/public.
func TestBrandLogosAreShipped(t *testing.T) {
	for key, b := range brandCatalog {
		if b.Logo == "" {
			continue // the default brand uses the bundled logo
		}
		if !strings.HasPrefix(b.Logo, "/brands/") {
			t.Errorf("brand %q: logo %q must be a /brands/ path", key, b.Logo)
			continue
		}
		p := filepath.Join("frontend", "public", filepath.FromSlash(strings.TrimPrefix(b.Logo, "/")))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("brand %q: logo %q is not shipped: %v", key, b.Logo, err)
		}
	}
}

func TestBrandCatalogEntriesAreComplete(t *testing.T) {
	for key, b := range brandCatalog {
		if b.Name != key {
			t.Errorf("brand %q: Name %q must match its key", key, b.Name)
		}
		if b.Title == "" {
			t.Errorf("brand %q: no Title", key)
		}
		for name, colour := range map[string]string{"Accent": b.Accent, "AccentHover": b.AccentHover} {
			if len(colour) != 7 || !strings.HasPrefix(colour, "#") {
				t.Errorf("brand %q: %s %q is not a #rrggbb colour", key, name, colour)
			}
		}
		requireAbsoluteHTTPURL(t, key, "BrandURL", b.BrandURL)
		for _, section := range []string{"about", "help", "updates", "contact"} {
			requireAbsoluteHTTPURL(t, key, "Urls["+section+"]", b.Urls[section])
		}
		if b.BuyStorageURL != "" {
			requireAbsoluteHTTPURL(t, key, "BuyStorageURL", b.BuyStorageURL)
		}
	}
}

// The app opens these links in the browser, where a relative or scheme-less
// link does nothing useful.
func requireAbsoluteHTTPURL(t *testing.T, key, field, raw string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		t.Errorf("brand %q: %s %q is not an absolute http(s) URL", key, field, raw)
	}
}

func TestResolveBrandNormalizesExeNames(t *testing.T) {
	cases := map[string]string{
		"salvanza-gui.exe":       defaultBrandKey,
		"salvanza-svc":           defaultBrandKey,
		"nimbusbackup-gui.exe":   "nimbusbackup",
		" NimbusBackup-SVC.exe ": "nimbusbackup",
		"etitechbackup-gui.exe":  "etitechbackup",
		"nimbusbackup":           "nimbusbackup",
		// The names before the rename still resolve: a portable copy, or the
		// old service started once more before the new one removes it.
		"EtitechBackup.exe":       "etitechbackup",
		"ETITECHBACKUP.EXE":       "etitechbackup",
		"EtitechBackupSVC.exe":    "etitechbackup",
		"ProxmoxBackupClient.exe": defaultBrandKey,
		"ProxmoxBackupClientSVC":  defaultBrandKey,
		// Unknown names (the Acme brand is gone) get the default brand, never
		// a partial match.
		" AcmeBackup.exe ":      defaultBrandKey,
		"EtitechBackup (1).exe": defaultBrandKey,
		"UnrelatedTool.exe":     defaultBrandKey,
	}
	for exe, wantKey := range cases {
		got := ResolveBrand(exe)
		if got.Name != wantKey || got.IsDefault != (wantKey == defaultBrandKey) {
			t.Errorf("ResolveBrand(%q) = %q (default %v), want %q", exe, got.Name, got.IsDefault, wantKey)
		}
	}
}

// isICO reports whether data starts with the ICO header (00 00 01 00).
func isICO(data []byte) bool {
	return len(data) >= 4 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0
}

// Every brand has its own tray icon, and an unknown brand still gets one.
func TestBrandTrayIconsAreEmbedded(t *testing.T) {
	seen := map[string]string{}
	for key := range brandCatalog {
		got := TrayIconForBrand(ResolveBrand(key))
		if !isICO(got) {
			t.Errorf("brand %q: no tray icon (regenerate with installer/icons/generate.sh)", key)
			continue
		}
		if other, dup := seen[string(got)]; dup {
			t.Errorf("brand %q: tray icon is the same as brand %q's", key, other)
		}
		seen[string(got)] = key
	}
	if !isICO(TrayIconForBrand(ResolveBrand("SomeOtherTool.exe"))) {
		t.Error("an unknown brand got no tray icon")
	}
}

// Each brand's installer icon (installer/icons/<ExeName>.ico) is its own.
func TestInstallerIconsAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, w := range wixBrands(t) {
		p := filepath.Join("..", "installer", "icons", w.exe+".ico")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: %v", w.file, err)
			continue
		}
		if !isICO(data) {
			t.Errorf("%s is not an ICO file", p)
		}
		if other, dup := seen[string(data)]; dup {
			t.Errorf("%s is the same as %s.ico", p, other)
		}
		seen[string(data)] = w.exe
	}
}
