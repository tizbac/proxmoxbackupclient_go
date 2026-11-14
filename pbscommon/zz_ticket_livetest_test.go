package pbscommon

import (
	"os"
	"strings"
	"testing"
)

// A live test against a real PBS of what happens to a long backup's comment
// when its ticket has expired (a PBS ticket lasts 2 hours; tiz found long
// backups ending without their comment). A spoiled ticket stands in for the
// expired one. Off unless PBS_TICKET_LIVE=1, with a username/password login
// that owns the newest snapshot of PBS_TICKET_GROUP (host/<id>):
//
//	PBS_TICKET_URL, PBS_TICKET_FP, PBS_TICKET_STORE, PBS_TICKET_USER,
//	PBS_TICKET_PASSWORD, PBS_TICKET_GROUP
func TestLiveCommentAfterTheTicketExpired(t *testing.T) {
	if os.Getenv("PBS_TICKET_LIVE") != "1" {
		t.Skip("PBS_TICKET_LIVE=1 and PBS_TICKET_* to run against a real PBS")
	}
	base := func() *PBSClient {
		return &PBSClient{
			BaseURL: os.Getenv("PBS_TICKET_URL"), CertFingerPrint: os.Getenv("PBS_TICKET_FP"),
			Insecure: os.Getenv("PBS_TICKET_FP") != "", Datastore: os.Getenv("PBS_TICKET_STORE"),
		}
	}
	login := base()
	login.Username, login.Password = os.Getenv("PBS_TICKET_USER"), os.Getenv("PBS_TICKET_PASSWORD")
	if err := login.ObtainTicket(); err != nil {
		t.Fatalf("login: %v", err)
	}
	snaps, err := login.ListGroupSnapshots("host", os.Getenv("PBS_TICKET_GROUP"))
	if err != nil || len(snaps) == 0 {
		t.Fatalf("snapshots of host/%s: %d, %v", os.Getenv("PBS_TICKET_GROUP"), len(snaps), err)
	}
	newest := snaps[0]
	for _, s := range snaps {
		if s.BackupTime > newest.BackupTime {
			newest = s
		}
	}
	snap := BackupManifest{BackupType: "host", BackupID: os.Getenv("PBS_TICKET_GROUP"), BackupTime: newest.BackupTime}
	// A ticket PBS no longer takes: the real one with its signature spoiled.
	spoilt := login.Ticket[:len(login.Ticket)-8] + "AAAAAAAA"

	// As before the fix: only the ticket, no way to a new one.
	before := base()
	before.Ticket, before.CSRFToken, before.Manifest = spoilt, login.CSRFToken, snap
	err = before.SetComment("not set")
	t.Logf("LIVE without a way to renew: %v", err)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("an expired ticket was taken, or failed otherwise: %v", err)
	}

	// The CLI: the client holds the login and gets a fresh ticket.
	cli := base()
	cli.Username, cli.Password = login.Username, login.Password
	cli.Ticket, cli.CSRFToken, cli.Manifest = spoilt, login.CSRFToken, snap
	if err := cli.SetComment("comment set after the ticket expired (CLI)"); err != nil {
		t.Fatalf("CLI: %v", err)
	}
	got, err := cli.GetComment()
	t.Logf("LIVE CLI: comment now %q, %v", got, err)
	if got != "comment set after the ticket expired (CLI)" {
		t.Fatalf("CLI: comment %q, %v", got, err)
	}

	// The app: only a ticket and the hook (App.ticketRenewer mints one as here).
	app := base()
	app.Ticket, app.CSRFToken, app.Manifest = spoilt, login.CSRFToken, snap
	calls := 0
	app.RenewTicket = func() (string, string, error) {
		calls++
		fresh := base()
		fresh.Username, fresh.Password = login.Username, login.Password
		if err := fresh.ObtainTicket(); err != nil {
			return "", "", err
		}
		return fresh.Ticket, fresh.CSRFToken, nil
	}
	if err := app.SetComment("comment set after the ticket expired (app)"); err != nil {
		t.Fatalf("app: %v", err)
	}
	got, err = app.GetComment()
	t.Logf("LIVE app: comment now %q, %v; renewals %d", got, err, calls)
	if got != "comment set after the ticket expired (app)" || calls != 1 {
		t.Fatalf("app: comment %q, %v, renewals %d", got, err, calls)
	}
}
