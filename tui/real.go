package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/salvanza/salvanza_client/gui/api"
)

// serviceBackend is the real backend: the local service API.
type serviceBackend struct {
	c    *api.Client
	name string
}

func newServiceBackend(url, tokenPath string) *serviceBackend {
	var c *api.Client
	if url != "" && url != api.DefaultServiceURL {
		c = api.NewClientAt(url, tokenPath)
	} else {
		c = api.NewClient(tokenPath)
	}
	return &serviceBackend{c: c, name: "service"}
}

func (b *serviceBackend) Name() string { return b.name }

func (b *serviceBackend) Status() (*api.StatusResponse, error) { return b.c.GetStatus() }

func (b *serviceBackend) Sets() ([]Set, error) {
	raw, err := b.c.GetScheduledJobsFull()
	if err != nil {
		return nil, err
	}
	out := make([]Set, 0, len(raw))
	for _, m := range raw {
		out = append(out, toSet(m))
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (b *serviceBackend) Runs() ([]api.BackupProgress, error) {
	r, err := b.c.GetRuns()
	if err != nil {
		return nil, err
	}
	return r.Runs, nil
}

func (b *serviceBackend) RunSet(id string) error { return b.c.RunJob(id) }

func (b *serviceBackend) Control(action, jobID string) error {
	_, err := b.c.ControlRun(action, jobID)
	return err
}

func (b *serviceBackend) SetEnabled(id string, on bool) error { return b.c.SetJobEnabled(id, on) }

func (b *serviceBackend) History() ([]HistoryEntry, error) {
	raw, err := b.c.GetJobHistory()
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, 0, len(raw))
	for _, m := range raw {
		out = append(out, toHistory(m))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}

func (b *serviceBackend) Messages() ([]MessageEntry, error) {
	raw, err := b.c.GetMessageLog()
	if err != nil {
		return nil, err
	}
	out := make([]MessageEntry, 0, len(raw))
	for _, m := range raw {
		out = append(out, toMessage(m))
	}
	return out, nil
}

func (b *serviceBackend) Servers() ([]Server, error) {
	doc, err := b.c.GetFullConfig()
	if err != nil {
		return nil, err
	}
	def, _ := doc["default_pbs_id"].(string)
	var out []Server
	if m, ok := doc["pbs_servers"].(map[string]interface{}); ok {
		for id, v := range m {
			s, _ := v.(map[string]interface{})
			if s == nil {
				continue
			}
			login := "token"
			if str(s, "username") != "" {
				login = "password"
			}
			out = append(out, Server{
				ID: id, Name: firstNonEmpty(str(s, "name"), id), URL: str(s, "baseurl"), Datastore: str(s, "datastore"),
				Namespace: str(s, "namespace"), Fingerprint: str(s, "certfingerprint"), Default: id == def,
				Login: login, Encrypted: str(s, "encryption_key_file") != "",
			})
		}
	}
	if len(out) == 0 && str(doc, "baseurl") != "" {
		out = append(out, Server{
			ID: "default", Name: "Default", URL: str(doc, "baseurl"), Datastore: str(doc, "datastore"),
			Namespace: str(doc, "namespace"), Fingerprint: str(doc, "certfingerprint"), Default: true,
			Login: "token", Encrypted: str(doc, "encryption_key_file") != "",
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

// describeErr turns a connection problem into something a person can act on.
func describeErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"), strings.Contains(strings.ToLower(msg), "unauthorized"):
		return "the service refused the token: run as administrator or root, or set SALVANZA_API_TOKEN"
	case strings.Contains(msg, "refused"), strings.Contains(msg, "connect"):
		return "the Salvanza service is not running or not reachable"
	case strings.Contains(msg, "501"):
		return "this service is older and does not offer that"
	}
	return fmt.Sprint(msg)
}
