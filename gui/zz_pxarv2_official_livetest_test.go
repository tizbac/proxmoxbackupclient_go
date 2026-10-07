package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in interoperability test (PBS_PXARV2=1) against snapshots written by the
// OFFICIAL proxmox-backup-client 3.4.9 on the test PBS, in each
// change-detection mode, from the same fixture tree:
//
//	host/pxarv2-legacy @1791337552  legacy   (fixture.pxar.didx + catalog)
//	host/pxarv2-split  @1791337553  data     (fixture.mpxar.didx + fixture.ppxar.didx)
//	host/pxarv2-split  @1791337554  metadata (300 of 309 files reused, padding)
//	host/pxarv2-split  @1791337568  metadata (306 reused, 4 partially reused chunks)
//
// testdata/pxarv2/<label>.sha256 is `sha256sum` of every regular file at the
// moment each backup ran (recorded on the source host). Every snapshot is
// browsed, fully restored (sequential and parallel) and partially restored,
// and every restored file is compared with those hashes.
//
// Known limitation, same as for classic archives today: the second name of a
// hard-linked pair is a PXAR_HARDLINK record, which the reader skips, so that
// one path is reported but not counted as a failure.

type pxarv2Snapshot struct {
	label    string
	backupID string
	unix     int64
}

var pxarv2Snapshots = []pxarv2Snapshot{
	{"legacy", "pxarv2-legacy", 1791337552},
	{"data", "pxarv2-split", 1791337553},
	{"metadata1", "pxarv2-split", 1791337554},
	{"metadata2", "pxarv2-split", 1791337568},
}

// Paths the reader is known not to restore (see above).
var pxarv2KnownSkipped = map[string]bool{"a/b/hardlink-dst.txt": true, "a/hardlink-src.txt": true}

func pxarv2Expected(t *testing.T, label string) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "pxarv2", label+".sha256"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 67 {
			continue
		}
		m[strings.TrimPrefix(line[66:], "./")] = line[:64]
	}
	return m
}

func pxarv2Opts(s pxarv2Snapshot) RestoreOptions {
	return RestoreOptions{
		BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret,
		Datastore: e2eStore, BackupID: s.backupID, SnapshotTime: time.Unix(s.unix, 0).UTC(),
	}
}

// pxarv2Restored hashes every regular file under the directory that holds the
// restored tree (the one containing "a/hello.txt"), keyed by relative path.
func pxarv2Restored(t *testing.T, dest string) map[string]string {
	t.Helper()
	root := ""
	filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
		if err == nil && root == "" && !info.IsDir() && filepath.ToSlash(p) != "" && strings.HasSuffix(filepath.ToSlash(p), "/a/hello.txt") {
			root = filepath.Dir(filepath.Dir(p))
		}
		return nil
	})
	got := map[string]string{}
	if root == "" {
		return got
	}
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		f, ferr := os.Open(p)
		if ferr != nil {
			return nil
		}
		h := sha256.New()
		io.Copy(h, f)
		f.Close()
		got[filepath.ToSlash(rel)] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return got
}

func pxarv2Compare(t *testing.T, label string, want, got map[string]string, only func(string) bool) {
	t.Helper()
	checked, missing, wrong := 0, 0, 0
	for p, sum := range want {
		if only != nil && !only(p) {
			continue
		}
		g, ok := got[p]
		switch {
		case !ok && pxarv2KnownSkipped[p]:
			t.Logf("%s: %s not restored (hard link, known limitation)", label, p)
		case !ok:
			missing++
			if missing <= 5 {
				t.Errorf("%s: %s missing", label, p)
			}
		case g != sum:
			wrong++
			if wrong <= 5 {
				t.Errorf("%s: %s content differs", label, p)
			}
		default:
			checked++
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("%s: unexpected restored file %s", label, p)
		}
	}
	t.Logf("%s: %d files byte-identical, %d missing, %d different", label, checked, missing, wrong)
	if checked == 0 {
		t.Errorf("%s: nothing was verified", label)
	}
}

