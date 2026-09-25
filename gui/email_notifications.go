package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"clientcommon"
)

// SetSMTPSettings persists the Preferences > Advanced global SMTP account
// used by every Backup Set's own on-completion/on-failure email toggle.
// Bypasses SaveConfig/Validate the same way SetParallelRestore does — see its
// doc comment for why. An empty password means "keep the existing one" (the
// frontend never receives the real stored password, matching SaveConfig's
// existing SMTPPassword handling), so clearing it for real requires deleting
// the whole account (host empty).
func (a *App) SetSMTPSettings(host, port, username, password string, insecure bool, from string) error {
	a.config.SMTPHost = host
	a.config.SMTPPort = port
	a.config.SMTPUsername = username
	if password != "" {
		a.config.SMTPPassword = password
	}
	a.config.SMTPInsecure = insecure
	a.config.EmailFrom = from
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save SMTP settings: %w", err)
	}
	return nil
}

// SendTestEmail sends a short test message to `to` using the currently saved
// global SMTP account, so a user can confirm their SMTP settings actually
// work before relying on them for real backup notifications.
func (a *App) SendTestEmail(to string) error {
	if a.config.SMTPHost == "" {
		return fmt.Errorf("no SMTP server configured — set one in Preferences first")
	}
	client, err := clientcommon.SetupMailClient(a.config.SMTPHost, a.config.SMTPPort, a.config.SMTPUsername, a.config.SMTPPassword, a.config.SMTPInsecure)
	if err != nil {
		return fmt.Errorf("could not connect to mail server: %w", err)
	}
	defer client.Quit()

	from := a.config.EmailFrom
	if from == "" {
		from = a.config.SMTPUsername
	}
	body := fmt.Sprintf("This is a test message from Proxmox Backup Client Go, sent at %s.\n\nIf you received this, your SMTP settings are working correctly.", time.Now().Format(time.RFC1123))
	if err := clientcommon.SendMail(from, to, "Proxmox Backup Client Go — test email", body, client); err != nil {
		return fmt.Errorf("could not send test email: %w", err)
	}
	return nil
}

// sendJobNotificationEmail sends one on-completion/on-failure notification
// for a finished Backup Set, using the global SMTP account. Best-effort: a
// failure here is logged, never propagated (a backup that actually
// succeeded/failed must not be reported differently just because the
// notification email itself couldn't be sent).
func (a *App) sendJobNotificationEmail(job ScheduledJob, success bool, message string) {
	to := job.EmailOnSuccessTo
	enabled := job.EmailOnSuccess
	if !success {
		to = job.EmailOnFailureTo
		enabled = job.EmailOnFailure
	}
	if !enabled || to == "" {
		return
	}
	if a.config.SMTPHost == "" {
		writeDebugLog(fmt.Sprintf("[PostBackup] %q wants an email on %s but no SMTP server is configured in Preferences — skipping",
			job.Name, map[bool]string{true: "completion", false: "failure"}[success]))
		return
	}

	client, err := clientcommon.SetupMailClient(a.config.SMTPHost, a.config.SMTPPort, a.config.SMTPUsername, a.config.SMTPPassword, a.config.SMTPInsecure)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[PostBackup] could not connect to mail server for job %q: %v", job.Name, err))
		return
	}
	defer client.Quit()

	from := a.config.EmailFrom
	if from == "" {
		from = a.config.SMTPUsername
	}
	status := "completed"
	if !success {
		status = "FAILED"
	}
	subject := fmt.Sprintf("Proxmox Backup Client Go — %q %s", job.Name, status)
	hostname, _ := os.Hostname()
	body := fmt.Sprintf("Backup Set %q %s on %s at %s.\n\n%s",
		job.Name, status, hostname, time.Now().Format(time.RFC1123), message)

	if err := clientcommon.SendMail(from, to, subject, body, client); err != nil {
		writeDebugLog(fmt.Sprintf("[PostBackup] could not send notification email for job %q: %v", job.Name, err))
	}
}

// runPreBackupApp runs a Backup Set's configured "before" application, if
// any. Best-effort: logged, never blocks the backup itself from starting.
func runPreBackupApp(job ScheduledJob) {
	runPostBackupApp(job.RunAppBefore, job.Name, "before")
}

func runPostBackupApp(command, jobName, when string) {
	if command == "" {
		return
	}
	writeDebugLog(fmt.Sprintf("[PostBackup] running %s-backup application for job %q: %s", when, jobName, command))
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		writeDebugLog(fmt.Sprintf("[PostBackup] %s-backup application for job %q failed: %v: %s", when, jobName, err, string(out)))
	}
}

// runPostBackupActions runs everything a finished Backup Set is configured
// to do after it completes: email notification, an application, then exiting
// the app and/or shutting down the computer — deliberately in that order,
// since a shutdown is irreversible and must be the very last thing attempted.
func (a *App) runPostBackupActions(job ScheduledJob, success bool, message string) {
	a.sendJobNotificationEmail(job, success, message)
	runPostBackupApp(job.RunAppAfter, job.Name, "after")

	if job.ExitAppAfter {
		writeDebugLog(fmt.Sprintf("[PostBackup] job %q configured to exit the app after backup — exiting", job.Name))
		go func() {
			time.Sleep(2 * time.Second) // give the history write and any event emit a moment to land
			os.Exit(0)
		}()
	}
	if job.ShutdownAfter {
		writeDebugLog(fmt.Sprintf("[PostBackup] job %q configured to shut down the computer after backup — shutting down", job.Name))
		go func() {
			time.Sleep(2 * time.Second)
			shutdownComputer()
		}()
	}
}

// shutdownComputer requests an OS shutdown. Best-effort: logged on failure,
// there is nothing more a running backup client can do if the OS itself
// refuses the request (e.g. insufficient privilege).
func shutdownComputer() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("shutdown", "/s", "/t", "0")
	default:
		cmd = exec.Command("shutdown", "-h", "now")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		writeDebugLog(fmt.Sprintf("[PostBackup] shutdown command failed: %v: %s", err, string(out)))
	}
}
