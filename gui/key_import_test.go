//go:build !service

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	goqr "github.com/skip2/go-qrcode"
	"pbscommon"
)

// The official key files made by proxmox-backup-client 3.4.9, with their
// throwaway passphrase (see pbscommon/official_keyfile_test.go).
const (
	officialScryptKey     = "../pbscommon/testdata/official-scrypt.key"
	officialScryptFP      = "be:6c:3e:ae:84:21:9c:ef:a0:89:aa:b2:71:b5:96:7b:91:97:f6:71:6f:1b:ac:b5:ae:e9:cc:d0:d6:2a:b7:97"
	officialKeyPassphrase = "throwaway-passphrase-for-test"
)

// newTestKey writes a new key without a passphrase and returns its file's
// text and fingerprint.
func newTestKey(t *testing.T) (string, string) {
	t.Helper()
	enc, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	crypt, err := pbscommon.NewCryptConfig(enc)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "k.json")
	if err := pbscommon.SaveKeyFile(p, crypt); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), crypt.Fingerprint()
}

// withoutFingerprint is the key file text with its fingerprint left out.
func withoutFingerprint(t *testing.T, text string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "fingerprint")
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// flip changes one character of the key data to another valid base64 one,
// as a typing mistake would.
func flip(text string) string {
	k := strings.Index(text, `"data"`) + len(`"data"`)
	i := k + strings.Index(text[k:], `"`) + 1 + 5 // 6th character of the value
	c := byte('A')
	if text[i] == 'A' {
		c = 'B'
	}
	return text[:i] + string(c) + text[i+1:]
}

func TestKeyImportFromText(t *testing.T) {
	text, fp := newTestKey(t)
	official := "Subject: test\n\n" + keyBeginMarker + "\n" + strings.TrimSpace(text) + "\n" + keyEndMarker + "\n█▀▀▀▀▀█ ▄▄ (QR code art)\n"
	typed := strings.ReplaceAll(text, `"`, "\u201c") // curly quotes everywhere
	typed = strings.Replace(typed, fp, strings.ToUpper(strings.ReplaceAll(fp, ":", ": ")), 1)
	if i := strings.Index(typed, "data\u201c: \u201c"); i >= 0 {
		j := i + len("data\u201c: \u201c") + 10
		typed = typed[:j] + " " + typed[j:j+10] + "  " + typed[j+10:]
	}

	for _, c := range []struct {
		name, in  string
		sameBytes bool
	}{
		{"file as saved", text, true},
		{"BOM and CRLF", "\ufeff" + strings.ReplaceAll(text, "\n", "\r\n"), false},
		{"official paper key layout", official, false},
		{"typed from paper", typed, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			ki, err := keyImportFromText(c.in)
			if err != nil || ki.Problem != "" {
				t.Fatalf("err %v, problem %q (%s)", err, ki.Problem, ki.Detail)
			}
			if ki.Fingerprint != fp || !ki.Checked || ki.PassphraseProtected {
				t.Fatalf("got fingerprint %s checked %v protected %v, want %s checked", ki.Fingerprint, ki.Checked, ki.PassphraseProtected, fp)
			}
			if c.sameBytes && ki.KeyText != text {
				t.Fatalf("a key file read as it is must be saved byte for byte:\n%q\n%q", ki.KeyText, text)
			}
			if _, cfg, err := parseKeyText(ki.KeyText); err != nil || cfg.Fingerprint != fp {
				t.Fatalf("the text to save does not read back: %v", err)
			}
		})
	}

	hex := "0123456789abcdef"
	badFP := strings.Replace(text, fp, fp[:len(fp)-1]+string(hex[(strings.IndexByte(hex, fp[len(fp)-1])+1)%16]), 1)
	i := strings.Index(text, `"data": "`) + 15
	for _, c := range []struct{ name, in string }{
		{"one character wrong in the data", flip(text)},
		{"one character wrong in the fingerprint", badFP},
		{"a character missing", text[:i] + text[i+1:]},
	} {
		t.Run(c.name, func(t *testing.T) {
			ki, err := keyImportFromText(c.in)
			if err != nil || ki.Problem != "damaged" || ki.KeyText != "" {
				t.Fatalf("a mistyped key must come back as damaged: err %v %+v", err, ki)
			}
		})
	}
	t.Run("no fingerprint in the file", func(t *testing.T) {
		noFP := withoutFingerprint(t, text)
		ki, err := keyImportFromText(noFP)
		if err != nil || ki.Problem != "" {
			t.Fatalf("err %v problem %q %s\n%s", err, ki.Problem, ki.Detail, noFP)
		}
		if ki.Checked || ki.Fingerprint != fp {
			t.Fatalf("without a fingerprint to compare, the key is unchecked but shows the one it computes to: %+v", ki)
		}
	})
	for _, c := range []struct{ name, in string }{
		{"empty", "   "},
		{"not JSON", "hello world"},
		{"JSON but no key", `{"name": "x"}`},
		{"BEGIN without END", keyBeginMarker + "\n" + text},
		{"URL from some other QR code", "https://example.com/"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ki, err := keyImportFromText(c.in)
			if err != nil || ki.Problem != "not_key" || ki.KeyText != "" {
				t.Fatalf("want problem not_key, got err %v %+v", err, ki)
			}
		})
	}
}

