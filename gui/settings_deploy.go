package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// The Windows installer sets up many computers from one settings file:
// msiexec ... CONFIGFILE=<path of a Preferences > Export file> runs the
// installed service program as SYSTEM with -apply-settings <path> before the
// service starts (installer/DEPLOYMENT.md). It reads the file where the
// administrator keeps it and merges it as Import would; secrets are sealed
// as they are written (Config.Save). Nothing is copied into the settings
// folder first: a plain-text copy there could be read, or swapped, by another
// account before the folder is locked down.

// maxSettingsFile caps what -apply-settings reads; an export is a few KB.
const maxSettingsFile = 10 << 20

// applySettingsArg finds -apply-settings <path> (or --apply-settings,
// -apply-settings=<path>) in a program's arguments.
func applySettingsArg(args []string) (string, bool) {
	for i, a := range args {
		name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if name != "apply-settings" || !strings.HasPrefix(a, "-") {
			continue
		}
		if hasVal {
			return val, true
		}
		if i+1 < len(args) {
			return args[i+1], true
		}
		return "", true
	}
	return "", false
}

// runApplySettings is the -apply-settings command: it prints what it did and
// returns the exit code (0 applied, 1 not), which fails and rolls back the
// install when the file cannot be used.
func runApplySettings(path string) int {
	a := NewApp()
	a.headless = true
	servers, sets, err := a.applySettingsFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "settings not applied: %v\n", err)
		return 1
	}
	fmt.Printf("settings applied from %s: %d PBS server(s), %d Backup Set(s)\n", path, servers, sets)
	return 0
}

// applySettingsFile locks the settings folder down (as the service does when
// it starts, so the secrets are never written into a folder other accounts
// can read), then merges the Export Settings file at path into it for this
// computer (mergeSettingsBundle with forDeployment).
func (a *App) applySettingsFile(path string) (servers, sets int, err error) {
	folderOK := true // false: write nothing into the settings folder
	defer func() {
		if err != nil {
			writeDebugLog(fmt.Sprintf("[Deploy] settings from %s not applied: %v", path, err))
			if folderOK {
				LogMessage("App", "error", "Settings from the installer could not be applied: "+err.Error(), "",
					MsgDeploySettingsFailed, msgParams{"error": err.Error()})
			}
		}
	}()
	if path == "" {
		return 0, 0, fmt.Errorf("no settings file given")
	}
	dir, err := getConfigDir()
	if err != nil {
		return 0, 0, err
	}
	// A folder made into a link (by another account, before the install)
	// would take the settings, and the permissions set below, elsewhere.
	if fi, lerr := os.Lstat(dir); lerr != nil || !fi.IsDir() || fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		folderOK = false
		return 0, 0, fmt.Errorf("the settings folder %s is not a plain folder", dir)
	}
	if err := hardenConfigDir(dir); err != nil {
		return 0, 0, fmt.Errorf("locking down %s: %w", dir, err)
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxSettingsFile+1))
	f.Close()
	if err != nil {
		return 0, 0, err
	}
	if len(raw) > maxSettingsFile {
		return 0, 0, fmt.Errorf("%s is larger than a settings export can be", path)
	}
	bundle, err := parseSettingsBundle(raw)
	if err != nil {
		return 0, 0, err
	}
	if a.config == nil {
		a.config = LoadConfig()
	}
	servers, sets, err = a.mergeSettingsBundle(bundle, true)
	if err != nil {
		return servers, sets, err
	}
	for id, srv := range bundle.Config.PBSServers {
		if srv.Secret == "" && srv.Password == "" {
			writeDebugLog(fmt.Sprintf("[Deploy] PBS server %q came without a token or password (exported without them?): backups to it fail until one is entered", id))
		}
	}
	writeDebugLog(fmt.Sprintf("[Deploy] settings from %s applied: %d PBS server(s), %d Backup Set(s) (exported from %q)", path, servers, sets, bundle.SourceHost))
	LogMessage("App", "info", fmt.Sprintf("Settings from the installer applied (PBS servers: %d, Backup Sets: %d).", servers, sets), "",
		MsgDeploySettingsApplied, msgParams{"servers": servers, "sets": sets})
	return servers, sets, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// hostSlug is a computer name as the Backup Set editor puts it at the start of
// the backup IDs it suggests (frontend backupId.js slugId; computer names are
// plain ASCII, so its accent folding is not needed here).
func hostSlug(name string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// deployedBackupID gives a Backup Set's backup ID on the computer the
// settings are deployed to. The exporting computer's own name becomes empty
// (this computer's name at run time), and an ID the editor suggested for it,
// "<that computer>-<set>", becomes "<this computer>-<set>". Any other ID is
// kept: one meant to be shared, or one machine's.
func deployedBackupID(id, sourceHost, thisHost string) string {
	if sourceHost == "" || id == "" {
		return id
	}
	if strings.EqualFold(id, sourceHost) {
		return ""
	}
	src := hostSlug(sourceHost)
	if src == "" || !strings.HasPrefix(strings.ToLower(id), src+"-") {
		return id
	}
	this := hostSlug(thisHost)
	if this == "" {
		this = "host"
	}
	out := this + id[len(src):]
	if len(out) > 60 {
		out = strings.TrimRight(out[:60], "-")
	}
	return out
}
