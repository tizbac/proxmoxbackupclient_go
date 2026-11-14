package pbscommon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeTicketPBS is a PBS that takes only the ticket "fresh" on its notes API
// (the one the backup started with has expired), and hands out "fresh" on a
// password login.
type fakeTicketPBS struct {
	mu      sync.Mutex
	logins  int
	notes   int
	refused int
	stored  string
}

func newFakeTicketPBS(t *testing.T) (*fakeTicketPBS, *httptest.Server) {
	t.Helper()
	f := &fakeTicketPBS{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			f.logins++
			_ = r.ParseForm()
			if r.Form.Get("password") != "pw" {
				http.Error(w, `{"data":null}`, http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"ticket":"fresh","CSRFPreventionToken":"csrf2","username":"u@pbs"}}`))
		case "/api2/json/admin/datastore/store/notes":
			f.notes++
			if r.Header.Get("Cookie") != "PBSAuthCookie=fresh" && r.Header.Get("Authorization") != "PBSAPIToken=tok@pbs!t:good" {
				f.refused++
				http.Error(w, "authentication failed - invalid credentials", http.StatusUnauthorized)
				return
			}
			if r.Method == http.MethodPut {
				_ = r.ParseForm()
				f.stored = r.Form.Get("notes")
				return
			}
			_, _ = w.Write([]byte(`{"data":"` + f.stored + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func ticketClient(url string) *PBSClient {
	return &PBSClient{
		BaseURL:   url,
		Datastore: "store",
		Ticket:    "expired",
		CSRFToken: "csrf1",
		Manifest:  BackupManifest{BackupType: "host", BackupID: "pc", BackupTime: 1794000000},
	}
}

// A backup that outlived its ticket still gets its comment: the CLI's client
// holds the login and gets a fresh ticket with it.
func TestSetCommentRenewsAnExpiredTicketWithTheLogin(t *testing.T) {
	f, srv := newFakeTicketPBS(t)
	c := ticketClient(srv.URL)
	c.Username, c.Password = "u@pbs", "pw"
	if err := c.SetComment("nightly"); err != nil {
		t.Fatalf("SetComment: %v", err)
	}
	if f.stored != "nightly" || f.logins != 1 || f.notes != 2 || f.refused != 1 {
		t.Fatalf("stored %q, logins %d, notes requests %d, refused %d; want the comment after one login and one retry", f.stored, f.logins, f.notes, f.refused)
	}
	if c.Ticket != "fresh" || c.CSRFToken != "csrf2" {
		t.Errorf("the client kept ticket %q / %q", c.Ticket, c.CSRFToken)
	}
	got, err := c.GetComment()
	if err != nil || got != "nightly" {
		t.Fatalf("GetComment = %q, %v", got, err)
	}
}

// The app hands the backup only a ticket, and a way to get a new one.
func TestSetCommentRenewsThroughTheHook(t *testing.T) {
	f, srv := newFakeTicketPBS(t)
	c := ticketClient(srv.URL)
	calls := 0
	c.RenewTicket = func() (string, string, error) { calls++; return "fresh", "csrf2", nil }
	if err := c.SetComment("from the app"); err != nil {
		t.Fatalf("SetComment: %v", err)
	}
	if calls != 1 || f.logins != 0 || f.stored != "from the app" {
		t.Fatalf("hook calls %d, password logins %d, stored %q", calls, f.logins, f.stored)
	}
}

// A ticket that still works is used as it is: no login, one request.
func TestSetCommentWithAValidTicketDoesNotLogIn(t *testing.T) {
	f, srv := newFakeTicketPBS(t)
	c := ticketClient(srv.URL)
	c.Ticket = "fresh"
	c.RenewTicket = func() (string, string, error) { t.Error("renewed a ticket that worked"); return "", "", nil }
	if err := c.SetComment("x"); err != nil {
		t.Fatal(err)
	}
	if f.notes != 1 || f.logins != 0 {
		t.Fatalf("notes requests %d, logins %d", f.notes, f.logins)
	}
}

// An API token does not expire: its 401 is the answer, with no login.
func TestSetCommentDoesNotRetryAnAPIToken(t *testing.T) {
	f, srv := newFakeTicketPBS(t)
	c := ticketClient(srv.URL)
	c.Ticket, c.CSRFToken = "", ""
	c.AuthID, c.Secret = "tok@pbs!t", "wrong"
	err := c.SetComment("x")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("SetComment with a wrong token = %v, want the 401", err)
	}
	if f.notes != 1 || f.logins != 0 {
		t.Fatalf("notes requests %d, logins %d", f.notes, f.logins)
	}
}

// No way to get a new ticket: the error says why the comment is missing.
func TestSetCommentWithoutALoginSaysTheTicketExpired(t *testing.T) {
	_, srv := newFakeTicketPBS(t)
	c := ticketClient(srv.URL)
	err := c.SetComment("x")
	if err == nil || !strings.Contains(err.Error(), "ticket lasts 2 hours") {
		t.Fatalf("SetComment = %v, want the expired-ticket reason", err)
	}
}

// A new ticket PBS also refuses is not tried again and again.
func TestSetCommentRetriesOnlyOnce(t *testing.T) {
	f, srv := newFakeTicketPBS(t)
	c := ticketClient(srv.URL)
	c.RenewTicket = func() (string, string, error) { return "also-refused", "c", nil }
	err := c.SetComment("x")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("SetComment = %v, want the second 401", err)
	}
	if f.notes != 2 {
		t.Fatalf("notes requests %d, want 2", f.notes)
	}
}
