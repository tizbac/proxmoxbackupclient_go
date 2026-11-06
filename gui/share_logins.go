package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// Logins for network shares (Preferences > Network Shares).
//
// On Windows the backups run in the background service, as SYSTEM, which
// reaches another computer's share as this computer's own account. In a
// workgroup that account is all but anonymous, so a folder on \\server\share
// that needs a user name and password could not be backed up at all once the
// backups moved into the service (found 2026-11-06; when the app ran them, it
// read the share as the signed-in user). A login is saved per server, as
// Windows allows one user name per server for each logon session anyway, and
// the service connects to a share with it before it reads it (the same as
// net use), and lets go when it is done.
//
// The password is sealed in config.json like the PBS secrets
// (config_secrets.go), never goes to the window (sanitized), and changes only
// through SetShareLogin and DeleteShareLogin, so a settings save from the
// window, which does not hold it, cannot lose it.

// useShareLogins: only the service connects with a saved login. The app reads
// shares as the signed-in user, who has their own connections to the server;
// connecting there under another user name would clash with them (Windows
// takes one per server and logon session). A variable, as are the two
// functions below, so tests can stand in for Windows.
var (
	useShareLogins  = serviceBuild
	shareConnect    = connectShare
	shareDisconnect = disconnectShare
	shareOpenable   = openableDir
)

// ShareLogin is the user name and password for one file server's shares.
type ShareLogin struct {
	Server   string `json:"server"` // as in \\server\share
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	// PasswordSet says a password is stored, in the copies the window gets
	// (which never hold the password itself).
	PasswordSet bool `json:"password_set,omitempty"`
}

// shareKey is the map key of a server's login: servers are named without
// regard to case.
func shareKey(server string) string { return strings.ToLower(strings.TrimSpace(server)) }