// setField returns the key file text with one top-level field set (or
// removed, for nil), written out again.
func setField(t *testing.T, text, field string, v any) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatal(err)
	}
	if v == nil {
		delete(m, field)
	} else {
		m[field] = v
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// Found by review: what a lenient reader lets through, or wrongly refuses.
func TestKeyImportEdgeCases(t *testing.T) {
	text, fp := newTestKey(t)
	var m map[string]any
	json.Unmarshal([]byte(text), &m)
	data := m["data"].(string)

	t.Run("spare bits of the last data character", func(t *testing.T) {
		// 32 bytes leave 2 unused bits in the last character before "=":
		// a lenient decoder reads the same key from a mistyped one.
		const b64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
		i := len(data) - 2
		bad := data[:i] + string(b64[strings.IndexByte(b64, data[i])^1]) + data[i+1:]
		if raw, err := base64.StdEncoding.DecodeString(bad); err != nil || base64.StdEncoding.EncodeToString(raw) != data {
			t.Fatalf("test setup: %q should decode leniently to the same key", bad)
		}
		ki, err := keyImportFromText(setField(t, text, "data", bad))
		if err != nil || ki.Problem != "damaged" {
			t.Fatalf("a typo in the spare bits must be refused: err %v %+v", err, ki)
		}
	})
	t.Run("curly quotes and a non-breaking space in a real hint", func(t *testing.T) {
		withHint := setField(t, text, "hint", "the \u201cusual\u201d one,\u00a0at home")
		ki, err := keyImportFromText(withHint)
		if err != nil || ki.Problem != "" || ki.KeyText != withHint {
			t.Fatalf("a valid key file must be read and kept as it is: err %v %+v", err, ki)
		}
	})
	t.Run("fingerprint typed without colons", func(t *testing.T) {
		ki, err := keyImportFromText(setField(t, text, "fingerprint", strings.ReplaceAll(fp, ":", "")))
		if err != nil || ki.Problem != "" || !ki.Checked || ki.Fingerprint != fp {
			t.Fatalf("err %v %+v", err, ki)
		}
	})
	t.Run("text far longer than a key", func(t *testing.T) {
		ki, err := keyImportFromText(text + strings.Repeat(" x", keyImportMaxFile))
		if err != nil || ki.Problem != "not_key" {
			t.Fatalf("err %v %+v", err, ki)
		}
	})

	raw, err := os.ReadFile(officialScryptKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("passphrase settings far beyond the official client's", func(t *testing.T) {
		huge := strings.Replace(string(raw), `"n":65536`, `"n":1073741824`, 1)
		ki, err := keyImportFromText(huge)
		if err != nil || ki.Problem != "damaged" {
			t.Fatalf("a key asking scrypt for a TiB must be refused before the passphrase is tried: err %v %+v", err, ki)
		}
	})
	t.Run("protected key with a mistyped fingerprint", func(t *testing.T) {
		bad := strings.Replace(string(raw), officialScryptFP[:5], "00:00", 1)
		ki, err := (&App{}).VerifyImportedKey(bad, officialKeyPassphrase)
		if err != nil || ki.Problem != "damaged" {
			t.Fatalf("the right passphrase with a wrong fingerprint is a mistake in the key, not the passphrase: err %v %+v", err, ki)
		}
	})
	t.Run("a protected file naming the same fingerprint is not the same key", func(t *testing.T) {
		// The user may import an unprotected copy because the protected one
		// cannot be unlocked: a file that only names the fingerprint must
		// not stand in for it.
		dir := t.TempDir()
		decoy := strings.Replace(string(raw), officialScryptFP, fp, 1)
		if err := os.WriteFile(filepath.Join(dir, "encryption-key-old.json"), []byte(decoy), 0o600); err != nil {
			t.Fatal(err)
		}
		p, existed, err := saveImportedKey(dir, text, "")
		if err != nil || existed {
			t.Fatalf("got %q existed %v err %v: the decoy was taken for the key", p, existed, err)
		}
		if got, _ := os.ReadFile(p); string(got) != text {
			t.Fatal("saved text differs")
		}
	})
}

// pngHeader is the start of a PNG of w x h pixels, enough for DecodeConfig.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 17)
	copy(ihdr, "IHDR")
	binary.BigEndian.PutUint32(ihdr[4:], w)
	binary.BigEndian.PutUint32(ihdr[8:], h)
	ihdr[12], ihdr[13] = 8, 6 // 8-bit RGBA
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&b, binary.BigEndian, uint32(13))
	b.Write(ihdr)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(ihdr))
	return b.Bytes()
}

