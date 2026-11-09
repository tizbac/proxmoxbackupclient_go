package tui

import (
	"time"
)

// Prefs are the computer's Preferences that the window edits: backup options
// and the e-mail account, as the config document has them.
type Prefs struct {
	UploadMBps, DownloadMBps, DiskReadMBps int
	UploadThreads                          int // 0 = automatic
	ChunkTimeout                           int // seconds, 0 = default
	Compression                            string
	ParallelRestore                        bool
	ParallelRead                           bool
	ReadWorkers                            int
	MaxRuns                                int // 0 = default

	SMTPHost, SMTPPort, SMTPUser string
	SMTPPasswordSet              bool
	SMTPPassword                 string // only ever typed in; empty keeps the stored one
	SMTPInsecure                 bool
	MailFrom                     string
}

func (p Prefs) limitsChanged(o Prefs) bool {
	return p.UploadMBps != o.UploadMBps || p.DownloadMBps != o.DownloadMBps || p.DiskReadMBps != o.DiskReadMBps
}

func (p Prefs) mailChanged(o Prefs) bool {
	return p.SMTPHost != o.SMTPHost || p.SMTPPort != o.SMTPPort || p.SMTPUser != o.SMTPUser ||
		p.SMTPInsecure != o.SMTPInsecure || p.MailFrom != o.MailFrom || p.SMTPPassword != ""
}

// toInt reads a JSON number of a config document.
func toInt(m map[string]interface{}, k string) int { return int(num(m, k)) }

func prefsFromConfig(doc map[string]interface{}) Prefs {
	return Prefs{
		UploadMBps: toInt(doc, "upload_limit_mbps"), DownloadMBps: toInt(doc, "download_limit_mbps"),
		DiskReadMBps: toInt(doc, "disk_read_limit_mbps"), UploadThreads: toInt(doc, "upload_threads"),
		ChunkTimeout: toInt(doc, "chunk_timeout_seconds"), Compression: firstNonEmpty(str(doc, "compression_level"), "default"),
		ParallelRestore: boolean(doc, "parallel_restore"), ParallelRead: boolean(doc, "parallel_backup_read"),
		ReadWorkers: toInt(doc, "backup_read_workers"), MaxRuns: toInt(doc, "max_concurrent_runs"),
		SMTPHost: str(doc, "smtp_host"), SMTPPort: str(doc, "smtp_port"), SMTPUser: str(doc, "smtp_username"),
		SMTPPasswordSet: boolean(doc, "smtp_password_set"), SMTPInsecure: boolean(doc, "smtp_insecure"),
		MailFrom: str(doc, "email_from"),
	}
}

// KeyInfo describes a key file without the key: what the service reports.
type KeyInfo struct {
	Path, Fingerprint, Created, Hint, Reason string
	Exists, Usable                           bool
	PassphraseProtected, OpenToOthers        bool
}

func keyInfoFrom(m map[string]interface{}) KeyInfo {
	return KeyInfo{
		Path: str(m, "path"), Fingerprint: str(m, "fingerprint"), Created: str(m, "created"), Hint: str(m, "hint"),
		Reason: str(m, "reason"), Exists: boolean(m, "exists"), Usable: boolean(m, "usable"),
		PassphraseProtected: boolean(m, "passphrase_protected"), OpenToOthers: boolean(m, "open_to_others"),
	}
}

// LogFile is one of the service's log files.
type LogFile struct {
	Folder            int
	Name, Kind        string
	BackupID          string
	Started, Modified time.Time
	Size              int64
	Compressed        bool
	Service           bool
}

func logFileFrom(m map[string]interface{}) LogFile {
	f := LogFile{
		Folder: toInt(m, "folder"), Name: str(m, "name"), Kind: str(m, "kind"), BackupID: str(m, "backupId"),
		Size: int64(num(m, "size")), Compressed: boolean(m, "compressed"), Service: boolean(m, "service"),
	}
	if ms := int64(num(m, "started")); ms > 0 {
		f.Started = time.UnixMilli(ms)
	}
	if ms := int64(num(m, "modified")); ms > 0 {
		f.Modified = time.UnixMilli(ms)
	}
	return f
}
