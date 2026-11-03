package pbscommon

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PBS refuses a backup session whose backup time is not later than the
// group's newest snapshot with HTTP 400 and "backup timestamp is older than
// last backup" on the upgrade. That surfaced as "PBS authentication failed";
// it is a BackupTimeTakenErr now, which the tools wait out.
func TestUpgradeRefusedBackupTimeTaken(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "backup timestamp is older than last backup.", http.StatusBadRequest)
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
	_, err := pbs.DownloadPreviousToBytes("backup.pxar.didx")
	if err == nil {
		t.Fatal("no error for a refused session")
	}
	var bt *BackupTimeTakenErr
	if !errors.As(err, &bt) || !IsBackupTimeTaken(err) {
		t.Fatalf("err %v: not a BackupTimeTakenErr", err)
	}
	if bt.BackupTime != pbs.Manifest.BackupTime || strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("err %v (backup time %d, want %d)", err, bt.BackupTime, pbs.Manifest.BackupTime)
	}
	// Recognised as text too, once wrapped by %v somewhere along the way.
	if !IsBackupTimeTaken(fmt.Errorf("backup failed: %v", err)) {
		t.Error("the wrapped text is not recognised")
	}
	if IsBackupTimeTaken(errors.New("HTTP 400 - something else")) || IsBackupTimeTaken(nil) {
		t.Error("other errors taken for a backup time clash")
	}
}

// Only a 401 says "authentication failed"; any other refusal of the session
// says it was refused.
func TestAuthErrWording(t *testing.T) {
	if s := (&AuthErr{StatusCode: "401 Unauthorized"}).Error(); !strings.HasPrefix(s, "PBS authentication failed: HTTP 401") {
		t.Errorf("401: %q", s)
	}
	for _, code := range []string{"400 Bad Request", "403 Forbidden", "500 Internal Server Error"} {
		if s := (&AuthErr{StatusCode: code}).Error(); !strings.HasPrefix(s, "PBS refused the backup session: HTTP "+code) {
			t.Errorf("%s: %q", code, s)
		}
	}
}