func TestPXARv2OfficialInterop(t *testing.T) {
	if os.Getenv("PBS_PXARV2") == "" {
		t.Skip("set PBS_PXARV2=1")
	}
	base := t.TempDir()

	for _, s := range pxarv2Snapshots {
		want := pxarv2Expected(t, s.label)

		// Browse: every expected file must be listed with its size.
		entries, err := ListSnapshotContentsInline(pxarv2Opts(s), "", true)
		if err != nil {
			t.Errorf("%s: list: %v", s.label, err)
			continue
		}
		listed := map[string]string{}
		for _, e := range entries {
			p := filepath.ToSlash(e.Path)
			if i := strings.Index(p, "/"); i >= 0 {
				listed[p[i+1:]] = p
			}
		}
		notListed := 0
		for p := range want {
			if _, ok := listed[p]; !ok && !pxarv2KnownSkipped[p] {
				notListed++
				if notListed <= 3 {
					t.Errorf("%s: %s not in listing", s.label, p)
				}
			}
		}
		t.Logf("%s: listing has %d entries", s.label, len(entries))

		// Full restore, sequential and parallel.
		for _, parallel := range []bool{false, true} {
			dest := filepath.Join(base, s.label+map[bool]string{false: "-seq", true: "-par"}[parallel])
			os.MkdirAll(dest, 0755)
			err := RestoreSnapshotInline(func() RestoreOptions {
				o := pxarv2Opts(s)
				o.DestPath, o.Mode, o.Overwrite, o.ParallelExtraction = dest, RestoreModeAlternateAbs, true, parallel
				return o
			}())
			if err != nil {
				t.Errorf("%s parallel=%v: restore: %v", s.label, parallel, err)
				continue
			}
			pxarv2Compare(t, s.label+map[bool]string{false: " full", true: " full parallel"}[parallel], want, pxarv2Restored(t, dest), nil)
		}

		// Partial restores: one file, one nested file and one directory. In a
		// split archive a file's GOODBYE span overstates its size, so these
		// also check that nothing beyond the selection is restored.
		for _, sel := range []string{"a/hello.txt", "a/b/c/deep.txt", "small"} {
			inc, ok := listed[sel]
			if !ok {
				t.Errorf("%s: %s not in listing for partial restore", s.label, sel)
				continue
			}
			dest := filepath.Join(base, s.label+"-partial-"+strings.ReplaceAll(sel, "/", "_"))
			os.MkdirAll(dest, 0755)
			err := RestoreSnapshotInline(func() RestoreOptions {
				o := pxarv2Opts(s)
				o.DestPath, o.Mode, o.Overwrite, o.IncludePaths = dest, RestoreModeAlternateAbs, true, []string{inc}
				return o
			}())
			if err != nil {
				t.Errorf("%s partial %s: %v", s.label, sel, err)
				continue
			}
			// The restored tree has no a/hello.txt for these, so collect by
			// the selection's own relative path below dest.
			got := map[string]string{}
			filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel := filepath.ToSlash(p)
				for wp := range want {
					if strings.HasSuffix(rel, "/"+wp) {
						f, _ := os.Open(p)
						h := sha256.New()
						io.Copy(h, f)
						f.Close()
						got[wp] = hex.EncodeToString(h.Sum(nil))
						return nil
					}
				}
				got["UNEXPECTED:"+rel] = ""
				return nil
			})
			inSel := func(p string) bool { return p == sel || strings.HasPrefix(p, sel+"/") }
			for p := range got {
				if !inSel(p) {
					t.Errorf("%s partial %s: restored something outside the selection: %s", s.label, sel, p)
				}
			}
			pxarv2Compare(t, s.label+" partial "+sel, want, filterKeys(got, inSel), inSel)
		}
	}
}

func filterKeys(m map[string]string, keep func(string) bool) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if keep(k) {
			out[k] = v
		}
	}
	return out
}
