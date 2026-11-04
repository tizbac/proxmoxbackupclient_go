package clientcommon

import (
	"flag"
	"testing"

	"pbscommon"
)

// Flags given override the config file's limits; flags left out keep them.
func TestSpeedLimitFlags(t *testing.T) {
	fs := flag.NewFlagSet("tool", flag.ContinueOnError)
	apply := SpeedLimitFlags(fs)
	if err := fs.Parse([]string{"-upload-limit", "10", "-disk-read-limit", "0"}); err != nil {
		t.Fatal(err)
	}
	l := SpeedLimits{UploadMBps: 1, DownloadMBps: 2, DiskReadMBps: 3} // from the file
	apply(&l)
	if l != (SpeedLimits{UploadMBps: 10, DownloadMBps: 2, DiskReadMBps: 0}) {
		t.Errorf("limits %+v", l)
	}
}

func TestSpeedLimitsProblem(t *testing.T) {
	for _, c := range []struct {
		l    SpeedLimits
		want string
	}{
		{SpeedLimits{}, ""},
		{SpeedLimits{UploadMBps: MaxSpeedLimitMBps, DownloadMBps: 1, DiskReadMBps: 50}, ""},
		{SpeedLimits{UploadMBps: -1}, "-upload-limit is -1: use 0 (no limit) to 100000 MB/s"},
		{SpeedLimits{DownloadMBps: MaxSpeedLimitMBps + 1}, "-download-limit is 100001: use 0 (no limit) to 100000 MB/s"},
		{SpeedLimits{DiskReadMBps: -5}, "-disk-read-limit is -5: use 0 (no limit) to 100000 MB/s"},
	} {
		if got := c.l.SpeedLimitProblem(); got != c.want {
			t.Errorf("%+v: %q, want %q", c.l, got, c.want)
		}
	}
}

func TestSpeedLimitsApply(t *testing.T) {
	t.Cleanup(func() { SpeedLimits{}.ApplySpeedLimits() })
	line := SpeedLimits{UploadMBps: 10, DiskReadMBps: 3}.ApplySpeedLimits()
	if line != "Speed limits: upload 10 MB/s, download none, disk read 3 MB/s" {
		t.Errorf("line %q", line)
	}
	if pbscommon.SpeedLimit(pbscommon.LimitUpload) != 10_000_000 || pbscommon.SpeedLimit(pbscommon.LimitDownload) != 0 ||
		pbscommon.SpeedLimit(pbscommon.LimitDiskRead) != 3_000_000 {
		t.Errorf("budgets %d %d %d", pbscommon.SpeedLimit(pbscommon.LimitUpload), pbscommon.SpeedLimit(pbscommon.LimitDownload),
			pbscommon.SpeedLimit(pbscommon.LimitDiskRead))
	}
	if line := (SpeedLimits{}).ApplySpeedLimits(); line != "" || pbscommon.SpeedLimit(pbscommon.LimitUpload) != 0 {
		t.Errorf("none: line %q, upload %d", line, pbscommon.SpeedLimit(pbscommon.LimitUpload))
	}
}

// -upload-threads takes 0 (auto) to 8, overrides the file, and is printed.
func TestUploadThreads(t *testing.T) {
	t.Cleanup(func() { SpeedLimits{}.ApplySpeedLimits() })
	fs := flag.NewFlagSet("tool", flag.ContinueOnError)
	apply := SpeedLimitFlags(fs)
	if err := fs.Parse([]string{"-upload-threads", "2"}); err != nil {
		t.Fatal(err)
	}
	l := SpeedLimits{UploadMBps: 5, UploadThreads: 6} // from the file
	apply(&l)
	if l != (SpeedLimits{UploadMBps: 5, UploadThreads: 2}) {
		t.Errorf("settings %+v", l)
	}
	if p := (SpeedLimits{UploadThreads: 9}).SpeedLimitProblem(); p != "-upload-threads is 9: use 0 (auto) to 8" {
		t.Errorf("9: %q", p)
	}
	if line := l.ApplySpeedLimits(); line != "Speed limits: upload 5 MB/s, download none, disk read none; Upload threads: 2" || pbscommon.UploadThreads() != 2 {
		t.Errorf("line %q, threads %d", line, pbscommon.UploadThreads())
	}
	if line := (SpeedLimits{UploadThreads: 1}).ApplySpeedLimits(); line != "Upload threads: 1" {
		t.Errorf("threads only: %q", line)
	}
	if (SpeedLimits{}).ApplySpeedLimits(); pbscommon.UploadThreads() != 0 {
		t.Errorf("auto: %d", pbscommon.UploadThreads())
	}
}
