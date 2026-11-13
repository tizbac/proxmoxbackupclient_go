package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func boolPtr(v bool) *bool { return &v }

// switchText says a switch the way a failed test should print it.
func switchText(v *bool) string {
	switch {
	case v == nil:
		return "unset"
	case *v:
		return "on"
	}
	return "off"
}

func TestCompressEncryptedOn(t *testing.T) {
	if !compressEncryptedOn(nil) {
		t.Error("unset is not on")
	}
	if !compressEncryptedOn(boolPtr(true)) {
		t.Error("true is not on")
	}
	if compressEncryptedOn(boolPtr(false)) {
		t.Error("false is on")
	}
}

// Settings an earlier version left: every server with a key starts off, with
// the question to ask; a server without a key, and one already set, are left.
func TestDecideCompressEncryptedEarlierSettings(t *testing.T) {
	c := &Config{PBSServers: map[string]*PBSServer{
		"b":     {ID: "b", Name: "Office", EncryptionKeyFile: "/keys/b.json"},
		"a":     {ID: "a", EncryptionKeyFile: "/keys/a.json"},
		"plain": {ID: "plain", Name: "No key"},
		"set":   {ID: "set", Name: "Already on", EncryptionKeyFile: "/keys/s.json", CompressEncrypted: boolPtr(true)},
		"gone":  nil,
	}}
	got := c.decideCompressEncrypted(false)
	if want := []string{"Office", "a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("turned off %v, want %v (names, the ID where there is none, sorted)", got, want)
	}
	if !c.CompressEncryptedDecided {
		t.Error("not marked as decided")
	}
	for _, id := range []string{"a", "b"} {
		s := c.PBSServers[id]
		if compressEncryptedOn(s.CompressEncrypted) || s.CompressEncrypted == nil || !s.CompressEncryptedAsk {
			t.Errorf("server %s: switch %s, ask %v; want off and to be asked", id, switchText(s.CompressEncrypted), s.CompressEncryptedAsk)
		}
	}
	if s := c.PBSServers["plain"]; s.CompressEncrypted != nil || s.CompressEncryptedAsk {
		t.Errorf("a server without a key was given a switch: %s, ask %v", switchText(s.CompressEncrypted), s.CompressEncryptedAsk)
	}
	if s := c.PBSServers["set"]; !compressEncryptedOn(s.CompressEncrypted) || s.CompressEncryptedAsk {
		t.Errorf("a switch already set was changed: %s, ask %v", switchText(s.CompressEncrypted), s.CompressEncryptedAsk)
	}
	if c.CompressEncrypted != nil {
		t.Errorf("no key beside the servers, yet the switch there is %s", switchText(c.CompressEncrypted))
	}

	// Decided once: a key added later is not turned off by a second pass.
	c.PBSServers["plain"].EncryptionKeyFile = "/keys/p.json"
	if again := c.decideCompressEncrypted(false); again != nil {
		t.Errorf("a second pass turned off %v", again)
	}
	if s := c.PBSServers["plain"]; s.CompressEncrypted != nil {
		t.Errorf("a second pass set the switch to %s", switchText(s.CompressEncrypted))
	}
}

// The single server of settings from before there were several has its key
// beside the list: its switch goes off with it.
func TestDecideCompressEncryptedSingleServerSettings(t *testing.T) {
	c := &Config{
		BaseURL:           "https://pbs.example:8007",
		EncryptionKeyFile: "/keys/k.json",
		DefaultPBSID:      "default",
		PBSServers: map[string]*PBSServer{
			"default": {ID: "default", Name: "Main PBS Server", EncryptionKeyFile: "/keys/k.json"},
		},
	}
	if got := c.decideCompressEncrypted(false); !reflect.DeepEqual(got, []string{"Main PBS Server"}) {
		t.Fatalf("turned off %v", got)
	}
	if c.CompressEncrypted == nil || *c.CompressEncrypted {
		t.Errorf("the switch beside the list is %s, want off", switchText(c.CompressEncrypted))
	}
}

