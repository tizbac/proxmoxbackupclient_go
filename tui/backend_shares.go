package tui

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ShareLogin is the saved login for one file server's network shares. The
// service never gives the password out: PasswordSet says one is stored.
type ShareLogin struct {
	Server, Username string
	PasswordSet      bool
}

// ShareCheck is whether the backups can read one folder on a share.
type ShareCheck struct {
	Path      string `json:"path"`
	Server    string `json:"server"`
	OK        bool   `json:"ok"`
	HasLogin  bool   `json:"has_login"`
	NeedLogin bool   `json:"need_login"`
	Error     string `json:"error,omitempty"`
}

// shareLoginsFromConfig reads the logins of the service's settings document.
func shareLoginsFromConfig(doc map[string]interface{}) []ShareLogin {
	raw, _ := doc["share_logins"].(map[string]interface{})
	var out []ShareLogin
	for _, v := range raw {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, ShareLogin{Server: str(m, "server"), Username: str(m, "username"), PasswordSet: boolean(m, "password_set")})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Server) < strings.ToLower(out[j].Server) })
	return out
}

// writeNewFile writes data to path with owner-only access, never over a file
// that exists: a settings export can hold secrets, and a typo must not destroy
// another file.
func writeNewFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- the path the user typed
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists: choose another name, it is not overwritten", path)
		}
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	return f.Close()
}

func (b *serviceBackend) ShareLogins() ([]ShareLogin, error) {
	doc, err := b.c.GetFullConfig()
	if err != nil {
		return nil, err
	}
	return shareLoginsFromConfig(doc), nil
}

func (b *serviceBackend) SetShareLogin(server, username, password, testPath string) error {
	return b.c.Call("SetShareLogin", nil, server, username, password, testPath)
}

func (b *serviceBackend) DeleteShareLogin(server string) error {
	return b.c.Call("DeleteShareLogin", nil, server)
}

func (b *serviceBackend) CheckShares(paths []string) ([]ShareCheck, error) {
	var out []ShareCheck
	err := b.c.Call("CheckShareAccess", &out, paths)
	return out, err
}

func (b *serviceBackend) ExportSettings(path string, withSecrets bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("give the file to write")
	}
	data, err := b.c.ExportSettings(withSecrets)
	if err != nil {
		return err
	}
	return writeNewFile(path, data)
}

func (b *serviceBackend) ImportSettings(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the path the user typed
	if err != nil {
		return "", err
	}
	return b.c.ImportSettings(data)
}

// The demo keeps its logins and its settings in memory.

func (d *demoBackend) ShareLogins() ([]ShareLogin, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.shares == nil {
		d.shares = []ShareLogin{{Server: "nas01", Username: "backup", PasswordSet: true}}
	}
	out := append([]ShareLogin(nil), d.shares...)
	return out, nil
}

func (d *demoBackend) SetShareLogin(server, username, password, testPath string) error {
	server = strings.TrimLeft(strings.TrimSpace(server), `\/`)
	if server == "" || strings.ContainsAny(server, `\/`) {
		return errors.New(`give the server's name, as in \\server\share`)
	}
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("a user name is needed for %s", server)
	}
	if _, err := d.ShareLogins(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, l := range d.shares {
		if strings.EqualFold(l.Server, server) {
			if password == "" && !l.PasswordSet {
				return fmt.Errorf("a password is needed for %s", server)
			}
			d.shares[i] = ShareLogin{Server: server, Username: strings.TrimSpace(username), PasswordSet: true}
			return nil
		}
	}
	if password == "" {
		return fmt.Errorf("a password is needed for %s", server)
	}
	d.shares = append(d.shares, ShareLogin{Server: server, Username: strings.TrimSpace(username), PasswordSet: true})
	return nil
}

func (d *demoBackend) DeleteShareLogin(server string) error {
	if _, err := d.ShareLogins(); err != nil {
		return err
	}
	server = strings.TrimLeft(strings.TrimSpace(server), `\/`)
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, l := range d.shares {
		if strings.EqualFold(l.Server, server) {
			d.shares = append(d.shares[:i], d.shares[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("no login is saved for %s", server)
}

func (d *demoBackend) CheckShares(paths []string) ([]ShareCheck, error) {
	logins, _ := d.ShareLogins()
	var out []ShareCheck
	for _, p := range paths {
		q := strings.ReplaceAll(p, "/", `\`)
		if !strings.HasPrefix(q, `\\`) {
			continue
		}
		server := strings.SplitN(q[2:], `\`, 2)[0]
		r := ShareCheck{Path: p, Server: server}
		for _, l := range logins {
			if strings.EqualFold(l.Server, server) {
				r.HasLogin = true
			}
		}
		if r.HasLogin {
			r.OK = true
		} else {
			r.Error = "access is denied"
			r.NeedLogin = true
		}
		out = append(out, r)
	}
	return out, nil
}

func (d *demoBackend) ExportSettings(path string, withSecrets bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("give the file to write")
	}
	return writeNewFile(path, []byte(`{"demo":true}`))
}

func (d *demoBackend) ImportSettings(path string) (string, error) {
	if _, err := os.ReadFile(path); err != nil { // #nosec G304 -- the path the user typed
		return "", err
	}
	return "Imported the demo settings: nothing was changed.", nil
}
