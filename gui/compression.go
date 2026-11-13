package main

import (
	"fmt"

	"pbscommon"
)

// Preferences > Backup Options "Compression": how hard each chunk is
// compressed before it is sent to PBS, which stores it as sent. One setting
// for every backup this computer makes, folder and machine alike; the
// service's settings are the ones its runs use. Encrypted chunks are
// compressed too, before they are encrypted; "none" sends every chunk as it
// is (pbscommon.UploadChunk).

// effectiveCompression is the level a run uses: one named for the run itself
// (a request to the service API that gives one), otherwise the setting,
// otherwise default (zstd level 3). It used to be "fastest", filled in at
// every layer a run passed through, and machine backups ignored it.
func (a *App) effectiveCompression(requested string) string {
	if requested != "" {
		return string(pbscommon.ParseCompressionLevel(requested))
	}
	return compressionSetting(a.config)
}

// compressionSetting is the level c's Backup Options setting gives: default
// when it is empty, or not one of the five (a hand-edited config.json).
func compressionSetting(c *Config) string {
	if c != nil && c.CompressionLevel != "" && pbscommon.ValidCompressionLevel(c.CompressionLevel) {
		return string(pbscommon.ParseCompressionLevel(c.CompressionLevel))
	}
	return string(pbscommon.CompressionDefault)
}

// SetCompressionLevel saves Preferences > Backup Options "Compression" and
// applies it to the next run to start. Same SaveConfig/Validate bypass as
// SetSpeedLimits.
func (a *App) SetCompressionLevel(level string) error {
	if !pbscommon.ValidCompressionLevel(level) {
		return fmt.Errorf("compression must be none, fastest, default, better or best, not %q", level)
	}
	if a.delegated() {
		return a.serviceCall("SetCompressionLevel", level)
	}
	a.config.CompressionLevel = level
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save the compression setting: %w", err)
	}
	writeDebugLog(fmt.Sprintf("Compression set to %s", a.effectiveCompression("")))
	return nil
}