// An install that replaced a client which already wrote the compressed form
// stays on, with nothing to ask.
func TestDecideCompressEncryptedAfterCompressingClient(t *testing.T) {
	c := &Config{
		EncryptionKeyFile: "/keys/k.json",
		PBSServers: map[string]*PBSServer{
			"a": {ID: "a", EncryptionKeyFile: "/keys/a.json"},
		},
	}
	if got := c.decideCompressEncrypted(true); got != nil {
		t.Fatalf("turned off %v", got)
	}
	if !c.CompressEncryptedDecided {
		t.Error("not marked as decided")
	}
	if s := c.PBSServers["a"]; s.CompressEncrypted != nil || s.CompressEncryptedAsk {
		t.Errorf("server: switch %s, ask %v; want unset (on) and no question", switchText(s.CompressEncrypted), s.CompressEncryptedAsk)
	}
	if c.CompressEncrypted != nil {
		t.Errorf("the switch beside the list is %s, want unset (on)", switchText(c.CompressEncrypted))
	}
}

func TestCompressEncryptedOffNotice(t *testing.T) {
	got := compressEncryptedOffNotice("Office")
	for _, want := range []string{"Office", "not compressed", "2.0.0", "Compress encrypted backups", "restore ISO"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q does not say %q", got, want)
		}
	}
}

func TestSettleCompressEncrypted(t *testing.T) {
	cases := []struct {
		name     string
		existing PBSServer
		update   PBSServer
		want     *bool
		wantAsk  bool
	}{
		{
			name:     "an update that does not say the switch keeps it and the question",
			existing: PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			update:   PBSServer{},
			want:     boolPtr(false), wantAsk: true,
		},
		{
			name:     "an update that does not say the switch keeps on",
			existing: PBSServer{CompressEncrypted: boolPtr(true)},
			update:   PBSServer{},
			want:     boolPtr(true),
		},
		{
			name:     "an update that does not say the switch keeps unset",
			existing: PBSServer{},
			update:   PBSServer{CompressEncryptedAsk: true},
			want:     nil,
		},
		{
			name:     "the answer turn on ends the question",
			existing: PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			update:   PBSServer{CompressEncrypted: boolPtr(true), CompressEncryptedAsk: true},
			want:     boolPtr(true),
		},
		{
			name:     "the answer keep off ends the question",
			existing: PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			update:   PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: false},
			want:     boolPtr(false),
		},
		{
			name:     "another edit of a server not yet asked about keeps the question",
			existing: PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			update:   PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			want:     boolPtr(false), wantAsk: true,
		},
		{
			name:     "turning a switch off by hand asks nothing",
			existing: PBSServer{CompressEncrypted: boolPtr(true)},
			update:   PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			want:     boolPtr(false),
		},
		{
			name:     "turning an unset switch off by hand asks nothing",
			existing: PBSServer{},
			update:   PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			want:     boolPtr(false),
		},
		{
			name:     "an update never raises the question",
			existing: PBSServer{CompressEncrypted: boolPtr(false)},
			update:   PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			want:     boolPtr(false),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			existing, s := tc.existing, tc.update
			settleCompressEncrypted(&existing, &s)
			if switchText(s.CompressEncrypted) != switchText(tc.want) || s.CompressEncryptedAsk != tc.wantAsk {
				t.Errorf("switch %s, ask %v; want %s, ask %v", switchText(s.CompressEncrypted), s.CompressEncryptedAsk, switchText(tc.want), tc.wantAsk)
			}
		})
	}
}

func TestFollowDefaultCompressEncrypted(t *testing.T) {
	// Settings with only the list: nothing beside it to keep in step.
	c := &Config{DefaultPBSID: "a", PBSServers: map[string]*PBSServer{
		"a": {ID: "a", CompressEncrypted: boolPtr(false)},
	}}
	c.followDefaultCompressEncrypted()
	if c.CompressEncrypted != nil {
		t.Errorf("settings with no single server were given the switch %s", switchText(c.CompressEncrypted))
	}

	// Settings from before there were several: the default server's switch.
	c.BaseURL = "https://pbs.example:8007"
	c.followDefaultCompressEncrypted()
	if c.CompressEncrypted == nil || *c.CompressEncrypted {
		t.Errorf("the switch beside the list is %s, want the default server's off", switchText(c.CompressEncrypted))
	}
	c.PBSServers["b"] = &PBSServer{ID: "b"}
	c.DefaultPBSID = "b"
	c.followDefaultCompressEncrypted()
	if c.CompressEncrypted != nil {
		t.Errorf("the switch beside the list is %s, want the new default server's unset", switchText(c.CompressEncrypted))
	}
}

