package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Windows service must register under exactly the name the installer
// created it with, otherwise StartServiceCtrlDispatcher fails (error 1061) and
// Windows reports "service failed to start". These tests keep
// serviceIdentityForExeBase in sync with installer/wix/*.wxs.
var (
	wxsNameRe = regexp.MustCompile(`<\?define\s+ServiceExeName\s*=\s*"([^"]+)"\s*\?>`)
	wxsProdRe = regexp.MustCompile(`<\?define\s+ProductName\s*=\s*"([^"]+)"\s*\?>`)
	wxsExeRe  = regexp.MustCompile(`<\?define\s+ExeName\s*=\s*"([^"]+)"\s*\?>`)
)

func wixEntries(t *testing.T) map[string][2]string {
	t.Helper()
	pattern := filepath.Join("..", "installer", "wix", "*.wxs")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no WiX source files found at %s", pattern)
	}

	out := map[string][2]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		content := string(data)
		svc := wxsNameRe.FindStringSubmatch(content)
		prod := wxsProdRe.FindStringSubmatch(content)
		if svc == nil || prod == nil {
			continue // not a brand entry (e.g. a partial file)
		}
		base := strings.TrimSuffix(filepath.Base(f), ".wxs")
		if base == "Product" {
			base = "ProxmoxBackupClient"
		}
		out[base] = [2]string{svc[1], prod[1]}
	}
	return out
}

func TestServiceNameMatchesWiXServiceInstall(t *testing.T) {
	for file, entry := range wixEntries(t) {
		serviceExeName := entry[0]
		name, _ := serviceIdentityForExeBase(serviceExeName)
		if name != serviceExeName {
			t.Errorf("%s.wxs registers service %q but the binary would dispatch %q: "+
				"SCM would reject the start with error 1061",
				file, serviceExeName, name)
		}
	}
}

func TestServiceDisplayNameMatchesWiX(t *testing.T) {
	for file, entry := range wixEntries(t) {
		serviceExeName, productName := entry[0], entry[1]
		_, displayName := serviceIdentityForExeBase(serviceExeName)
		want := productName + " Service"
		if displayName != want {
			t.Errorf("%s.wxs DisplayName would be %q, installer uses %q", file, displayName, want)
		}
	}
}

// ServiceExeName must stay ExeName + "SVC": the build script stages the
// service binary under that name and the GUI brand key is the ExeName.
func TestServiceExeNameIsExeNameWithSvcSuffix(t *testing.T) {
	for file, entry := range wixEntries(t) {
		exeName := wxsExeRe.FindStringSubmatch(mustRead(t, filepath.Join("..", "installer", "wix", wixFileFor(file))))
		if exeName == nil {
			continue
		}
		if want := exeName[1] + "SVC"; entry[0] != want {
			t.Errorf("%s.wxs: ServiceExeName=%q, expected %q", file, entry[0], want)
		}
	}
}

func wixFileFor(base string) string {
	if base == "ProxmoxBackupClient" {
		return "Product.wxs"
	}
	return base + ".wxs"
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