// splitShare parses a UNC path into its server and its share root
// (\\server\share). It reports false for anything else, the long form
// \\?\UNC\server\share\... included as a share.
func splitShare(path string) (server, root string, ok bool) {
	p := strings.ReplaceAll(path, "/", `\`)
	switch {
	case strings.HasPrefix(strings.ToUpper(p), `\\?\UNC\`):
		p = `\\` + p[len(`\\?\UNC\`):]
	case strings.HasPrefix(p, `\\?\`), strings.HasPrefix(p, `\\.\`):
		return "", "", false
	case !strings.HasPrefix(p, `\\`):
		return "", "", false
	}
	parts := strings.SplitN(p[2:], `\`, 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], `\\` + parts[0] + `\` + parts[1], true
}

// driveExists reports whether drive ("Z:") exists for this program.
func driveExists(drive string) bool {
	_, err := os.Stat(drive + `\`)
	return err == nil || !os.IsNotExist(err)
}

// resolveMappedDrives is dirs with each folder on a drive letter mapped to a
// share as the share's own path (resolveMappedDrive; Windows, in the user's
// program). The others are left as they are.
func resolveMappedDrives(dirs []string) []string {
	if len(dirs) == 0 {
		return dirs
	}
	out := make([]string, len(dirs))
	for i, d := range dirs {
		out[i] = resolveMappedDrive(d)
		if out[i] != d {
			writeDebugLog(fmt.Sprintf("Folder %s is on a mapped drive: kept as %s", d, out[i]))
		}
	}
	return out
}

// sanitizedShareLogins is logins without their passwords, for the window.
func sanitizedShareLogins(logins map[string]*ShareLogin) map[string]*ShareLogin {
	if logins == nil {
		return nil
	}
	out := make(map[string]*ShareLogin, len(logins))
	for k, l := range logins {
		if l == nil {
			continue
		}
		// The app's copy (from the service) has no password, only the mark.
		out[k] = &ShareLogin{Server: l.Server, Username: l.Username, PasswordSet: l.Password != "" || l.PasswordSet}
	}
	return out
}

// copyShareLogins is a deep copy of logins.
func copyShareLogins(logins map[string]*ShareLogin) map[string]*ShareLogin {
	if logins == nil {
		return nil
	}
	out := make(map[string]*ShareLogin, len(logins))
	for k, l := range logins {
		if l == nil {
			continue
		}
		c := *l
		out[k] = &c
	}
	return out
}

// ListShareLogins is the saved logins, without their passwords, sorted by
// server.
func (a *App) ListShareLogins() []ShareLogin {
	var list []ShareLogin
	if a.config == nil {
		return list
	}
	for _, l := range sanitizedShareLogins(a.config.ShareLogins) {
		list = append(list, *l)
	}
	sort.Slice(list, func(i, j int) bool { return shareKey(list[i].Server) < shareKey(list[j].Server) })
	return list
}

// SetShareLogin saves the login for server, after trying it on testPath (a
// folder on one of its shares) when one is given, so a wrong password is
// found while the user is there and not at the next backup. An empty
// password keeps the stored one (the window never has it).
func (a *App) SetShareLogin(server, username, password, testPath string) error {
	if a.delegated() {
		return a.serviceCall("SetShareLogin", server, username, password, testPath)
	}
	if !useShareLogins {
		return fmt.Errorf("network share logins are for the background service, which runs the backups; without it, backups read shares as you")
	}
	server = strings.TrimSpace(server)
	server = strings.TrimLeft(server, `\/`)
	if server == "" || strings.ContainsAny(server, `\/`) {
		return fmt.Errorf("give the server's name, as in \\\\server\\share")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("a user name is needed for %s", server)
	}
	key := shareKey(server)
	if password == "" {
		if old := a.config.ShareLogins[key]; old != nil {
			password = old.Password
		}
	}
	if password == "" {
		return fmt.Errorf("a password is needed for %s", server)
	}
	login := &ShareLogin{Server: server, Username: username, Password: password}
	if testPath != "" {
		if s, _, ok := splitShare(testPath); !ok || shareKey(s) != key {
			return fmt.Errorf("%s is not on \\\\%s", testPath, server)
		}
		if err := tryShareLogin(login, testPath); err != nil {
			return err
		}
	}
	if a.config.ShareLogins == nil {
		a.config.ShareLogins = map[string]*ShareLogin{}
	}
	a.config.ShareLogins[key] = login
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save the login for %s: %w", server, err)
	}
	writeDebugLog(fmt.Sprintf("Network share login saved for \\\\%s (user %s)", server, username))
	return nil
}

// DeleteShareLogin removes the saved login for server.
func (a *App) DeleteShareLogin(server string) error {
	if a.delegated() {
		return a.serviceCall("DeleteShareLogin", server)
	}
	key := shareKey(strings.TrimLeft(server, `\/`))
	if a.config.ShareLogins[key] == nil {
		return fmt.Errorf("no login is saved for %s", server)
	}
	delete(a.config.ShareLogins, key)
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to remove the login for %s: %w", server, err)
	}
	writeDebugLog(fmt.Sprintf("Network share login removed for \\\\%s", server))
	return nil
}

// ShareAccess is whether the backups can read one folder on a share.
type ShareAccess struct {
	Path      string `json:"path"`
	Server    string `json:"server"`
	OK        bool   `json:"ok"`
	HasLogin  bool   `json:"has_login"`
	NeedLogin bool   `json:"need_login"` // refused for want of a login
	Error     string `json:"error,omitempty"`
}

// CheckShareAccess tries each folder of paths that is on a share, as the
// backups would: with the server's saved login if there is one. The window
// asks before it saves a Backup Set or starts a backup, and asks for a login
// where one is needed. Folders not on a share are left out.
func (a *App) CheckShareAccess(paths []string) ([]ShareAccess, error) {
	// A folder on a mapped drive letter is checked as the share it is on,
	// as the set will keep it (resolveMappedDrives; only the user's own
	// program sees the letter).
	paths = resolveMappedDrives(paths)
	if a.delegated() {
		var out []ShareAccess
		err := a.apiClient.Call("CheckShareAccess", &out, paths)
		return out, err
	}
	var out []ShareAccess
	for _, p := range paths {
		server, _, ok := splitShare(p)
		if !ok {
			continue
		}
		r := ShareAccess{Path: p, Server: server}
		var login *ShareLogin
		if a.config != nil && useShareLogins {
			login = a.config.ShareLogins[shareKey(server)]
		}
		r.HasLogin = login != nil
		var err error
		if login != nil {
			err = tryShareLogin(login, p)
		} else {
			err = shareOpenable(p)
		}
		if err == nil {
			r.OK = true
		} else {
			r.Error = err.Error()
			// Only the service can use a login; here the share is read as
			// the user, who simply has no access.
			r.NeedLogin = useShareLogins && isLoginRefusal(err)
		}
		out = append(out, r)
	}
	return out, nil
}

// shareUse counts the runs using each share connection, so one run ending
// does not cut off another reading the same share; shareUsedAs is the login
// each one was made with.
var (
	shareUseMu  sync.Mutex
	shareUse    = map[string]int{}        // share root, lower case
	shareUsedAs = map[string]ShareLogin{} // share root, lower case
)

// connectShares connects to the share of each path in paths that has a
// saved login, for a backup, and returns the function that lets them go
// again. A share without a login is left to the account the program runs
// as. A login that fails is in failed, by share root in lower case; the
// folders on that share then fail on their own, and the others go on.
func connectShares(logins map[string]*ShareLogin, paths []string) (release func(), failed map[string]error) {
	failed = map[string]error{}
	if !useShareLogins {
		return func() {}, failed
	}
	var held []string
	release = func() {
		for _, root := range held {
			releaseShare(root)
		}
		held = nil
	}
	seen := map[string]bool{}
	for _, p := range paths {
		server, root, ok := splitShare(p)
		if !ok || seen[strings.ToLower(root)] {
			continue
		}
		seen[strings.ToLower(root)] = true
		login := logins[shareKey(server)]
		if login == nil {
			continue
		}
		if err := acquireShare(root, login); err != nil {
			failed[strings.ToLower(root)] = fmt.Errorf("connecting to %s as %s: %w", root, login.Username, err)
			continue
		}
		held = append(held, root)
	}
	return release, failed
}

// shareFailure is why the saved login for path's share failed to connect,
// or nil.
func shareFailure(failed map[string]error, path string) error {
	if _, root, ok := splitShare(path); ok {
		return failed[strings.ToLower(root)]
	}
	return nil
}

func acquireShare(root string, login *ShareLogin) error {
	key := strings.ToLower(root)
	shareUseMu.Lock()
	defer shareUseMu.Unlock()
	if shareUse[key] == 0 {
		if err := shareConnect(root, login.Username, login.Password); err != nil {
			return err
		}
		shareUsedAs[key] = *login
	}
	shareUse[key]++
	return nil
}

func releaseShare(root string) {
	key := strings.ToLower(root)
	shareUseMu.Lock()
	defer shareUseMu.Unlock()
	if shareUse[key] == 0 {
		return
	}
	shareUse[key]--
	if shareUse[key] == 0 {
		delete(shareUse, key)
		delete(shareUsedAs, key)
		shareDisconnect(root)
	}
}

// tryShareLogin connects to the share of path with login, checks the folder
// opens and lists, and lets the share go again. While a run holds the share,
// its connection is used, which is only a test of login if it is the same
// login: Windows takes one user name per server at a time, so another one
// cannot be tried until the run has finished.
func tryShareLogin(login *ShareLogin, path string) error {
	_, root, ok := splitShare(path)
	if !ok {
		return fmt.Errorf("%s is not on a network share", path)
	}
	if !useShareLogins {
		return shareOpenable(path)
	}
	key := strings.ToLower(root)
	shareUseMu.Lock()
	defer shareUseMu.Unlock()
	if shareUse[key] > 0 {
		if used := shareUsedAs[key]; used.Username != login.Username || used.Password != login.Password {
			return fmt.Errorf("a backup is reading %s with the saved login; try the new one when it has finished", root)
		}
		return shareOpenable(path)
	}
	if err := shareConnect(root, login.Username, login.Password); err != nil {
		return err
	}
	defer shareDisconnect(root)
	return shareOpenable(path)
}
