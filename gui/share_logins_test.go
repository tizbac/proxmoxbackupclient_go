//go:build !service

package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeShares stands in for Windows: connect and disconnect are recorded,
// connecting fails for a wrong password, and a folder opens once its share
// is connected (or always, with open).
type fakeShares struct {
	connects, disconnects []string
	connected             map[string]bool
	password              string // the one that works
	open                  bool
}

func useFakeShares(t *testing.T, password string) *fakeShares {
	t.Helper()
	f := &fakeShares{connected: map[string]bool{}, password: password}
	savedUse, savedC, savedD, savedO := useShareLogins, shareConnect, shareDisconnect, shareOpenable
	useShareLogins = true
	shareConnect = func(root, user, pass string) error {
		if pass != f.password {
			return errors.New("the user name or password is wrong")
		}
		f.connects = append(f.connects, root+" as "+user)
		f.connected[strings.ToLower(root)] = true
		return nil
	}
	shareDisconnect = func(root string) {
		f.disconnects = append(f.disconnects, root)
		delete(f.connected, strings.ToLower(root))
	}
	shareOpenable = func(path string) error {
		if f.open {
			return nil
		}
		if _, root, ok := splitShare(path); ok && f.connected[strings.ToLower(root)] {
			return nil
		}
		return os.ErrPermission
	}
	t.Cleanup(func() {
		useShareLogins, shareConnect, shareDisconnect, shareOpenable = savedUse, savedC, savedD, savedO
		shareUseMu.Lock()
		shareUse, shareUsedAs = map[string]int{}, map[string]ShareLogin{}
		shareUseMu.Unlock()
	})
	return f
}

func TestSplitShare(t *testing.T) {
	for _, c := range []struct {
		path, server, root string
		ok                 bool
	}{
		{`\\deepthought\backup\Test Files`, "deepthought", `\\deepthought\backup`, true},
		{`\\nas\share`, "nas", `\\nas\share`, true},
		{`//nas/share/x`, "nas", `\\nas\share`, true},
		{`\\?\UNC\nas\share\x`, "nas", `\\nas\share`, true},
		{`\\nas`, "", "", false},
		{`\\?\C:\x`, "", "", false},
		{`\\.\PhysicalDrive0`, "", "", false},
		{`C:\Users`, "", "", false},
		{`/home/x`, "", "", false},
	} {
		server, root, ok := splitShare(c.path)
		if server != c.server || root != c.root || ok != c.ok {
			t.Errorf("splitShare(%q) = %q, %q, %v; want %q, %q, %v", c.path, server, root, ok, c.server, c.root, c.ok)
		}
	}
}

// A login is tried before it is saved; a wrong password is refused and not
// saved; an empty password keeps the stored one; the window's copy of the
// settings never holds the password.
func TestSetShareLogin(t *testing.T) {
	strictHome(t)
	f := useFakeShares(t, "right")
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}

	if err := a.SetShareLogin("Deepthought", "DEEPTHOUGHT\\mick", "wrong", `\\deepthought\backup\Test Files`); err == nil {
		t.Fatal("a wrong password was saved")
	}
	if len(a.config.ShareLogins) != 0 {
		t.Fatalf("a refused login is stored: %+v", a.config.ShareLogins)
	}
	if err := a.SetShareLogin(`\\Deepthought`, "DEEPTHOUGHT\\mick", "right", `\\deepthought\backup\Test Files`); err != nil {
		t.Fatal(err)
	}
	if got := f.connects; len(got) != 1 || got[0] != `\\deepthought\backup as DEEPTHOUGHT\mick` {
		t.Errorf("connects %q", got)
	}
	if len(f.connected) != 0 {
		t.Errorf("the test connection was not let go: %v", f.connected)
	}
	l := a.config.ShareLogins["deepthought"]
	if l == nil || l.Server != "Deepthought" || l.Password != "right" {
		t.Fatalf("stored %+v", l)
	}
	if c := LoadConfig(); c.ShareLogins["deepthought"] == nil || c.ShareLogins["deepthought"].Password != "right" {
		t.Errorf("not saved to config.json: %+v", c.ShareLogins)
	}

	// The user name changed, the password left empty: kept.
	if err := a.SetShareLogin("deepthought", "mick", "", ""); err != nil {
		t.Fatal(err)
	}
	if l := a.config.ShareLogins["deepthought"]; l.Username != "mick" || l.Password != "right" {
		t.Errorf("after a change without a password: %+v", l)
	}

	doc := a.GetFullConfigForAPI()
	raw, _ := json.Marshal(doc)
	if strings.Contains(string(raw), "right") {
		t.Errorf("the settings document holds the password: %s", raw)
	}
	if !strings.Contains(string(raw), `"password_set":true`) {
		t.Errorf("the settings document does not say a password is stored: %s", raw)
	}
	list := a.ListShareLogins()
	if len(list) != 1 || list[0].Password != "" || !list[0].PasswordSet {
		t.Errorf("ListShareLogins = %+v", list)
	}

	for _, bad := range [][4]string{
		{"", "u", "p", ""},
		{`\\a\b`, "u", "p", ""},
		{"nas", "", "p", ""},
		{"newserver", "u", "", ""},
		{"nas", "u", "right", `\\other\share\x`},
	} {
		if err := a.SetShareLogin(bad[0], bad[1], bad[2], bad[3]); err == nil {
			t.Errorf("SetShareLogin%q accepted", bad)
		}
	}

	if err := a.DeleteShareLogin(`\\DEEPTHOUGHT`); err != nil {
		t.Fatal(err)
	}
	if len(a.config.ShareLogins) != 0 || len(LoadConfig().ShareLogins) != 0 {
		t.Error("the login is still there after DeleteShareLogin")
	}
	if err := a.DeleteShareLogin("deepthought"); err == nil {
		t.Error("deleting a login that is not there gave no error")
	}
}