func TestImportedCompressEncrypted(t *testing.T) {
	cases := []struct {
		name          string
		existing      *PBSServer
		imported      PBSServer
		fileHasSwitch bool
		want          *bool
		wantAsk       bool
	}{
		{
			name:     "a new server with a key from a file written before the switch starts off, to be asked",
			imported: PBSServer{EncryptionKeyFile: "/keys/k.json"},
			want:     boolPtr(false), wantAsk: true,
		},
		{
			name:          "a new server with a key from a file written with the switch is as the file says: unset is on",
			imported:      PBSServer{EncryptionKeyFile: "/keys/k.json"},
			fileHasSwitch: true,
			want:          nil,
		},
		{
			name:          "a switch the file says is kept",
			imported:      PBSServer{EncryptionKeyFile: "/keys/k.json", CompressEncrypted: boolPtr(false)},
			fileHasSwitch: true,
			want:          boolPtr(false),
		},
		{
			name:     "a server this computer has keeps its switch and its question",
			existing: &PBSServer{CompressEncrypted: boolPtr(false), CompressEncryptedAsk: true},
			imported: PBSServer{EncryptionKeyFile: "/keys/k.json"},
			want:     boolPtr(false), wantAsk: true,
		},
		{
			name:     "a server this computer has on stays on",
			existing: &PBSServer{CompressEncrypted: boolPtr(true)},
			imported: PBSServer{EncryptionKeyFile: "/keys/k.json"},
			want:     boolPtr(true),
		},
		{
			name:     "a server without a key gets no switch",
			imported: PBSServer{},
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.imported
			importedCompressEncrypted(tc.existing, &s, tc.fileHasSwitch)
			if switchText(s.CompressEncrypted) != switchText(tc.want) || s.CompressEncryptedAsk != tc.wantAsk {
				t.Errorf("switch %s, ask %v; want %s, ask %v", switchText(s.CompressEncrypted), s.CompressEncryptedAsk, switchText(tc.want), tc.wantAsk)
			}
		})
	}
}

// The switch travels with the key to wherever a run reads its server from.
func TestCompressEncryptedReachesTheRun(t *testing.T) {
	srv := &PBSServer{ID: "a", BaseURL: "https://pbs.example:8007", Datastore: "store", EncryptionKeyFile: "/keys/k.json", CompressEncrypted: boolPtr(false)}
	if got := srv.ToConfig().CompressEncrypted; got == nil || *got {
		t.Errorf("ToConfig: switch %s, want off", switchText(got))
	}
	c := &Config{DefaultPBSID: "a", PBSServers: map[string]*PBSServer{"a": srv}}
	if got := c.EffectivePBS().CompressEncrypted; got == nil || *got {
		t.Errorf("EffectivePBS: switch %s, want off", switchText(got))
	}
}

// unlockCrypt hands a run a key that writes the way the server's switch says.
func TestUnlockCryptFollowsCompressEncrypted(t *testing.T) {
	key := writeKeyFile(t, t.TempDir(), "k.json")
	cases := []struct {
		name string
		sw   *bool
		want bool
	}{
		{"unset", nil, true},
		{"on", boolPtr(true), true},
		{"off", boolPtr(false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{EncryptionKeyFile: key, CompressEncrypted: tc.sw}
			if err := c.unlockCrypt(false); err != nil {
				t.Fatal(err)
			}
			if c.Crypt == nil {
				t.Fatal("no key unlocked")
			}
			if c.Crypt.Compresses() != tc.want {
				t.Errorf("the key compresses: %v, want %v", c.Crypt.Compresses(), tc.want)
			}
		})
	}
}

