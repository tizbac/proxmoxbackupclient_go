package main

import (
	"bufio"
	"clientcommon"
	"errors"
	"fmt"
	"io"
	"machinebackuplib"
	"net"
	"net/mail"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is just enough of an SMTP server for net/smtp: it offers AUTH
// PLAIN, accepts any login and keeps every message it is sent.
type fakeSMTP struct {
	ln   net.Listener
	mu   sync.Mutex
	msgs []fakeMail
}

type fakeMail struct {
	from string
	to   []string
	data string
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSMTP{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *fakeSMTP) port() string {
	return fmt.Sprint(s.ln.Addr().(*net.TCPAddr).Port)
}

func (s *fakeSMTP) mails() []fakeMail {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fakeMail(nil), s.msgs...)
}

func (s *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	reply := func(l string) { fmt.Fprintf(c, "%s\r\n", l) }
	reply("220 localhost fake ESMTP")
	var cur fakeMail
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			reply("250-localhost")
			reply("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "HELO"):
			reply("250 localhost")
		case strings.HasPrefix(cmd, "AUTH"):
			reply("235 2.7.0 Authentication successful")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			cur = fakeMail{from: strings.Trim(line[len("MAIL FROM:"):], "<> ")}
			reply("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			cur.to = append(cur.to, strings.Trim(line[len("RCPT TO:"):], "<> "))
			reply("250 OK")
		case cmd == "DATA":
			reply("354 End data with <CR><LF>.<CR><LF>")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			cur.data = b.String()
			s.mu.Lock()
			s.msgs = append(s.msgs, cur)
			s.mu.Unlock()
			reply("250 OK")
		case cmd == "RSET":
			reply("250 OK")
		case cmd == "QUIT":
			reply("221 Bye")
			return
		default:
			reply("502 Command not implemented")
		}
	}
}

// subjectAndBody splits a received message into its Subject header and its
// body (with \n line ends).
func subjectAndBody(t *testing.T, m fakeMail) (string, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(m.data))
	if err != nil {
		t.Fatalf("cannot parse the received message: %v\n%s", err, m.data)
	}
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		t.Fatal(err)
	}
	// The line end before the closing "." belongs to the SMTP transfer.
	return msg.Header.Get("Subject"), strings.TrimSuffix(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
}

func testSMTPConfig(srv *fakeSMTP) *machinebackuplib.SMTPConfig {
	return &machinebackuplib.SMTPConfig{
		Host:     "127.0.0.1",
		Port:     srv.port(),
		Username: "user",
		Password: "password",
		Mails: []machinebackuplib.MailSendConfig{
			{From: "backup@example.com", To: "admin@example.com"},
			{From: "backup@example.com", To: "other@example.com"},
		},
	}
}