// Without the service (the app runs the backups as the user) a login is
// refused, and shares are only checked as the user.
func TestShareLoginsAppOnly(t *testing.T) {
	strictHome(t)
	useFakeShares(t, "right")
	useShareLogins = false
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}
	if err := a.SetShareLogin("nas", "u", "right", ""); err == nil {
		t.Error("the app saved a share login without the service")
	}
	res, err := a.CheckShareAccess([]string{`\\nas\share\x`})
	if err != nil || len(res) != 1 || res[0].OK || res[0].NeedLogin {
		t.Errorf("CheckShareAccess = %+v, %v; want refused, no login asked for", res, err)
	}
	release, failed := connectShares(map[string]*ShareLogin{"nas": {Server: "nas", Username: "u", Password: "right"}}, []string{`\\nas\share\x`})
	release()
	if len(failed) != 0 {
		t.Errorf("failed %v", failed)
	}
}

func TestCheckShareAccess(t *testing.T) {
	strictHome(t)
	f := useFakeShares(t, "right")
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{}}}

	res, err := a.CheckShareAccess([]string{`C:\local`, `\\nas\share\x`})
	if err != nil || len(res) != 1 {
		t.Fatalf("CheckShareAccess = %+v, %v; want one result (the share)", res, err)
	}
	if r := res[0]; r.OK || !r.NeedLogin || r.HasLogin || r.Server != "nas" {
		t.Errorf("no login: %+v", r)
	}

	a.config.ShareLogins = map[string]*ShareLogin{"nas": {Server: "nas", Username: "u", Password: "right"}}
	res, _ = a.CheckShareAccess([]string{`\\nas\share\x`})
	if r := res[0]; !r.OK || !r.HasLogin {
		t.Errorf("with the login: %+v", r)
	}
	if len(f.connected) != 0 {
		t.Errorf("the check left the share connected: %v", f.connected)
	}

	a.config.ShareLogins["nas"].Password = "changed on the server"
	res, _ = a.CheckShareAccess([]string{`\\nas\share\x`})
	if r := res[0]; r.OK || !r.HasLogin || r.Error == "" {
		t.Errorf("with a login that no longer works: %+v", r)
	}
}

// Two runs on one share connect once and let go when the last one ends; a
// share whose login fails is reported, and the other shares still connect.
func TestConnectShares(t *testing.T) {
	f := useFakeShares(t, "right")
	logins := map[string]*ShareLogin{
		"nas": {Server: "nas", Username: "u", Password: "right"},
		"old": {Server: "old", Username: "u", Password: "stale"},
	}
	dirs := []string{`\\nas\share\a`, `\\NAS\share\b`, `\\old\share\c`, `\\other\share\d`, `C:\e`}

	rel1, failed := connectShares(logins, dirs)
	if len(f.connects) != 1 || !f.connected[`\\nas\share`] {
		t.Fatalf("connects %q", f.connects)
	}
	if len(failed) != 1 || shareFailure(failed, `\\old\share\c`) == nil || shareFailure(failed, `\\nas\share\a`) != nil {
		t.Errorf("failed %v", failed)
	}
	rel2, _ := connectShares(logins, []string{`\\nas\share\z`})
	if len(f.connects) != 1 {
		t.Errorf("a second run connected again: %q", f.connects)
	}
	rel1()
	if !f.connected[`\\nas\share`] {
		t.Error("the first run ending cut the second one off")
	}
	// A different login cannot be tried while a run holds the share.
	if err := tryShareLogin(&ShareLogin{Server: "nas", Username: "v", Password: "right"}, `\\nas\share\a`); err == nil {
		t.Error("another login was tried on a share a run holds")
	}
	if err := tryShareLogin(logins["nas"], `\\nas\share\a`); err != nil {
		t.Errorf("the run's own login on its share: %v", err)
	}
	rel2()
	if f.connected[`\\nas\share`] || len(f.disconnects) != 1 {
		t.Errorf("not let go after the last run: connected %v, disconnects %q", f.connected, f.disconnects)
	}
	rel2() // twice is harmless
	if len(f.disconnects) != 1 {
		t.Errorf("disconnects %q", f.disconnects)
	}
}