// configDirForSwitchTest is an empty service settings folder for one test.
func configDirForSwitchTest(t *testing.T) string {
	t.Helper()
	programData := t.TempDir()
	useServiceDirForTest(t)
	t.Setenv("ProgramData", programData)
	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(filepath.Dir(dir), programData) {
		t.Fatalf("settings folder %s is not in %s", dir, programData)
	}
	return dir
}

func savedConfigForSwitchTest(t *testing.T, dir string) Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	return saved
}

// The first LoadConfig of an earlier version's settings turns the switch off
// for the server with a key and saves that, so it is decided once: a server
// added afterwards starts on, and the next load changes nothing.
func TestLoadConfigDecidesCompressEncryptedOnce(t *testing.T) {
	dir := configDirForSwitchTest(t)
	key := writeKeyFile(t, t.TempDir(), "k.json")
	raw, err := json.Marshal(&Config{DefaultPBSID: "old", PBSServers: map[string]*PBSServer{
		"old":   {ID: "old", Name: "Old", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!t", Secret: "x", Datastore: "store", EncryptionKeyFile: key},
		"plain": {ID: "plain", Name: "Plain", BaseURL: "https://pbs2.example:8007", AuthID: "backup@pbs!t", Secret: "x", Datastore: "store"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "compress_encrypted") {
		t.Fatalf("settings as an earlier version wrote them mention the switch: %s", raw)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	c := LoadConfig()
	check := func(what string, cfg *Config) {
		t.Helper()
		if !cfg.CompressEncryptedDecided {
			t.Errorf("%s: not marked as decided", what)
		}
		old := cfg.PBSServers["old"]
		if old == nil || old.CompressEncrypted == nil || *old.CompressEncrypted || !old.CompressEncryptedAsk {
			t.Errorf("%s: the server with a key is %+v, want the switch off and to be asked", what, old)
		}
		if plain := cfg.PBSServers["plain"]; plain == nil || plain.CompressEncrypted != nil || plain.CompressEncryptedAsk {
			t.Errorf("%s: the server without a key is %+v, want no switch", what, plain)
		}
	}
	check("loaded", c)
	saved := savedConfigForSwitchTest(t, dir)
	check("saved", &saved)

	// A server added now starts on, whatever the caller says about asking.
	added := &PBSServer{ID: "new", Name: "New", BaseURL: "https://pbs3.example:8007", AuthID: "backup@pbs!t", Secret: "x", Datastore: "store", EncryptionKeyFile: key, CompressEncryptedAsk: true}
	if err := c.AddPBSServer(added); err != nil {
		t.Fatal(err)
	}
	again := LoadConfig()
	check("loaded again", again)
	if s := again.PBSServers["new"]; s == nil || s.CompressEncrypted != nil || s.CompressEncryptedAsk {
		t.Errorf("the server added afterwards is %+v, want the switch unset (on) and no question", s)
	}

	// The answer "turn on" through an update ends the question for good.
	answer := *again.PBSServers["old"]
	answer.CompressEncrypted = boolPtr(true)
	if err := again.UpdatePBSServer(&answer); err != nil {
		t.Fatal(err)
	}
	final := LoadConfig()
	if s := final.PBSServers["old"]; s == nil || !compressEncryptedOn(s.CompressEncrypted) || s.CompressEncryptedAsk {
		t.Errorf("after the answer the server is %+v, want on and no question", s)
	}
}

// With no settings at all there is nothing an earlier version wrote: the
// first server with a key starts on and is never asked about.
func TestLoadConfigFreshInstallCompressesEncrypted(t *testing.T) {
	dir := configDirForSwitchTest(t)
	key := writeKeyFile(t, t.TempDir(), "k.json")
	c := LoadConfig()
	if !c.CompressEncryptedDecided {
		t.Fatal("a fresh install is not marked as decided")
	}
	if err := c.AddPBSServer(&PBSServer{ID: "a", Name: "A", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!t", Secret: "x", Datastore: "store", EncryptionKeyFile: key}); err != nil {
		t.Fatal(err)
	}
	saved := savedConfigForSwitchTest(t, dir)
	if !saved.CompressEncryptedDecided {
		t.Error("the saved settings are not marked as decided")
	}
	loaded := LoadConfig()
	if s := loaded.PBSServers["a"]; s == nil || s.CompressEncrypted != nil || s.CompressEncryptedAsk {
		t.Errorf("the server is %+v, want the switch unset (on) and no question", s)
	}
}

// Importing a settings file (Preferences, Import, and the installer's
// deployment file): a server with a key from a file written before the switch
// comes in off, to be asked about; from a file written with it, as the file
// says; and a server this computer already has keeps its own.
func TestImportSettingsGivesServersTheirSwitch(t *testing.T) {
	key := writeKeyFile(t, t.TempDir(), "k.json")
	server := func(id string, keyed bool) *PBSServer {
		s := &PBSServer{ID: id, Name: id, BaseURL: "https://" + id + ".example:8007", AuthID: "backup@pbs!t", Secret: "x", Datastore: "store"}
		if keyed {
			s.EncryptionKeyFile = key
		}
		return s
	}
	bundle := func(decided bool) SettingsBundle {
		return SettingsBundle{
			BundleVersion: currentSettingsBundleVersion,
			SourceHost:    "SRC-PC",
			Config: &Config{
				CompressEncryptedDecided: decided,
				DefaultPBSID:             "keyed",
				PBSServers:               map[string]*PBSServer{"keyed": server("keyed", true), "plain": server("plain", false)},
			},
		}
	}

	for _, deploy := range []bool{false, true} {
		// a file from before the switch, onto a computer with no settings
		configDirForSwitchTest(t)
		a := &App{config: LoadConfig()}
		if _, _, err := a.mergeSettingsBundle(bundle(false), deploy); err != nil {
			t.Fatal(err)
		}
		got := LoadConfig()
		if s := got.PBSServers["keyed"]; s == nil || compressEncryptedOn(s.CompressEncrypted) || !s.CompressEncryptedAsk {
			t.Errorf("deployment %v, file from before the switch: the server with a key is %+v, want off and to be asked", deploy, s)
		}
		if s := got.PBSServers["plain"]; s == nil || s.CompressEncrypted != nil || s.CompressEncryptedAsk {
			t.Errorf("deployment %v, file from before the switch: the server without a key is %+v, want no switch", deploy, s)
		}

		// a file written with the switch, which leaves it unset: on
		configDirForSwitchTest(t)
		a = &App{config: LoadConfig()}
		if _, _, err := a.mergeSettingsBundle(bundle(true), deploy); err != nil {
			t.Fatal(err)
		}
		got = LoadConfig()
		if s := got.PBSServers["keyed"]; s == nil || s.CompressEncrypted != nil || s.CompressEncryptedAsk {
			t.Errorf("deployment %v, file with the switch: the server with a key is %+v, want unset (on) and no question", deploy, s)
		}

		// and with the switch off in it: off, as the file says
		configDirForSwitchTest(t)
		a = &App{config: LoadConfig()}
		b := bundle(true)
		b.Config.PBSServers["keyed"].CompressEncrypted = boolPtr(false)
		if _, _, err := a.mergeSettingsBundle(b, deploy); err != nil {
			t.Fatal(err)
		}
		got = LoadConfig()
		if s := got.PBSServers["keyed"]; s == nil || compressEncryptedOn(s.CompressEncrypted) || s.CompressEncryptedAsk {
			t.Errorf("deployment %v, file with the switch off: the server with a key is %+v, want off and no question", deploy, s)
		}

		// a server this computer has, answered "turn on" here, stays on when
		// a file from before the switch is imported over it
		configDirForSwitchTest(t)
		a = &App{config: LoadConfig()}
		here := server("keyed", true)
		here.CompressEncrypted = boolPtr(true)
		if err := a.config.AddPBSServer(here); err != nil {
			t.Fatal(err)
		}
		if _, _, err := a.mergeSettingsBundle(bundle(false), deploy); err != nil {
			t.Fatal(err)
		}
		got = LoadConfig()
		if s := got.PBSServers["keyed"]; s == nil || !compressEncryptedOn(s.CompressEncrypted) || s.CompressEncryptedAsk {
			t.Errorf("deployment %v, import over a server answered here: %+v, want on and no question", deploy, s)
		}
	}
}
