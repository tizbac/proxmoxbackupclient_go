//go:build !service

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// Connecting with own settings (UseOwnSettings): the user's own Backup Sets
// do not run while the app works on the computer's, so Connect says so first
// and offers to copy them, with their PBS servers, into the computer's
// settings, where the service runs them whoever is signed in.

// OwnSetsCount is how many Backup Sets the user has in their own settings,
// while the app works on them (0 otherwise): the window asks before Connect.
func (a *App) OwnSetsCount() int {
	if !a.ownSettings.Load() {
		return 0
	}
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return 0
	}
	return len(jobs)
}

// ComputerServers is the computer's PBS servers without their logins (GET
// /servers/public), for a user on their own settings: Preferences offers
// them so the user can add one with their own token or login instead of
// typing its address, datastore and fingerprint. Only on own settings: on
// the computer's the servers are the user's already. An error when the
// service does not answer, or is older than the route.
func (a *App) ComputerServers() ([]api.PublicServer, error) {
	if !a.ownSettings.Load() {
		return nil, fmt.Errorf("only while working on your own settings")
	}
	c := a.apiClient
	if c == nil {
		c = api.NewClient(getAPITokenPath())
	}
	list, err := c.PublicServers()
	if err != nil {
		return nil, fmt.Errorf("the background service did not give this computer's servers: %w", err)
	}
	return list, nil
}

// ConnectServiceCopyingOwn is Connect from the user's own settings, copying
// their PBS servers and Backup Sets into the computer's once connected (an
// administrator has just approved). Their own copy is left as it was.
func (a *App) ConnectServiceCopyingOwn() error {
	if !a.ownSettings.Load() {
		return a.ConnectService(true)
	}
	ownCfg := a.config
	ownJobs, err := a.GetScheduledJobs()
	if err != nil {
		return fmt.Errorf("reading your own Backup Sets: %w", err)
	}
	return a.connect(true, func() error {
		return a.copyOwnSettingsToService(ownCfg, ownJobs)
	})
}

// copyOwnSettingsToService adds the user's own servers and sets to the
// service's settings (an import, merged by the service). A server the
// computer already has (same address, datastore, namespace and login) is
// used as it is; a server or set whose ID is taken, or a set whose name is,
// gets the user's name added. Sets that used the user's default server name
// it, so they never fall to the computer's default. An encryption key is
// copied into the service's keys folder, which is the only place it uses keys
// from. The computer's default server, e-mail account and theme are left
// alone. The window is told (onServiceConnected) after this.
func (a *App) copyOwnSettingsToService(ownCfg *Config, ownJobs []ScheduledJob) error {
	doc, err := a.apiClient.GetFullConfig()
	if err != nil {
		return err
	}
	svcCfg, err := configFromServiceDoc(doc)
	if err != nil {
		return err
	}
	svcJobs, err := a.GetScheduledJobs()
	if err != nil {
		return err
	}
	who := ownerSlug()
	servers, jobs := planOwnCopy(ownCfg, ownJobs, svcCfg, svcJobs, who)
	for _, s := range servers {
		if s.EncryptionKeyFile == "" {
			continue
		}
		path, err := a.copyKeyToService(s.EncryptionKeyFile, who)
		if err != nil {
			return fmt.Errorf("copying the encryption key of %q: %w", s.Name, err)
		}
		s.EncryptionKeyFile = path
	}

	host, _ := os.Hostname()
	bundle := SettingsBundle{
		BundleVersion: currentSettingsBundleVersion,
		ExportedAt:    time.Now().Format(time.RFC3339),
		AppVersion:    appVersion,
		Config:        &Config{PBSServers: servers, CompressEncryptedDecided: ownCfg.CompressEncryptedDecided},
		ScheduledJobs: jobs,
		SourceHost:    host, // this computer: machine sets keep their disks
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	summary, err := a.apiClient.ImportSettings(data)
	if err != nil {
		return fmt.Errorf("copying your Backup Sets: %w", err)
	}
	writeDebugLog(fmt.Sprintf("[Service] own settings copied to the service: %d PBS server(s), %d Backup Set(s) (%s)", len(servers), len(jobs), summary))
	return nil
}

// copyKeyToService saves a copy of the key file at path in the service's keys
// folder (an existing file there is never overwritten) and returns where.
func (a *App) copyKeyToService(path, who string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the user's own key, named in their own settings
	if err != nil {
		return "", err
	}
	name := who + "-" + filepath.Base(path)
	saved, err := a.apiClient.SaveKey(name, data)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		ext := filepath.Ext(name)
		saved, err = a.apiClient.SaveKey(fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), time.Now().Unix(), ext), data)
	}
	return saved, err
}

