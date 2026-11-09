package pbscommon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AccessLevel is what an account may do on a datastore, from the permissions
// PBS reports for it.
type AccessLevel int

const (
	// AccessUnknown: PBS did not say (an older PBS, an answer that cannot be
	// read). Nothing is claimed in that case; TestConnectionAccess gives no
	// access at all then.
	AccessUnknown AccessLevel = iota
	// AccessFull: the account may back up (Datastore.Backup). It also restores
	// what it backed up; Datastore.Read adds the backups of others.
	AccessFull
	// AccessRestoreOnly: Datastore.Read without Datastore.Backup. Backups to
	// the datastore are refused.
	AccessRestoreOnly
	// AccessDenied: the account has neither Datastore.Backup nor
	// Datastore.Read on the path, whatever else it has (or nothing, as an
	// account whose permissions are on a namespace only has on the datastore).
	AccessDenied
)

// DatastoreAccess is the permissions of the account on one datastore path.
type DatastoreAccess struct {
	Path       string
	Privileges []string // sorted, those PBS reports as held
	Level      AccessLevel
}

// accessLevel works out what the privileges allow.
func accessLevel(privs map[string]bool) AccessLevel {
	switch {
	case privs["Datastore.Backup"]:
		return AccessFull
	case privs["Datastore.Read"]:
		return AccessRestoreOnly
	}
	return AccessDenied
}

// accessPath is the ACL path of the datastore, or of its namespace.
func (pbs *PBSClient) accessPath() string {
	p := "/datastore/" + pbs.Datastore
	if ns := strings.Trim(pbs.Namespace, "/"); ns != "" {
		p += "/" + ns
	}
	return p
}

// Access asks PBS what the account may do on the datastore (and namespace) it
// is set up for: GET /access/permissions?path=... A PBS that does not have the
// call, or an answer that cannot be read, gives an error; callers treat that
// as "not known", not as a refusal.
func (pbs *PBSClient) Access() (*DatastoreAccess, error) {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: pbs.buildTLSConfig()},
	}
	return pbs.accessWith(client)
}

func (pbs *PBSClient) accessWith(client *http.Client) (*DatastoreAccess, error) {
	path := pbs.accessPath()
	req, err := http.NewRequest(http.MethodGet, pbs.BaseURL+"/api2/json/access/permissions?path="+url.QueryEscape(path), nil)
	if err != nil {
		return nil, err
	}
	pbs.setAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("permissions: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Data map[string]map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("permissions: %w", err)
	}
	if out.Data == nil {
		return nil, fmt.Errorf("permissions: no data in the answer")
	}
	// An empty set is an answer too: an account whose permissions are on a
	// namespace only can read the datastore's status and has none on the
	// datastore itself.
	held := map[string]bool{}
	for _, v := range out.Data {
		for name, on := range v {
			switch t := on.(type) {
			case bool:
				held[name] = held[name] || t
			case float64:
				held[name] = held[name] || t != 0
			}
		}
	}
	acc := &DatastoreAccess{Path: path}
	for name, on := range held {
		if on {
			acc.Privileges = append(acc.Privileges, name)
		} else {
			delete(held, name)
		}
	}
	sort.Strings(acc.Privileges)
	acc.Level = accessLevel(held)
	return acc, nil
}

// Note is the warning to show next to a successful connection test, or "" when
// there is nothing to warn about.
func (a *DatastoreAccess) Note() string {
	if a != nil && a.Level == AccessRestoreOnly {
		return fmt.Sprintf("restore only: this account has Datastore.Read but not Datastore.Backup on %s, so backups to it are refused", a.Path)
	}
	return ""
}

// DeniedError is the failure to show when the account may neither back up nor
// restore.
func (a *DatastoreAccess) DeniedError() error {
	has := "no permission there"
	if len(a.Privileges) > 0 {
		has = strings.Join(a.Privileges, ", ")
	}
	return fmt.Errorf("access denied: this account may not back up or restore on %s (backing up needs the Datastore.Backup permission, restoring Datastore.Read; it has %s)", a.Path, has)
}

// TestConnectionAccess is TestConnection that also says what the account may do:
// the error is as TestConnection's, plus access denied when the account can
// neither back up nor restore; the access is nil when PBS did not say, and
// Note() of it is the warning for a restore-only account.
func (pbs *PBSClient) TestConnectionAccess() (*DatastoreAccess, error) {
	if err := pbs.testStatus(); err != nil {
		return nil, err
	}
	acc, err := pbs.Access()
	if err != nil {
		return nil, nil
	}
	if acc.Level == AccessDenied {
		return acc, acc.DeniedError()
	}
	return acc, nil
}
