package main

import (
	"fmt"
	"time"

	"pbscommon"
)

// Preferences > Backup Options "Speed limits": upload to PBS, download from
// PBS and disk read, each in MB/s (decimal megabytes), 0 for none. The limits
// themselves live in pbscommon (ratelimit.go) as one budget per kind for the
// whole process, so every backup and restore running at the same time
// shares it. The service runs the scheduled backups, so its budgets are the
// ones that count for them; an app working on its own settings has its own.

// MaxSpeedLimitMBps is the most a speed limit may be set to (100 GB/s, far
// above any real link or disk; it only keeps the arithmetic sane).
const MaxSpeedLimitMBps = 100000

// applySpeedLimits hands the settings' speed limits to pbscommon. Both builds
// call it whenever their settings are loaded or replaced (start, ReloadConfig
// before each run, the service's settings arriving in the app) and after
// SetSpeedLimits, so a change applies at once, also to runs already going.
//
// Upload threads and Chunk timeout (Preferences > Backup Options) go to
// pbscommon the same way.
func applySpeedLimits(c *Config) {
	var up, down, disk, threads, chunk int
	if c != nil {
		up, down, disk, threads, chunk = c.UploadLimitMBps, c.DownloadLimitMBps, c.DiskReadLimitMBps, c.UploadThreads, c.ChunkTimeoutSeconds
	}
	pbscommon.SetSpeedLimit(pbscommon.LimitUpload, mbpsToBytes(up))
	pbscommon.SetSpeedLimit(pbscommon.LimitDownload, mbpsToBytes(down))
	pbscommon.SetSpeedLimit(pbscommon.LimitDiskRead, mbpsToBytes(disk))
	// Out of range (hand-edited): auto, and the default chunk timeout.
	pbscommon.SetUploadThreads(threads)
	pbscommon.SetChunkStallTimeout(time.Duration(chunk) * time.Second)
}

// mbpsToBytes turns a setting in MB/s into bytes per second; anything out of
// range (a hand-edited config.json) counts as no limit.
func mbpsToBytes(mbps int) int64 {
	if mbps <= 0 || mbps > MaxSpeedLimitMBps {
		return 0
	}
	return int64(mbps) * 1000 * 1000
}

// SetSpeedLimits saves Preferences > Backup Options "Speed limits" and applies
// them at once. Same SaveConfig/Validate bypass as SetMaxConcurrentRuns.
func (a *App) SetSpeedLimits(upload, download, diskRead int) error {
	for _, v := range []int{upload, download, diskRead} {
		if v < 0 || v > MaxSpeedLimitMBps {
			return fmt.Errorf("speed limits must be between 0 and %d MB/s", MaxSpeedLimitMBps)
		}
	}
	if a.delegated() {
		// serviceCall reads the service's settings back (hydrateFromService),
		// and with them the limits for the restores and browsing this app
		// does itself.
		return a.serviceCall("SetSpeedLimits", upload, download, diskRead)
	}
	a.config.UploadLimitMBps = upload
	a.config.DownloadLimitMBps = download
	a.config.DiskReadLimitMBps = diskRead
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save the speed limits: %w", err)
	}
	applySpeedLimits(a.config)
	writeDebugLog(fmt.Sprintf("Speed limits set: upload %d MB/s, download %d MB/s, disk read %d MB/s (0 = none)", upload, download, diskRead))
	return nil
}

// SetUploadThreads saves Preferences > Backup Options "Upload threads": how
// many chunks a backup sends to PBS at a time, 1 to MaxUploadThreads, or 0
// for auto. It applies at once, also to backups already running.
func (a *App) SetUploadThreads(n int) error {
	if n < 0 || n > pbscommon.MaxUploadThreads {
		return fmt.Errorf("upload threads must be auto (0) or between 1 and %d", pbscommon.MaxUploadThreads)
	}
	if a.delegated() {
		return a.serviceCall("SetUploadThreads", n)
	}
	a.config.UploadThreads = n
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save the upload threads setting: %w", err)
	}
	applySpeedLimits(a.config)
	writeDebugLog(fmt.Sprintf("Upload threads set to %d (0 = auto)", n))
	return nil
}

// SetChunkTimeout saves Preferences > Backup Options "Chunk timeout": how many
// seconds a chunk upload may make no progress before it is given up and sent
// again, 30 to 1800, or 0 for the default (90). It applies to the uploads that
// start after it, also in backups already running.
func (a *App) SetChunkTimeout(seconds int) error {
	if lo, hi := int(pbscommon.MinChunkStallTimeout/time.Second), int(pbscommon.MaxChunkStallTimeout/time.Second); seconds != 0 && (seconds < lo || seconds > hi) {
		return fmt.Errorf("the chunk timeout must be the default (0) or between %d and %d seconds", lo, hi)
	}
	if a.delegated() {
		return a.serviceCall("SetChunkTimeout", seconds)
	}
	a.config.ChunkTimeoutSeconds = seconds
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save the chunk timeout: %w", err)
	}
	applySpeedLimits(a.config)
	writeDebugLog(fmt.Sprintf("Chunk timeout set to %d s (0 = default)", seconds))
	return nil
}
