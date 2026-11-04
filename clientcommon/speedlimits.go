package clientcommon

import (
	"flag"
	"fmt"
	"strings"

	"pbscommon"
)

// MaxSpeedLimitMBps is the most a speed limit may be set to, as in the app
// (gui/speed_limits.go): 100 GB/s, far above any link or disk.
const MaxSpeedLimitMBps = 100000

// SpeedLimits are the backup tools' -upload-limit, -download-limit and
// -disk-read-limit in MB/s (1 MB = 1,000,000 bytes), 0 for none: the app's
// Preferences > Backup Options > Speed limits. pbscommon charges them
// (ratelimit.go): upload and download on the connection to PBS, disk read
// where a folder backup reads a file and a machine backup reads a disk.
//
// UploadThreads is -upload-threads, the app's Upload threads: how many chunks
// go to PBS at a time, 1 to pbscommon.MaxUploadThreads, or 0 for auto, which
// starts with one and adds one for every 4 Mbit/s of upload speed measured
// (pbscommon/upload_threads.go).
type SpeedLimits struct {
	UploadMBps    int `json:"upload-limit,omitempty"`
	DownloadMBps  int `json:"download-limit,omitempty"`
	DiskReadMBps  int `json:"disk-read-limit,omitempty"`
	UploadThreads int `json:"upload-threads,omitempty"`
}

// SpeedLimitFlags defines the speed limit flags and -upload-threads on fs.
// The function it returns, called after parsing, copies the ones given on
// the command line over l, so a flag overrides the config file and one left
// out keeps the file's value.
func SpeedLimitFlags(fs *flag.FlagSet) func(l *SpeedLimits) {
	up := fs.Int("upload-limit", 0, "Send at most this many MB/s to PBS (1 MB = 1,000,000 bytes); 0 or left out: no limit")
	down := fs.Int("download-limit", 0, "Read at most this many MB/s from PBS (the previous backup's index and file list); 0 or left out: no limit")
	disk := fs.Int("disk-read-limit", 0, "Read at most this many MB/s from the disk being backed up; 0 or left out: no limit")
	threads := fs.Int("upload-threads", 0, fmt.Sprintf("Send this many chunks to PBS at a time, 1 to %d; 0 or left out: auto, which starts with one and adds one for every 4 Mbit/s of upload speed it measures, so a slow link is not shared between more chunks than it can carry", pbscommon.MaxUploadThreads))
	return func(l *SpeedLimits) {
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "upload-limit":
				l.UploadMBps = *up
			case "download-limit":
				l.DownloadMBps = *down
			case "disk-read-limit":
				l.DiskReadMBps = *disk
			case "upload-threads":
				l.UploadThreads = *threads
			}
		})
	}
}

// SpeedLimitProblem says which limit is out of range, or "".
func (l SpeedLimits) SpeedLimitProblem() string {
	for _, v := range []struct {
		name string
		mbps int
	}{{"upload-limit", l.UploadMBps}, {"download-limit", l.DownloadMBps}, {"disk-read-limit", l.DiskReadMBps}} {
		if v.mbps < 0 || v.mbps > MaxSpeedLimitMBps {
			return fmt.Sprintf("-%s is %d: use 0 (no limit) to %d MB/s", v.name, v.mbps, MaxSpeedLimitMBps)
		}
	}
	if l.UploadThreads < 0 || l.UploadThreads > pbscommon.MaxUploadThreads {
		return fmt.Sprintf("-upload-threads is %d: use 0 (auto) to %d", l.UploadThreads, pbscommon.MaxUploadThreads)
	}
	return ""
}

// ApplySpeedLimits sets the limits and upload threads for this process and
// returns a line saying what they are, or "" when none is set. Call it after
// SpeedLimitProblem has passed.
func (l SpeedLimits) ApplySpeedLimits() string {
	pbscommon.SetSpeedLimit(pbscommon.LimitUpload, int64(l.UploadMBps)*1000*1000)
	pbscommon.SetSpeedLimit(pbscommon.LimitDownload, int64(l.DownloadMBps)*1000*1000)
	pbscommon.SetSpeedLimit(pbscommon.LimitDiskRead, int64(l.DiskReadMBps)*1000*1000)
	pbscommon.SetUploadThreads(l.UploadThreads)
	var parts []string
	if limits := (SpeedLimits{l.UploadMBps, l.DownloadMBps, l.DiskReadMBps, 0}); limits != (SpeedLimits{}) {
		show := func(name string, mbps int) string {
			if mbps <= 0 {
				return name + " none"
			}
			return fmt.Sprintf("%s %d MB/s", name, mbps)
		}
		parts = append(parts, "Speed limits: "+strings.Join([]string{
			show("upload", l.UploadMBps), show("download", l.DownloadMBps), show("disk read", l.DiskReadMBps),
		}, ", "))
	}
	if l.UploadThreads > 0 {
		parts = append(parts, fmt.Sprintf("Upload threads: %d", l.UploadThreads))
	}
	return strings.Join(parts, "; ")
}
