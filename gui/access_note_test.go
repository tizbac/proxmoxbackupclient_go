//go:build !service

package main

// The connection test's warning for an account that can restore but not back
// up, from the PBS permissions to the app's window: the service runs the
// test, and an app working through an older service falls back to the plain
// test.

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakePBS is a PBS that reads the datastore status and answers the permissions
// call with what the test sets.
type fakePBS struct {
	url         string
	fingerprint string

	mu     sync.Mutex
	status int
	perms  string
	asked  []string
}

func newFakePBS(t *testing.T) *fakePBS {
	t.Helper()
	f := &fakePBS{status: http.StatusOK}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.asked = append(f.asked, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			if f.status != http.StatusOK {
				http.Error(w, "nope", f.status)
				return
			}
			_, _ = w.Write([]byte(`{"data":{}}`))
		case r.URL.Path == "/api2/json/access/permissions":
			_, _ = w.Write([]byte(f.perms))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(srv.Certificate().Raw)
	var parts []string
	for _, b := range sum {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	f.url, f.fingerprint = srv.URL, strings.Join(parts, ":")
	return f
}

func (f *fakePBS) setPerms(privs string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.perms = `{"data":{"/datastore/store1":{` + privs + `}}}`
}

func (f *fakePBS) server(id string) *PBSServer {
	s := strictTokenServer(id)
	s.BaseURL, s.CertFingerprint = f.url, f.fingerprint
	return s
}

const (
	privsFull    = `"Datastore.Audit":true,"Datastore.Backup":true,"Datastore.Read":true`
	privsRestore = `"Datastore.Audit":true,"Datastore.Read":true`
	privsLook    = `"Datastore.Audit":true`
)

// In the service (and an app on its own settings) the test returns the note.
func TestTestPBSConnectionNoteInTheService(t *testing.T) {
	f := newFakePBS(t)
	svc := strictServiceApp(t)
	svc.config.PBSServers["pbs1"] = f.server("pbs1")

	for _, tc := range []struct {
		privs    string
		wantNote string
		wantErr  string
	}{
		{privs: privsFull},
		{privs: privsRestore, wantNote: "restore only: this account has Datastore.Read but not Datastore.Backup on /datastore/store1"},
		{privs: privsLook, wantErr: "may not back up or restore on /datastore/store1"},
	} {
		f.setPerms(tc.privs)
		note, err := svc.TestPBSConnectionNote("pbs1")
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: error %v, want one containing %q", tc.privs, err, tc.wantErr)
			}
		} else if err != nil {
			t.Errorf("%s: unexpected error %v", tc.privs, err)
		}
		if tc.wantNote == "" && note != "" || tc.wantNote != "" && !strings.Contains(note, tc.wantNote) {
			t.Errorf("%s: note %q, want one containing %q", tc.privs, note, tc.wantNote)
		}
		// The plain test fails exactly when the noted one does, and never warns.
		if err2 := svc.TestPBSConnection("pbs1"); (err2 == nil) != (err == nil) {
			t.Errorf("%s: TestPBSConnection %v, the noted test %v", tc.privs, err2, err)
		}
	}
	if _, err := svc.TestPBSConnectionNote("missing"); err == nil {
		t.Error("an unknown server tested")
	}
}

// The window's config form tests the settings it holds, without saving them.
func TestTestConnectionNoteWithAFormConfig(t *testing.T) {
	f := newFakePBS(t)
	f.setPerms(privsRestore)
	svc := strictServiceApp(t)
	note, err := svc.TestConnectionNote(&Config{BaseURL: f.url, CertFingerprint: f.fingerprint, AuthID: strictAuthID, Secret: strictSecret, Datastore: "store1"})
	if err != nil || !strings.Contains(note, "restore only") {
		t.Errorf("note %q, err %v", note, err)
	}
	if _, err := svc.TestConnectionNote(&Config{BaseURL: f.url, Datastore: "store1"}); err == nil {
		t.Error("a config without a login tested")
	}
}

// A refused status is reported as before, and the permissions are not asked.
func TestTestConnectionNoteStatusRefused(t *testing.T) {
	f := newFakePBS(t)
	f.status = http.StatusForbidden
	f.setPerms(privsFull)
	svc := strictServiceApp(t)
	svc.config.PBSServers["pbs1"] = f.server("pbs1")
	note, err := svc.TestPBSConnectionNote("pbs1")
	if err == nil || !strings.Contains(err.Error(), "has no permission on /datastore/store1") || note != "" {
		t.Errorf("note %q, err %v", note, err)
	}
	for _, p := range f.asked {
		if strings.Contains(p, "permissions") {
			t.Errorf("the permissions were asked after the status was refused: %v", f.asked)
		}
	}
}

// The app works through the service over HTTP: the note comes back.
func TestStrictAppConnectionNoteThroughService(t *testing.T) {
	f := newFakePBS(t)
	f.setPerms(privsRestore)
	p := newStrictPair(t)
	p.service(func(s *App) { s.config.PBSServers["pbs1"] = f.server("pbs1") })

	note, err := p.app.TestPBSConnectionNote("pbs1")
	if err != nil || !strings.Contains(note, "restore only") {
		t.Fatalf("note %q, err %v", note, err)
	}
	f.setPerms(privsFull)
	if note, err = p.app.TestPBSConnectionNote("pbs1"); err != nil || note != "" {
		t.Errorf("full access: note %q, err %v", note, err)
	}
	f.setPerms(privsLook)
	if _, err = p.app.TestPBSConnectionNote("pbs1"); err == nil || !strings.Contains(err.Error(), "may not back up or restore") {
		t.Errorf("look only: err %v", err)
	}
	// The form's config goes through too.
	f.setPerms(privsRestore)
	note, err = p.app.TestConnectionNote(&Config{BaseURL: f.url, CertFingerprint: f.fingerprint, AuthID: strictAuthID, Secret: strictSecret, Datastore: "store1"})
	if err != nil || !strings.Contains(note, "restore only") {
		t.Errorf("a form config: note %q, err %v", note, err)
	}
	p.saw(t, "POST /call/TestPBSConnectionNote", "POST /call/TestConnectionNote")
}

// A service from before the warning does not know the new calls: the app
// falls back to the plain test, and a failure still reaches the window.
func TestStrictAppConnectionNoteAgainstAnOlderService(t *testing.T) {
	f := newFakePBS(t)
	f.setPerms(privsRestore)
	p := newStrictPair(t)
	p.service(func(s *App) { s.config.PBSServers["pbs1"] = f.server("pbs1") })

	for _, name := range []string{"TestPBSConnectionNote", "TestConnectionNote"} {
		saved := apiCalls[name]
		delete(apiCalls, name)
		t.Cleanup(func() { apiCalls[name] = saved })
	}

	note, err := p.app.TestPBSConnectionNote("pbs1")
	if err != nil || note != "" {
		t.Errorf("an older service: note %q, err %v (it can only say the test passed)", note, err)
	}
	p.saw(t, "POST /call/TestPBSConnection")

	f.setPerms(privsLook)
	if _, err = p.app.TestPBSConnectionNote("pbs1"); err == nil || !strings.Contains(err.Error(), "may not back up or restore") {
		t.Errorf("an older service, look only: err %v", err)
	}
}
