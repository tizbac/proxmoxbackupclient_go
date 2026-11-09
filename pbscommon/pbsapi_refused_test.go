package pbscommon

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An account that may read a datastore but not write to it passes the
// connection test, and PBS refuses its backup session with a 403 that says what
// is missing. The HTTP/2 transport dials again for the requests after the
// failed one; they used to report "connection lost: PBS session cannot be
// resumed", which a backup takes for a dropped connection and retries.
func TestRefusedSessionKeepsReportingWhy(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing permissions 'Datastore.Backup' on '/datastore/store'", http.StatusForbidden)
	}))
	defer srv.Close()
	sum := sha256.Sum256(srv.Certificate().Raw)

	pbs := &PBSClient{
		BaseURL:         srv.URL,
		CertFingerPrint: hex.EncodeToString(sum[:]),
		Insecure:        true,
		Datastore:       "store",
		Manifest:        BackupManifest{BackupID: "test"},
	}
	pbs.Connect(false, "host")
	defer pbs.Close()
	for i := 1; i <= 3; i++ {
		_, err := pbs.DownloadPreviousToBytes("backup.pxar.didx")
		if err == nil {
			t.Fatalf("request %d: no error for a refused session", i)
		}
		for _, want := range []string{"missing permissions 'Datastore.Backup'", "HTTP 403", "Datastore.Backup permission"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("request %d: error lacks %q: %v", i, want, err)
			}
		}
		if strings.Contains(err.Error(), "cannot be resumed") || strings.Contains(err.Error(), "connection lost") {
			t.Errorf("request %d: a refused session is reported as a lost connection: %v", i, err)
		}
	}
}

// PBS's words are shown without the response headers around them.
func TestAuthErrShowsPBSsWordsOnly(t *testing.T) {
	e := &AuthErr{
		StatusCode:   "403 Forbidden",
		ResponseBody: "HTTP/1.1 403 Forbidden\r\nContent-Length: 60\r\nKeep-Alive: timeout=5\r\n\r\n\nBody: missing permissions 'Datastore.Backup' on '/datastore/s'\n",
	}
	got := e.Error()
	if !strings.Contains(got, "missing permissions 'Datastore.Backup' on '/datastore/s'") {
		t.Errorf("PBS's words are missing: %q", got)
	}
	for _, bad := range []string{"Content-Length", "Keep-Alive", "timeout"} {
		if strings.Contains(got, bad) {
			t.Errorf("response header text %q leaked into the message: %q", bad, got)
		}
	}
	if m := (&AuthErr{StatusCode: "401 Unauthorized", ResponseBody: "HTTP/1.1 401 Unauthorized\r\n\r\n"}).Message(); m != "" {
		t.Errorf("headers only should give no message: %q", m)
	}
}