func TestKeyImportSizeLimits(t *testing.T) {
	ki, err := keyImportFromImageBytes(pngHeader(9000, 9000))
	if err != nil || ki.Problem != "unsupported_image" || !strings.Contains(ki.Detail, "too large") {
		t.Fatalf("an 81 MP picture must be refused before it is decoded: err %v %+v", err, ki)
	}
	if _, err := (&App{}).ReadImportKeyCameraFrame(base64.StdEncoding.EncodeToString(pngHeader(12000, 100))); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("a camera frame 12000 px wide must be refused before it is decoded: %v", err)
	}
}

func TestKeyImportProtected(t *testing.T) {
	raw, err := os.ReadFile(officialScryptKey)
	if err != nil {
		t.Fatal(err)
	}
	ki, err := keyImportFromText(string(raw))
	if err != nil || ki.Problem != "" {
		t.Fatalf("err %v problem %q", err, ki.Problem)
	}
	if !ki.PassphraseProtected || ki.Checked || ki.Fingerprint != officialScryptFP {
		t.Fatalf("a protected key is unchecked until its passphrase is given: %+v", ki)
	}
	a := &App{}
	if got, err := a.VerifyImportedKey(ki.KeyText, "wrong passphrase"); err != nil || got.Problem != "wrong_passphrase" || got.Checked {
		t.Fatalf("wrong passphrase: err %v %+v", err, got)
	}
	if got, err := a.VerifyImportedKey(ki.KeyText, officialKeyPassphrase); err != nil || got.Problem != "" || !got.Checked || got.Fingerprint != officialScryptFP {
		t.Fatalf("right passphrase: err %v %+v", err, got)
	}
	// A mistake in the protected data is only found by the passphrase.
	bad := flip(ki.KeyText)
	if got, err := a.VerifyImportedKey(bad, officialKeyPassphrase); err != nil || got.Problem != "wrong_passphrase" {
		t.Fatalf("a mistyped protected key must not unlock: err %v %+v", err, got)
	}

	dir := filepath.Join(t.TempDir(), "keys")
	if _, _, err := saveImportedKey(dir, ki.KeyText, "wrong passphrase"); err == nil {
		t.Fatal("a protected key must not be saved without its passphrase")
	}
	if _, err := os.Stat(dir); err == nil {
		if m, _ := filepath.Glob(filepath.Join(dir, "*")); len(m) != 0 {
			t.Fatalf("nothing may be written for a refused key: %v", m)
		}
	}
	p, existed, err := saveImportedKey(dir, ki.KeyText, officialKeyPassphrase)
	if err != nil || existed {
		t.Fatalf("save: %v existed %v", err, existed)
	}
	saved, _ := os.ReadFile(p)
	if string(saved) != strings.TrimSpace(string(raw))+"\n" {
		t.Fatalf("the official key file must be saved as it was:\n%s\n%s", saved, raw)
	}
	if _, err := pbscommon.LoadKeyFile(p, []byte(officialKeyPassphrase)); err != nil {
		t.Fatalf("saved key does not unlock: %v", err)
	}
}

