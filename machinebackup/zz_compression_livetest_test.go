package machinebackup

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// -compression reaches the machine backup's uploads: the same compressible
// disk image sent at fastest and at best, each to a group of its own (so
// nothing is reused), and best sends less. Machine backups always used the
// default level before. Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID,
// PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE.
func TestCompressionLevelLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	// 12 MB of text from a small vocabulary in a random order: compressible,
	// but not so plain that every level gets it down to the same size.
	words := strings.Fields("backup restore chunk index snapshot volume partition sector datastore namespace manifest catalog archive digest encrypt compress upload download verify prune garbage collect schedule retention")
	r := rand.New(rand.NewSource(rand.Int63()))
	var b strings.Builder
	for b.Len() < 12*1000*1000 {
		b.WriteString(words[r.Intn(len(words))])
		b.WriteString(strconv.Itoa(r.Intn(1000)))
		b.WriteByte(' ')
	}
	dev := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(dev, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	sentRe := regexp.MustCompile(`Done in [^:]+:.* transferred ([0-9.]+)(B|kB|MB|GB)`)
	sent := func(level string) float64 {
		cmd := exec.Command(os.Args[0],
			"-baseurl", url,
			"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
			"-authid", os.Getenv("PBS_LIVE_AUTHID"),
			"-secret", os.Getenv("PBS_LIVE_SECRET"),
			"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
			"-backupdev", dev,
			"-backup-id", "machine-compression-test-"+level,
			"-compression", level,
		)
		cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: exit %v\n%s", level, err, out)
		}
		m := sentRe.FindStringSubmatch(string(out))
		if m == nil {
			t.Fatalf("%s: no transferred figure\n%s", level, out)
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		v *= map[string]float64{"B": 1, "kB": 1e3, "MB": 1e6, "GB": 1e9}[m[2]]
		t.Logf("%s: transferred %s%s", level, m[1], m[2])
		return v
	}
	fastest, best := sent("fastest"), sent("best")
	if best >= fastest {
		t.Errorf("best sent %.0f bytes, fastest %.0f: the level did not reach the upload", best, fastest)
	}
}
