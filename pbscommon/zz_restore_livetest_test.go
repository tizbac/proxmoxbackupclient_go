package pbscommon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// Live check that this client restores what proxmox-backup-client wrote, against
// a real PBS: the reverse of every other test of the end-to-end suite, which
// back up with this client and restore with the official one. It is the proof
// that compressed encrypted chunks and blobs (ENCR_COMPR), which the official
// client writes by default, are read by the folder, disk image and blob paths.
// Opt-in: PBS_RESTORE_CASES names a JSON file with
//
//	{"url": "https://pbs:8007", "fingerprint": "...", "datastore": "store", "namespace": "",
//	 "authid": "user@pbs!tok", "secret": "...", "keyfile": "/path/key.pem",
//	 "cases": [{"name": "...", "type": "host", "id": "...", "time": 1700000000,
//	            "kind": "image" | "blob" | "pxar", "archive": "disk.img.fidx",
//	            "payload": "", "sha256": "...", "dest": "/folder/to/extract/into"}]}
//
// image: the archive, read end to end, has the SHA-256 given. blob: so has the
// decoded blob. pxar: the archive is extracted into dest, for the caller to
// compare with the source; with "payload" it is a split archive, archive its
// .mpxar.didx and payload its .ppxar.didx. keyfile is a key with no passphrase,
// "" for unencrypted snapshots. testing/e2e/run.sh writes the file and runs this.
func TestLiveRestoreOfficial(t *testing.T) {
	path := os.Getenv("PBS_RESTORE_CASES")
	if path == "" {
		t.Skip("PBS_RESTORE_CASES not set")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		URL         string `json:"url"`
		Fingerprint string `json:"fingerprint"`
		Datastore   string `json:"datastore"`
		Namespace   string `json:"namespace"`
		AuthID      string `json:"authid"`
		Secret      string `json:"secret"`
		Keyfile     string `json:"keyfile"`
		Cases       []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			ID      string `json:"id"`
			Time    int64  `json:"time"`
			Kind    string `json:"kind"`
			Archive string `json:"archive"`
			Payload string `json:"payload"`
			SHA256  string `json:"sha256"`
			Dest    string `json:"dest"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Cases) == 0 {
		t.Fatal("no cases in " + path)
	}
	var crypt *CryptConfig
	if cfg.Keyfile != "" {
		if crypt, err = LoadKeyFile(cfg.Keyfile, nil); err != nil {
			t.Fatalf("key file: %v", err)
		}
	}
	hashAll := func(ra io.ReaderAt, size int64) (string, error) {
		h := sha256.New()
		if _, err := io.Copy(h, io.NewSectionReader(ra, 0, size)); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	for _, c := range cfg.Cases {
		t.Run(c.Name, func(t *testing.T) {
			pbs := &PBSClient{BaseURL: cfg.URL, CertFingerPrint: cfg.Fingerprint, Datastore: cfg.Datastore,
				Namespace: cfg.Namespace, AuthID: cfg.AuthID, Secret: cfg.Secret, Crypt: crypt,
				Manifest: BackupManifest{BackupID: c.ID, BackupTime: c.Time}}
			pbs.Connect(true, c.Type)
			defer pbs.Close()
			// The official client's manifest names its key and no writer:
			// neither check may refuse it.
			if err := pbs.CheckArchiveKey(c.Archive); err != nil {
				t.Fatalf("CheckArchiveKey: %v", err)
			}
			switch c.Kind {
			case "image":
				ra, size, err := pbs.NewFIDXReaderAt(c.Archive, 8, nil)
				if err != nil {
					t.Fatalf("open %s: %v", c.Archive, err)
				}
				got, err := hashAll(ra, size)
				if err != nil {
					t.Fatalf("read %s: %v", c.Archive, err)
				}
				if got != c.SHA256 {
					t.Fatalf("%s: %d bytes with sha256 %s, want %s", c.Archive, size, got, c.SHA256)
				}
				t.Logf("%s: %d bytes, sha256 as the source", c.Archive, size)
			case "blob":
				data, err := pbs.DownloadBlob(c.Archive)
				if err != nil {
					t.Fatalf("read %s: %v", c.Archive, err)
				}
				sum := sha256.Sum256(data)
				if got := hex.EncodeToString(sum[:]); got != c.SHA256 {
					t.Fatalf("%s: %d bytes with sha256 %s, want %s", c.Archive, len(data), got, c.SHA256)
				}
				t.Logf("%s: %d bytes, sha256 as the source", c.Archive, len(data))
			case "pxar":
				ra, size, err := pbs.NewDIDXReaderAt(c.Archive, 8, nil)
				if err != nil {
					t.Fatalf("open %s: %v", c.Archive, err)
				}
				pr := NewPXARReaderAt(ra, size)
				if c.Payload != "" {
					pra, psize, err := pbs.NewDIDXReaderAt(c.Payload, 8, nil)
					if err != nil {
						t.Fatalf("open %s: %v", c.Payload, err)
					}
					if pr, err = NewSplitPXARReaderAt(ra, size, pra, psize); err != nil {
						t.Fatalf("split archive %s + %s: %v", c.Archive, c.Payload, err)
					}
				}
				files, err := pr.ExtractAll(c.Dest)
				if err != nil {
					t.Fatalf("extract %s: %v", c.Archive, err)
				}
				written := 0
				for _, f := range files {
					if f.Skipped {
						t.Errorf("%s: skipped %s: %s", c.Archive, f.ArchivePath, f.SkipReason)
						continue
					}
					written++
				}
				if written == 0 {
					t.Fatalf("%s: nothing was extracted", c.Archive)
				}
				t.Logf("%s: %d entries extracted into %s", c.Archive, written, c.Dest)
			default:
				t.Fatalf("unknown kind %q", c.Kind)
			}
		})
	}
}
