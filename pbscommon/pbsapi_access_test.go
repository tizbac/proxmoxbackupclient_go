package pbscommon

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeAccessPBS answers the datastore status and the permissions calls the way
// the table says, and records what it was asked.
type fakeAccessPBS struct {
	statusCode int
	permCode   int
	permBody   string

	mu        sync.Mutex
	permPaths []string
	auths     []string
	statusHit int
}

func (f *fakeAccessPBS) start(t *testing.T) *PBSClient {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			f.statusHit++
			if f.statusCode != 0 && f.statusCode != http.StatusOK {
				http.Error(w, "permission check failed", f.statusCode)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"avail":1,"total":2,"used":1}}`))
		case r.URL.Path == "/api2/json/access/permissions":
			f.permPaths = append(f.permPaths, r.URL.Query().Get("path"))
			f.auths = append(f.auths, r.Header.Get("Authorization"))
			if f.permCode != 0 && f.permCode != http.StatusOK {
				http.Error(w, "nope", f.permCode)
				return
			}
			_, _ = w.Write([]byte(f.permBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(srv.Certificate().Raw)
	return &PBSClient{
		BaseURL: srv.URL, CertFingerPrint: hex.EncodeToString(sum[:]), Insecure: true,
		Datastore: "store", AuthID: "user@pbs!tok", Secret: "s3cret",
	}
}

func perms(path string, privs string) string {
	return `{"data":{"` + path + `":{` + privs + `}}}`
}

func TestTestConnectionAccessOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		permCode  int
		permBody  string
		wantLevel AccessLevel
		wantNil   bool   // the access is not known
		wantErr   string // part of the error, "" for none
		wantNote  string // part of the note, "" for none
	}{
		{name: "back up and restore",
			permBody:  perms("/datastore/store", `"Datastore.Audit":true,"Datastore.Backup":true,"Datastore.Read":true,"Datastore.Prune":true`),
			wantLevel: AccessFull},
		{name: "back up only (the PowerUser role has no Read)",
			permBody:  perms("/datastore/store", `"Datastore.Audit":true,"Datastore.Backup":true,"Datastore.Prune":true`),
			wantLevel: AccessFull},
		{name: "restore only",
			permBody:  perms("/datastore/store", `"Datastore.Audit":true,"Datastore.Read":true,"Datastore.Verify":true`),
			wantLevel: AccessRestoreOnly, wantNote: "restore only: this account has Datastore.Read but not Datastore.Backup on /datastore/store"},
		{name: "can only look",
			permBody:  perms("/datastore/store", `"Datastore.Audit":true`),
			wantLevel: AccessDenied, wantErr: "access denied: this account may not back up or restore on /datastore/store"},
		{name: "false values do not count",
			permBody:  perms("/datastore/store", `"Datastore.Backup":false,"Datastore.Read":true`),
			wantLevel: AccessRestoreOnly, wantNote: "restore only"},
		{name: "numbers instead of booleans",
			permBody:  perms("/datastore/store", `"Datastore.Backup":1,"Datastore.Read":0`),
			wantLevel: AccessFull},
		{name: "only false values",
			permBody:  perms("/datastore/store", `"Datastore.Backup":false,"Datastore.Read":false`),
			wantLevel: AccessDenied, wantErr: "it has no permission there"},
		{name: "no permission on the path (rights on a namespace only)",
			permBody:  `{"data":{}}`,
			wantLevel: AccessDenied, wantErr: "may not back up or restore on /datastore/store (backing up needs the Datastore.Backup permission, restoring Datastore.Read; it has no permission there)"},
		{name: "no data at all is not understood", permBody: `{"data":null}`, wantNil: true},
		{name: "an answer without data is not understood", permBody: `{}`, wantNil: true},
		{name: "an older PBS without the call", permCode: http.StatusNotFound, wantNil: true},
		{name: "the call refused", permCode: http.StatusForbidden, wantNil: true},
		{name: "an answer that is not JSON", permBody: "<html>nope</html>", wantNil: true},
		{name: "a surprising shape", permBody: `{"data":[1,2,3]}`, wantNil: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAccessPBS{permCode: tc.permCode, permBody: tc.permBody}
			pbs := f.start(t)

			acc, err := pbs.TestConnectionAccess()
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error %v, want one containing %q", err, tc.wantErr)
			}
			if tc.wantNil {
				if acc != nil {
					t.Fatalf("access %+v, want unknown (nil)", acc)
				}
				return
			}
			if acc == nil || acc.Level != tc.wantLevel {
				t.Fatalf("access %+v, want level %d", acc, tc.wantLevel)
			}
			note := acc.Note()
			if tc.wantNote == "" && note != "" {
				t.Errorf("unexpected note %q", note)
			}
			if tc.wantNote != "" && !strings.Contains(note, tc.wantNote) {
				t.Errorf("note %q, want one containing %q", note, tc.wantNote)
			}
			// TestConnection reports the same failure and ignores the warning.
			if got := pbs.TestConnection(); (got == nil) != (err == nil) {
				t.Errorf("TestConnection %v, TestConnectionAccess %v", got, err)
			}
		})
	}
}

func TestDeniedErrorNamesWhatTheAccountHas(t *testing.T) {
	f := &fakeAccessPBS{permBody: perms("/datastore/store", `"Datastore.Audit":true,"Datastore.Verify":true`)}
	_, err := f.start(t).TestConnectionAccess()
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"Datastore.Audit, Datastore.Verify", "Datastore.Backup", "Datastore.Read"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

func TestAccessAsksAboutTheNamespace(t *testing.T) {
	f := &fakeAccessPBS{permBody: perms("/datastore/store/team/a", `"Datastore.Backup":true`)}
	pbs := f.start(t)
	pbs.Namespace = "team/a"
	if _, err := pbs.TestConnectionAccess(); err != nil {
		t.Fatal(err)
	}
	if len(f.permPaths) != 1 || f.permPaths[0] != "/datastore/store/team/a" {
		t.Errorf("asked about %v, want /datastore/store/team/a", f.permPaths)
	}
	pbs.Namespace = "/other/"
	_, _ = pbs.TestConnectionAccess()
	if got := f.permPaths[len(f.permPaths)-1]; got != "/datastore/store/other" {
		t.Errorf("a namespace written with slashes: asked about %q", got)
	}
	pbs.Namespace = ""
	_, _ = pbs.TestConnectionAccess()
	if got := f.permPaths[len(f.permPaths)-1]; got != "/datastore/store" {
		t.Errorf("no namespace: asked about %q", got)
	}
}

func TestAccessSendsTheAccountsCredentials(t *testing.T) {
	f := &fakeAccessPBS{permBody: perms("/datastore/store", `"Datastore.Backup":true`)}
	pbs := f.start(t)
	if _, err := pbs.TestConnectionAccess(); err != nil {
		t.Fatal(err)
	}
	if len(f.auths) != 1 || f.auths[0] != "PBSAPIToken=user@pbs!tok:s3cret" {
		t.Errorf("credentials sent: %v", f.auths)
	}
}

// The permissions are only asked after the status was read: a wrong login or
// an unreachable server is reported as before.
func TestAccessIsNotAskedWhenTheStatusFails(t *testing.T) {
	for _, tc := range []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "authentication failed"},
		{http.StatusForbidden, "access denied: this account has no permission on /datastore/store"},
		{http.StatusInternalServerError, "server error: HTTP 500"},
	} {
		f := &fakeAccessPBS{statusCode: tc.code, permBody: perms("/datastore/store", `"Datastore.Backup":true`)}
		acc, err := f.start(t).TestConnectionAccess()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("status %d: error %v, want one containing %q", tc.code, err, tc.want)
		}
		if acc != nil || len(f.permPaths) != 0 {
			t.Errorf("status %d: the permissions were asked (%v) or access returned (%+v)", tc.code, f.permPaths, acc)
		}
	}
}

func TestAccessUnreachableServerIsAConnectionError(t *testing.T) {
	pbs := &PBSClient{BaseURL: "https://127.0.0.1:1", Datastore: "store", AuthID: "a@pbs!t", Secret: "x", Insecure: true}
	if err := pbs.TestConnection(); err == nil || !strings.Contains(err.Error(), "connection failed") {
		t.Errorf("error %v", err)
	}
}
