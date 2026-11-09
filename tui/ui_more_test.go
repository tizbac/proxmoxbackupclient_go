package tui

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestReportsScreen(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('7')
	h.waitFor("Reports: all recorded runs")
	s := h.screen()
	for _, want := range []string{"Backup Set", "Success", "Photos", "Documents", "All sets", "Recent failures", "no route to host"} {
		if !strings.Contains(s, want) {
			t.Errorf("reports lack %q:\n%s", want, s)
		}
	}
	h.typ('w')
	h.waitFor("the last 7 days")
	h.typ('w')
	h.waitFor("the last 30 days")
	h.typ('w')
	h.waitFor("all recorded runs")
}

func TestReportStatsGroupsByBackupSetAndPeriod(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	hist := []HistoryEntry{
		{Name: "A", Status: "success", At: now.Add(-1 * time.Hour)},
		{Name: "A", Status: "failed", At: now.Add(-2 * time.Hour), Message: "boom"},
		{Name: "A", Status: "success", At: now.AddDate(0, 0, -20)},
		{Name: "B", Status: "success", At: now.AddDate(0, 0, -40)},
	}
	all, fails := reportStats(hist, 0, now)
	if len(all) != 2 || all[0].Name != "A" || all[0].Runs != 3 || all[0].OK != 2 || all[0].Fail != 1 || len(fails) != 1 {
		t.Fatalf("all: %+v %+v", all, fails)
	}
	week, _ := reportStats(hist, 7, now)
	if len(week) != 1 || week[0].Runs != 2 {
		t.Errorf("7 days: %+v", week)
	}
	month, _ := reportStats(hist, 30, now)
	if len(month) != 1 || month[0].Runs != 3 {
		t.Errorf("30 days: %+v", month)
	}
}

func TestLogFilesBrowserAndFilter(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('8')
	h.waitFor("Log files")
	h.waitFor("service.log")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Log file: ")
	h.waitFor("New chunk")
	h.typ('/')
	h.waitFor("Filter:")
	h.typeText("warning")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("filter: warning")
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(h.screen(), "New chunk") {
		t.Errorf("the filter leaves out the other lines:\n%s", h.screen())
	}
	if !strings.Contains(h.screen(), "2 files skipped") {
		t.Errorf("the filter keeps the matching line:\n%s", h.screen())
	}
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Log files")
}

func TestServerManagement(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.waitFor("Offsite PBS")

	// test connection: the demo's offsite server fails, the first works
	h.typ('t')
	h.waitFor("connection to Main PBS works")
	h.key(tcell.KeyDown, 0)
	h.typ('t')
	h.waitFor("failed: connection test failed")

	// make the second the default
	h.typ('u')
	h.waitFor("Offsite PBS is the default server now")
	h.waitFor("(default)")

	// add a server
	h.typ('n')
	h.waitFor("New server")
	h.typeText("Lab PBS")
	h.key(tcell.KeyTab, 0)
	h.typeText("https://lab.example.com:8007")
	h.key(tcell.KeyTab, 0) // fingerprint
	h.key(tcell.KeyTab, 0) // datastore
	h.typeText("labstore")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Token ID")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("give the token ID")
	h.key(tcell.KeyEnter, 0)
	h.key(tcell.KeyTab, 0)
	h.typeText("lab@pbs!tui")
	h.key(tcell.KeyTab, 0)
	h.typeText("s3cret")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Lab PBS")
	h.waitFor("Lab PBS")

	// the secret never shows on screen
	if strings.Contains(h.screen(), "s3cret") {
		t.Errorf("a typed secret was shown:\n%s", h.screen())
	}

	// delete the new one
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyDown, 0)
	h.typ('d')
	h.waitFor("Delete the server")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("deleted ")
}

func TestServerEditorRefusesWhatIsMissing(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.typ('n')
	h.waitFor("New server")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("needs a name")
	h.key(tcell.KeyEnter, 0)
	h.typeText("X")
	h.key(tcell.KeyTab, 0)
	h.typeText("http://insecure.example.com")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("https://")
}

func TestPreferencesScreen(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Backup options")
	h.waitFor("Upload limit")
	// set the upload limit
	h.typeText("25")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("preferences saved")
	if p, err := h.a.be.Prefs(); err != nil || p.UploadMBps != 25 {
		t.Errorf("the upload limit was not stored: %+v %v", p, err)
	}

	// a refused value is explained
	for i := 0; i < 4; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.typeText("5") // chunk timeout 5 is under the 30 second minimum
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("between 30 and 1800")
	h.key(tcell.KeyEnter, 0)

	// e-mail
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("SMTP server")
	h.waitFor("smtp.example.com")
	h.key(tcell.KeyPgUp, 0)
	h.waitFor("Upload limit")
}

func TestSendTestMailNeedsAnAddress(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Test message to")
	// to the test message field, then its button
	for i := 0; i < 6; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.typeText("someone@example.com")
	// buttons: E-mail screen has Backup options, Save, Save and send test message
	for i := 0; i < 3; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("test message sent to someone@example.com")
}

func TestEncryptedBackupAsksForThePassphrase(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("host/DESKTOP-01-photos")
	h.key(tcell.KeyDown, 0) // the photos group, whose snapshots are encrypted
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Encrypted")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Enter the passphrase")
	// a wrong passphrase is explained and asked for again
	h.typeText("wrongpass")
	if strings.Contains(h.screen(), "wrongpass") {
		t.Errorf("the passphrase was shown:\n%s", h.screen())
	}
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("did not unlock the key")
	h.typeText(demoPassphrase)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Selection")
	h.waitFor("Documents")
	// the restore goes through with the passphrase given once
	h.typ('a')
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Restore into")
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Restore finished")
}

func TestCancellingThePassphraseGoesBackToTheSnapshots(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("host/DESKTOP-01-photos")
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Encrypted")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Enter the passphrase")
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Snapshots")
}

func TestFetchFingerprintOfATLSServer(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	fp, err := fetchFingerprint("https://" + u.Host)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(srv.Certificate().Raw)
	want := make([]string, len(sum))
	for i, b := range sum {
		want[i] = fmt.Sprintf("%02X", b)
	}
	if fp != strings.Join(want, ":") {
		t.Errorf("fingerprint %s, want %s", fp, strings.Join(want, ":"))
	}
	if _, err := fetchFingerprint("not a url"); err == nil {
		t.Error("a bad address is refused")
	}
}

func TestDescribeErrSeparatesTheServiceFromTheServer(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"failed to connect to service: Get \"http://127.0.0.1:18765/status\": dial tcp 127.0.0.1:18765: connectex: refused", "service is not running"},
		{"service returned error: 401", "refused the token"},
		{"service returned error: 501", "older"},
		{"connection test failed: no route to host", "connection test failed: no route to host"},
		{"dial tcp 192.168.0.242:8007: i/o timeout", "dial tcp 192.168.0.242:8007: i/o timeout"},
	} {
		if got := describeErr(fmt.Errorf("%s", c.in)); !strings.Contains(got, c.want) {
			t.Errorf("describeErr(%q) = %q, want it to contain %q", c.in, got, c.want)
		}
	}
}

func TestServerSlug(t *testing.T) {
	for in, want := range map[string]string{"Lab PBS": "lab-pbs", "  Main_Server.2 ": "main-server-2", "Ünï": "n"} {
		if got := serverSlug(in); got != want {
			t.Errorf("serverSlug(%q) = %q, want %q", in, got, want)
		}
	}
}