func TestSaveImportedKey(t *testing.T) {
	text, fp := newTestKey(t)
	dir := filepath.Join(t.TempDir(), "keys")
	want := filepath.Join(dir, "encryption-key-"+strings.ReplaceAll(fp, ":", "")[:8]+".json")

	p, existed, err := saveImportedKey(dir, text, "")
	if err != nil || existed || p != want {
		t.Fatalf("first save: %q existed %v err %v, want %q", p, existed, err, want)
	}
	if got, _ := os.ReadFile(p); string(got) != text {
		t.Fatalf("saved text differs:\n%s\n%s", got, text)
	}
	if info := inspectKeyFile(p); !info.Usable || info.Fingerprint != fp {
		t.Fatalf("saved key not usable: %+v", info)
	}

	// The same key again (from any source) is the file already there.
	p2, existed, err := saveImportedKey(dir, strings.ReplaceAll(text, "\n", "\r\n"), "")
	if err != nil || !existed || p2 != p {
		t.Fatalf("second save: %q existed %v err %v", p2, existed, err)
	}

	// Another file in the way of the name is never overwritten.
	other, otherFP := newTestKey(t)
	name := filepath.Join(dir, "encryption-key-"+strings.ReplaceAll(otherFP, ":", "")[:8]+".json")
	if err := os.WriteFile(name, []byte("not mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	p3, existed, err := saveImportedKey(dir, other, "")
	if err != nil || existed || p3 != strings.TrimSuffix(name, ".json")+"-2.json" {
		t.Fatalf("name taken: %q existed %v err %v", p3, existed, err)
	}
	if got, _ := os.ReadFile(name); string(got) != "not mine" {
		t.Fatal("an existing file was overwritten")
	}

	// A mistyped key is refused and nothing is written.
	before, _ := filepath.Glob(filepath.Join(dir, "*"))
	if _, _, err := saveImportedKey(dir, flip(text), ""); err == nil {
		t.Fatal("a mistyped key was saved")
	}
	if after, _ := filepath.Glob(filepath.Join(dir, "*")); len(after) != len(before) {
		t.Fatalf("files written for a refused key: %v", after)
	}

	// A key file with no fingerprint is saved with the one it computes to.
	k3, fp3 := newTestKey(t)
	noFP := withoutFingerprint(t, k3)
	p4, _, err := saveImportedKey(dir, noFP, "")
	if err != nil {
		t.Fatalf("no fingerprint: %v\n%s", err, noFP)
	}
	if info := inspectKeyFile(p4); info.Fingerprint != fp3 {
		t.Fatalf("saved without its fingerprint: %+v", info)
	}
}

// qrImage is the QR code a paper key prints, as an image.
func qrImage(t *testing.T, text string) image.Image {
	t.Helper()
	b, err := goqr.Encode(text, goqr.Medium, 640)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// photo puts the QR code on a w x h picture the way a photo of the page has
// it: qrSize px wide, turned by angle degrees, on paper on a darker desk,
// lit unevenly, with sensor noise.
func photo(qr image.Image, w, h, qrSize int, angle float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	qb := qr.Bounds()
	cx, cy := float64(w)/2, float64(h)/2
	s, c := math.Sin(angle*math.Pi/180), math.Cos(angle*math.Pi/180)
	qs := float64(qrSize)
	seed := uint32(12345)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			u, v := dx*c+dy*s, -dx*s+dy*c
			lum := 70.0
			switch {
			case math.Abs(u) < qs/2 && math.Abs(v) < qs/2:
				r, _, _, _ := qr.At(qb.Min.X+int((u+qs/2)/qs*float64(qb.Dx())), qb.Min.Y+int((v+qs/2)/qs*float64(qb.Dy()))).RGBA()
				lum = 40
				if r > 0x8000 {
					lum = 225
				}
			case math.Abs(u) < qs*0.8 && math.Abs(v) < qs*1.1:
				lum = 225
			}
			lum *= 0.75 + 0.25*float64(x)/float64(w)
			seed = seed*1664525 + 1013904223
			lum += float64(int(seed>>24)%15 - 7)
			l := uint8(math.Max(0, math.Min(255, lum)))
			img.Set(x, y, color.RGBA{l, l, l - l/16, 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image, q int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestKeyImportFromPicture(t *testing.T) {
	text, fp := newTestKey(t)
	qr := qrImage(t, text)
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, qr); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(officialScryptKey)
	protQR := qrImage(t, string(raw))

	for _, c := range []struct {
		name string
		pic  []byte
		fp   string
	}{
		{"the QR code itself (PNG)", pngBuf.Bytes(), fp},
		{"scan of the page (JPEG)", encodeJPEG(t, photo(qr, 1700, 2200, 820, 0.5), 85), fp},
		{"phone photo, 12 MP, at an angle", encodeJPEG(t, photo(qr, 3000, 4000, 1300, 9), 80), fp},
		{"phone photo, landscape, QR small", encodeJPEG(t, photo(qr, 4000, 3000, 700, -4), 80), fp},
		{"protected key photo", encodeJPEG(t, photo(protQR, 3000, 4000, 1400, 3), 80), officialScryptFP},
	} {
		t.Run(c.name, func(t *testing.T) {
			ki, err := keyImportFromImageBytes(c.pic)
			if err != nil || ki.Problem != "" {
				t.Fatalf("err %v problem %q %s", err, ki.Problem, ki.Detail)
			}
			if ki.Fingerprint != c.fp {
				t.Fatalf("fingerprint %s, want %s", ki.Fingerprint, c.fp)
			}
		})
	}

	blank := encodeJPEG(t, photo(image.NewGray(image.Rect(0, 0, 10, 10)), 1200, 900, 0, 0), 80)
	other := encodeJPEG(t, photo(qrImage(t, "https://example.com/"), 1200, 900, 500, 2), 85)
	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...)
	for _, c := range []struct {
		name, problem string
		pic           []byte
	}{
		{"no QR code", "no_qr", blank},
		{"some other QR code", "not_key", other},
		{"iPhone HEIC", "unsupported_image", heic},
		{"not a picture", "unsupported_image", []byte("hello, this is not a picture")},
	} {
		t.Run(c.name, func(t *testing.T) {
			ki, err := keyImportFromImageBytes(c.pic)
			if err != nil || ki.Problem != c.problem || ki.KeyText != "" {
				t.Fatalf("want problem %s, got err %v %+v", c.problem, err, ki)
			}
		})
	}
	if ki, _ := keyImportFromImageBytes(heic); !strings.Contains(ki.Detail, "HEIC") {
		t.Fatalf("a HEIC picture should say so: %q", ki.Detail)
	}
}

func TestReadImportKeyCameraFrame(t *testing.T) {
	text, fp := newTestKey(t)
	a := &App{}
	frame := encodeJPEG(t, photo(qrImage(t, text), 1280, 720, 380, 6), 85)
	for _, prefix := range []string{"", "data:image/jpeg;base64,"} {
		ki, err := a.ReadImportKeyCameraFrame(prefix + base64.StdEncoding.EncodeToString(frame))
		if err != nil || ki.Fingerprint != fp || !ki.Checked {
			t.Fatalf("prefix %q: err %v %+v", prefix, err, ki)
		}
	}
	// A frame without the QR code yet: nothing, and no error, so the
	// dialog carries on with the next one.
	empty := encodeJPEG(t, photo(image.NewGray(image.Rect(0, 0, 1, 1)), 1280, 720, 0, 0), 85)
	ki, err := a.ReadImportKeyCameraFrame(base64.StdEncoding.EncodeToString(empty))
	if err != nil || ki.KeyText != "" || ki.Problem != "" {
		t.Fatalf("empty frame: err %v %+v", err, ki)
	}
	if _, err := a.ReadImportKeyCameraFrame("!!!"); err == nil {
		t.Fatal("a broken frame must be an error")
	}
}

func TestReadKeyImportFile(t *testing.T) {
	text, fp := newTestKey(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "copy.json")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	ki, err := readKeyImportFile(p)
	if err != nil || ki.Fingerprint != fp || ki.SourcePath != p || ki.KeyText != text {
		t.Fatalf("err %v %+v", err, ki)
	}
	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, bytes.Repeat([]byte(" "), keyImportMaxFile+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readKeyImportFile(big); err == nil {
		t.Fatal("a file far larger than a key must be refused")
	}
}

// The camera may be allowed only while an import dialog is open, whatever
// order the dialog's calls (each on its own goroutine) arrive in.
func TestCameraPermission(t *testing.T) {
	var mu sync.Mutex
	allowed := false
	hold := make(chan struct{})
	waiting := make(chan struct{}, 1)
	slow := false
	oldPrepare, oldAllow := cameraPrepare, cameraAllow
	t.Cleanup(func() { cameraPrepare, cameraAllow = oldPrepare, oldAllow })
	cameraAllow = func(on bool) { mu.Lock(); allowed = on; mu.Unlock() }
	cameraPrepare = func() error {
		if slow {
			waiting <- struct{}{}
			<-hold
		}
		return nil
	}
	is := func(want bool, what string) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if allowed != want {
			t.Fatalf("%s: camera allowed %v, want %v", what, allowed, want)
		}
	}
	a := &App{}

	id := a.PrepareCamera()
	is(true, "dialog open")
	a.CameraClosed(id)
	is(false, "dialog closed")

	// Closed while PrepareCamera still waits for the window.
	slow = true
	done := make(chan int)
	go func() { done <- a.PrepareCamera() }()
	<-waiting
	camMu.Lock()
	pending := camOpen
	camMu.Unlock()
	a.CameraClosed(pending)
	close(hold)
	<-done
	is(false, "closed before the window was ready")
	slow = false

	// A late close of the last dialog after the next one opened.
	first := a.PrepareCamera()
	a.CameraClosed(first)
	second := a.PrepareCamera()
	a.CameraClosed(first)
	is(true, "stale close")
	a.CameraClosed(second)
	is(false, "second dialog closed")
}
