package main

import (
	"fmt"

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
func applySpeedLimits(c *Config) {
	var up, down, disk int
	if c != nil {
		up, down, disk = c.UploadLimitMBps, c.DownloadLimitMBps, c.DiskReadLimitMBps
	}
	pbscommon.SetSpeedLimit(pbscommon.LimitUpload, mbpsToBytes(up))
	pbscommon.SetSpeedLimit(pbscommon.LimitDownload, mbpsToBytes(down))
	pbscommon.SetSpeedLimit(pbscommon.LimitDiskRead, mbpsToBytes(disk))
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