// A settings save from the window (which has no passwords) keeps the logins.
func TestSaveConfigKeepsShareLogins(t *testing.T) {
	strictHome(t)
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{},
		BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!t", Secret: "s", Datastore: "store",
		ShareLogins: map[string]*ShareLogin{"nas": {Server: "nas", Username: "u", Password: "secret"}}}}
	if err := a.config.Save(); err != nil {
		t.Fatal(err)
	}
	fromWindow := a.config.sanitized()
	fromWindow.ShareLogins["nas"].Username = "changed by a stale window"
	if err := a.SaveConfig(fromWindow); err != nil {
		t.Fatal(err)
	}
	if l := LoadConfig().ShareLogins["nas"]; l == nil || l.Password != "secret" || l.Username != "u" {
		t.Errorf("after SaveConfig: %+v", l)
	}
}

// An export without secrets has the logins without passwords; with them,
// the passwords too. Importing a file without the password keeps the one
// stored here.
func TestShareLoginsExportImport(t *testing.T) {
	strictHome(t)
	a := &App{config: &Config{PBSServers: map[string]*PBSServer{},
		ShareLogins: map[string]*ShareLogin{"nas": {Server: "nas", Username: "u", Password: "secret"}}}}
	if err := a.config.Save(); err != nil {
		t.Fatal(err)
	}
	without, err := a.buildSettingsExport(false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(without), "secret") || !strings.Contains(string(without), `"username": "u"`) {
		t.Errorf("export without secrets: %s", without)
	}
	with, _ := a.buildSettingsExport(true)
	if !strings.Contains(string(with), "secret") {
		t.Error("export with secrets has no share password")
	}
	if a.config.ShareLogins["nas"].Password != "secret" {
		t.Error("exporting changed the stored password")
	}

	a.config.ShareLogins["nas"].Password = "here"
	bundle, err := parseSettingsBundle(without)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Config.ShareLogins["other"] = &ShareLogin{Server: "other", Username: "x"} // no password: left out
	if _, _, err := a.mergeSettingsBundle(bundle, false); err != nil {
		t.Fatal(err)
	}
	if l := a.config.ShareLogins["nas"]; l.Password != "here" {
		t.Errorf("import without a password: %+v", l)
	}
	if a.config.ShareLogins["other"] != nil {
		t.Error("a login with no password anywhere was imported")
	}
}

// The service runs the share calls for the app, with typed arguments.
func TestShareLoginServiceCalls(t *testing.T) {
	for _, name := range []string{"SetShareLogin", "DeleteShareLogin", "CheckShareAccess"} {
		if _, ok := apiCalls[name]; !ok {
			t.Errorf("%s is not on the service's list", name)
		}
	}
	useFakeShares(t, "right")
	svc := strictServiceApp(t)
	if _, err := svc.CallForAPI("SetShareLogin", json.RawMessage(`["nas","u"]`)); err == nil {
		t.Error("SetShareLogin with two arguments accepted")
	}
	if _, err := svc.CallForAPI("SetShareLogin", json.RawMessage(`["nas","u","right",""]`)); err != nil {
		t.Fatal(err)
	}
	if l := svc.config.ShareLogins["nas"]; l == nil || l.Password != "right" {
		t.Errorf("service login %+v", l)
	}
	out, err := svc.CallForAPI("CheckShareAccess", json.RawMessage(`[["\\\\nas\\share\\x"]]`))
	if err != nil || !strings.Contains(string(out), `"ok":true`) {
		t.Errorf("CheckShareAccess = %s, %v", out, err)
	}
	if _, err := svc.CallForAPI("DeleteShareLogin", json.RawMessage(`["nas"]`)); err != nil || svc.config.ShareLogins["nas"] != nil {
		t.Errorf("DeleteShareLogin: %v, %+v", err, svc.config.ShareLogins)
	}
}

func TestResolveMappedDrivesLeavesOthers(t *testing.T) {
	in := []string{`/home/x`, `\\nas\share`, `relative`}
	out := resolveMappedDrives(in)
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("resolveMappedDrives changed %q to %q", in[i], out[i])
		}
	}
}