// planOwnCopy is what copyOwnSettingsToService adds to the service's
// settings (svc, svcJobs) from the user's own (own, ownJobs): the servers to
// add, by their new IDs, and the sets, pointing at the servers' new IDs.
func planOwnCopy(own *Config, ownJobs []ScheduledJob, svc *Config, svcJobs []ScheduledJob, who string) (map[string]*PBSServer, []ScheduledJob) {
	key := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	unique := func(base string, taken func(string) bool) string {
		id := base
		for n := 1; taken(id); n++ {
			id = fmt.Sprintf("%s-%s", base, who)
			if n > 1 {
				id = fmt.Sprintf("%s-%s-%d", base, who, n)
			}
		}
		return id
	}

	servers := map[string]*PBSServer{}
	idFor := map[string]string{}
	serverNames := map[string]bool{}
	for _, s := range svc.PBSServers {
		if s != nil {
			serverNames[key(s.Name)] = true
		}
	}
	for id, s := range own.PBSServers {
		if s == nil {
			continue
		}
		if same := sameServerIn(svc, s); same != "" {
			idFor[id] = same
			continue
		}
		newID := unique(id, func(v string) bool { return svc.PBSServers[v] != nil || servers[v] != nil })
		cp := *s
		cp.ID = newID
		if serverNames[key(cp.Name)] {
			cp.Name = fmt.Sprintf("%s (%s)", cp.Name, who)
		}
		serverNames[key(cp.Name)] = true
		servers[newID] = &cp
		idFor[id] = newID
	}

	ids := map[string]bool{}
	names := map[string]bool{}
	for _, j := range svcJobs {
		ids[j.ID] = true
		names[key(j.Name)] = true
	}
	jobs := make([]ScheduledJob, 0, len(ownJobs))
	for _, j := range ownJobs {
		j.ID = unique(j.ID, func(v string) bool { return ids[v] })
		if names[key(j.Name)] {
			j.Name = fmt.Sprintf("%s (%s)", j.Name, who)
		}
		server := j.PBSServerID
		if server == "" {
			server = own.DefaultPBSID
		}
		if mapped, ok := idFor[server]; ok {
			j.PBSServerID = mapped
		}
		ids[j.ID] = true
		names[key(j.Name)] = true
		jobs = append(jobs, j)
	}
	return servers, jobs
}

// sameServerIn returns the ID of the server in cfg that is s (same address,
// datastore, namespace and login), or "".
func sameServerIn(cfg *Config, s *PBSServer) string {
	norm := func(v string) string { return strings.ToLower(strings.TrimRight(strings.TrimSpace(v), "/")) }
	for id, c := range cfg.PBSServers {
		if c != nil && norm(c.BaseURL) == norm(s.BaseURL) && c.Datastore == s.Datastore &&
			c.Namespace == s.Namespace && c.AuthID == s.AuthID && c.Username == s.Username {
			return id
		}
	}
	return ""
}

var nonIDChars = regexp.MustCompile(`[^a-z0-9]+`)

// ownerSlug is the signed-in user's name, for IDs and names that clash.
func ownerSlug() string {
	name := "user"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
		if i := strings.LastIndexAny(name, `\/`); i >= 0 {
			name = name[i+1:]
		}
	}
	s := strings.Trim(nonIDChars.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if s == "" {
		return "user"
	}
	return s
}