func TestSendMailSuccess(t *testing.T) {
	srv := startFakeSMTP(t)
	start := time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC)
	err := sendMail(testSMTPConfig(srv), clientcommon.MailCtx{
		Hostname:  "pc1",
		Datastore: "store",
		StartTime: start,
		EndTime:   start.Add(90 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	mails := srv.mails()
	if len(mails) != 2 {
		t.Fatalf("%d e-mails received, want 2", len(mails))
	}
	for i, want := range []string{"admin@example.com", "other@example.com"} {
		if len(mails[i].to) != 1 || mails[i].to[0] != want {
			t.Errorf("mail %d went to %q, want %q", i, mails[i].to, want)
		}
		subject, body := subjectAndBody(t, mails[i])
		if subject != "Backup Success" {
			t.Errorf("subject %q, want %q", subject, "Backup Success")
		}
		if body != "Backup complete (1m30s)" {
			t.Errorf("body %q, want %q", body, "Backup complete (1m30s)")
		}
	}
}

func TestSendMailCustomTemplates(t *testing.T) {
	srv := startFakeSMTP(t)
	cfg := testSMTPConfig(srv)
	cfg.Mails = cfg.Mails[:1]
	cfg.Template = &machinebackuplib.MailTemplate{
		Subject: "{{.Hostname}} to {{.Datastore}}: {{.Status}}",
		// Not a valid template: the default body is used instead.
		Body: "{{if .Success}",
	}
	err := sendMail(cfg, clientcommon.MailCtx{
		Error:     errors.New("disk gone"),
		Hostname:  "pc1",
		Datastore: "store",
	})
	if err != nil {
		t.Fatal(err)
	}
	mails := srv.mails()
	if len(mails) != 1 {
		t.Fatalf("%d e-mails received, want 1", len(mails))
	}
	subject, body := subjectAndBody(t, mails[0])
	if subject != "pc1 to store: Failed" {
		t.Errorf("subject %q", subject)
	}
	want := "Error occurred while working, backup may be not completed.\nLast error is: disk gone"
	if body != want {
		t.Errorf("body %q, want %q", body, want)
	}
}

// TestMain runs main() itself when a test starts this test binary again with
// MACHINEBACKUP_RUN_MAIN set, so a test can run the program as a whole.
func TestMain(m *testing.M) {
	if os.Getenv("MACHINEBACKUP_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A run whose backup fails still sends the notification e-mail, with the
// error in it, and still exits 1. The backup fails before any connection to
// PBS (a "vm" backup needs a numeric backup ID), so no server is needed.
func TestFailedBackupSendsMail(t *testing.T) {
	srv := startFakeSMTP(t)
	cmd := exec.Command(os.Args[0],
		"-baseurl", "https://127.0.0.1:1",
		"-certfingerprint", "00:11:22",
		"-authid", "test@pbs!test",
		"-secret", "secret",
		"-datastore", "store",
		"-backupdev", "nosuchdisk.img",
		"-type", "vm",
		"-backup-id", "not-a-number",
		"-mail-host", "127.0.0.1",
		"-mail-port", srv.port(),
		"-mail-username", "user",
		"-mail-password", "password",
		"-mail-from", "backup@example.com",
		"-mail-to", "admin@example.com",
	)
	cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected a non-zero exit, got %v\n%s", err, out)
	}
	if exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("exit code %d, want 1\n%s", exitErr.ExitCode(), out)
	}

	mails := srv.mails()
	if len(mails) != 1 {
		t.Fatalf("%d e-mails received, want 1\n%s", len(mails), out)
	}
	m := mails[0]
	if m.from != "backup@example.com" || len(m.to) != 1 || m.to[0] != "admin@example.com" {
		t.Errorf("envelope from %q to %q", m.from, m.to)
	}
	subject, body := subjectAndBody(t, m)
	if subject != "Backup Failed" {
		t.Errorf("subject %q, want %q", subject, "Backup Failed")
	}
	if !strings.Contains(body, "Last error is: ") || !strings.Contains(body, "needs a numeric VM ID") {
		t.Errorf("body does not carry the backup error:\n%s", body)
	}
	t.Logf("subject: %s\nbody:\n%s", subject, body)
}

// The devices to back up can come from the -config file alone. The run goes
// as far as the backup itself, which fails on the non-numeric VM ID before
// any connection to PBS; without the file's devices it stopped earlier, at
// "Missing or invalid options".
func TestConfigFileBackupDevices(t *testing.T) {
	cfg := t.TempDir() + string(os.PathSeparator) + "config.json"
	err := os.WriteFile(cfg, []byte(`{
		"baseurl": "https://127.0.0.1:1",
		"certfingerprint": "00:11:22",
		"authid": "test@pbs!test",
		"secret": "secret",
		"datastore": "store",
		"backupdev": ["nosuchdisk.img"],
		"backuptype": "vm",
		"backup-id": "not-a-number"
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-config", cfg)
	cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected a non-zero exit, got %v\n%s", err, out)
	}
	if exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	if strings.Contains(string(out), "Missing or invalid options") {
		t.Fatalf("the config file's backupdev was not used\n%s", out)
	}
	if !strings.Contains(string(out), "needs a numeric VM ID") {
		t.Fatalf("expected the backup to fail on the VM ID\n%s", out)
	}
}

// A successful backup sends one "Backup Success" e-mail. It backs up a small
// file as the device, so it needs a PBS but no administrator rights:
// PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and
// PBS_LIVE_DATASTORE.
func TestSuccessfulBackupSendsMail(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dev := t.TempDir() + string(os.PathSeparator) + "disk.img"
	if err := os.WriteFile(dev, []byte(strings.Repeat("machine backup mail test\n", 400000)), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := startFakeSMTP(t)
	cmd := exec.Command(os.Args[0],
		"-baseurl", url,
		"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
		"-authid", os.Getenv("PBS_LIVE_AUTHID"),
		"-secret", os.Getenv("PBS_LIVE_SECRET"),
		"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
		"-backupdev", dev,
		"-backup-id", "machine-mail-test",
		"-mail-host", "127.0.0.1",
		"-mail-port", srv.port(),
		"-mail-username", "user",
		"-mail-password", "password",
		"-mail-from", "backup@example.com",
		"-mail-to", "admin@example.com",
	)
	cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("backup failed: %v\n%s", err, out)
	}
	mails := srv.mails()
	if len(mails) != 1 {
		t.Fatalf("%d e-mails received, want 1\n%s", len(mails), out)
	}
	subject, body := subjectAndBody(t, mails[0])
	if subject != "Backup Success" || !strings.HasPrefix(body, "Backup complete (") {
		t.Errorf("subject %q body %q", subject, body)
	}
	t.Logf("subject: %s\nbody:\n%s", subject, body)
}
