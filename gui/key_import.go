//go:build !service

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"pbscommon"
	"security"
)

// Importing a key brings back a key the user kept somewhere else: a copy of
// the key file (on a USB stick, say), or a paper key, read from its QR code
// (with the camera or from a picture) or typed in from its text. Whatever the
// source, the key is checked before anything is written (a key without a
// passphrase must compute to its own fingerprint, a protected one must unlock
// with its passphrase, which catches a mistyped character either way), shown
// with its fingerprint to compare with the paper, then saved as a new file in
// this computer's keys folder. An existing file is never overwritten.

// KeyImport is a key read for import, before it is saved: what the import
// dialog shows to confirm. It carries the key file's text, as the paper key
// does, because the text is what gets saved.
type KeyImport struct {
	// KeyText is the key file as it will be written. Empty when nothing was
	// read: the user cancelled, or no QR code was found in a camera frame.
	KeyText             string `json:"key_text,omitempty"`
	Fingerprint         string `json:"fingerprint,omitempty"`
	Created             string `json:"created,omitempty"`
	Hint                string `json:"hint,omitempty"`
	PassphraseProtected bool   `json:"passphrase_protected"`
	// Checked: the key data was unlocked and computes to Fingerprint. Always
	// done for a key without a passphrase that names its fingerprint; for a
	// protected key only once its passphrase is given (VerifyImportedKey).
	Checked bool `json:"checked"`
	// SourcePath is the file the key was read from (key file or picture).
	SourcePath string `json:"source_path,omitempty"`
	// Problem is set instead of an error for what the user can put right,
	// so the dialog can say it in their language: "no_qr" (no readable QR
	// code in the picture), "not_key" (a QR code or text that is not a key
	// file), "damaged" (a key file with a character wrong or missing: it
	// does not compute to its fingerprint), "unsupported_image" (a picture
	// format that cannot be read), "wrong_passphrase" (or a mistake in a
	// protected key, which looks the same). Detail has the technical reason.
	Problem string `json:"problem,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

const (
	keyImportMaxFile    = 64 << 10 // a key file is under 1 KiB
	keyImportMaxPicture = 64 << 20 // a phone photo is a few MB
	keyImportMaxPixels  = 50 << 20 // ~50 megapixels: larger is refused before decoding
	keyImportMaxFrame   = 4096     // a camera frame's longer side, px
	keyBeginMarker      = "-----BEGIN PROXMOX BACKUP KEY-----"
	keyEndMarker        = "-----END PROXMOX BACKUP KEY-----"
)

// errNotKey is wrapped by parseKeyText for text that is not a key file.
var errNotKey = errors.New("not a key file")

// parseKeyText reads a key file's text as it comes from a file, a QR code or
// the keyboard. It accepts the official paper key's layout too (the JSON
// between BEGIN/END PROXMOX BACKUP KEY lines), and what typing from paper
// tends to add: curly quotes, spaces inside the base64 or the fingerprint.
// It returns the text to save (the input as it was when it needed no
// tidying, so a file or QR code is saved byte for byte) and the parsed key.
func parseKeyText(s string) (string, *pbscommon.KeyConfig, error) {
	s = strings.TrimSpace(string(stripUTF8BOM([]byte(s))))
	if i := strings.Index(s, keyBeginMarker); i >= 0 {
		rest := s[i+len(keyBeginMarker):]
		j := strings.Index(rest, keyEndMarker)
		if j < 0 {
			return "", nil, fmt.Errorf("%w: the text has a BEGIN PROXMOX BACKUP KEY line but no END line", errNotKey)
		}
		s = strings.TrimSpace(rest[:j])
	}
	if s == "" {
		return "", nil, fmt.Errorf("%w: there is no text", errNotKey)
	}
	if len(s) > keyImportMaxFile {
		return "", nil, fmt.Errorf("%w: the text is far longer than a key file", errNotKey)
	}
	// Text that is valid as it stands (a key file, a QR code) is read as it
	// is; only otherwise is it tidied of what typing adds (curly quotes,
	// non-breaking spaces), which would also change those inside a hint.
	text := strings.ReplaceAll(s, "\r\n", "\n")
	cfg := &pbscommon.KeyConfig{}
	if err := json.Unmarshal([]byte(text), cfg); err != nil {
		text = strings.NewReplacer("\u201c", `"`, "\u201d", `"`, "\u201e", `"`, "\u00ab", `"`, "\u00bb", `"`, "\u00a0", " ").Replace(text)
		if !strings.HasPrefix(text, "{") {
			return "", nil, fmt.Errorf("%w: a key file starts with {", errNotKey)
		}
		cfg = &pbscommon.KeyConfig{}
		if err := json.Unmarshal([]byte(text), cfg); err != nil {
			return "", nil, fmt.Errorf("%w: %v", errNotKey, err)
		}
	}
	data := strings.Join(strings.Fields(cfg.Data), "")
	fp := strings.ToLower(strings.Join(strings.Fields(cfg.Fingerprint), ""))
	if len(fp) == 64 && !strings.Contains(fp, ":") {
		// Typed without the colons.
		parts := make([]string, 32)
		for i := range parts {
			parts[i] = fp[2*i : 2*i+2]
		}
		fp = strings.Join(parts, ":")
	}
	if data == "" {
		return "", nil, fmt.Errorf("%w: it has no key data", errNotKey)
	}
	// Strict: the last character before "=" has spare bits that a lenient
	// decoder ignores, so a typo there would decode to the same key and be
	// saved as text the official client refuses.
	raw, err := base64.StdEncoding.Strict().DecodeString(data)
	if err != nil {
		return "", nil, fmt.Errorf("the key data is not valid base64 (a character is wrong or missing): %v", err)
	}
	if cfg.KDF == nil && len(raw) != 32 {
		return "", nil, fmt.Errorf("the key data is %d bytes long instead of 32 (a character is wrong or missing)", len(raw))
	}
	if fp != "" {
		if _, err := pbscommon.ParseFingerprint(fp); err != nil {
			return "", nil, fmt.Errorf("the fingerprint is not valid (a character is wrong or missing): %v", err)
		}
	}
	salt := ""
	if k := cfg.KDF; k != nil {
		salt = strings.Join(strings.Fields(k.Salt), "")
		if _, err := base64.StdEncoding.Strict().DecodeString(salt); err != nil {
			return "", nil, fmt.Errorf("the passphrase salt is not valid base64 (a character is wrong or missing): %v", err)
		}
		// proxmox-backup-client uses scrypt N=65536, r=8, p=1. Far larger
		// values (from a damaged or crafted key) would take the memory or
		// time of the computer when the passphrase is tried.
		if k.N > 1<<20 || k.R > 32 || k.P > 16 || k.Iter > 10_000_000 {
			return "", nil, fmt.Errorf("the key's passphrase settings (n=%d r=%d p=%d iter=%d) are far beyond what proxmox-backup-client uses", k.N, k.R, k.P, k.Iter)
		}
	}
	out := text
	if data != cfg.Data || fp != cfg.Fingerprint || (cfg.KDF != nil && salt != cfg.KDF.Salt) {
		// Tidied: write it out again, in the layout of a key file.
		cfg.Data, cfg.Fingerprint = data, fp
		if cfg.KDF != nil {
			cfg.KDF.Salt = salt
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return "", nil, err
		}
		out = string(b)
	}
	return out + "\n", cfg, nil
}

// keyImportFromText parses text into a KeyImport, checking a key without a
// passphrase straight away. Text that is not a key file, or a key with a
// mistake in it, comes back as a Problem, not an error.
func keyImportFromText(text string) (KeyImport, error) {
	out, cfg, err := parseKeyText(text)
	if err != nil {
		if errors.Is(err, errNotKey) {
			return KeyImport{Problem: "not_key", Detail: err.Error()}, nil
		}
		return KeyImport{Problem: "damaged", Detail: err.Error()}, nil
	}
	ki := KeyImport{
		KeyText:             out,
		Fingerprint:         cfg.Fingerprint,
		Created:             cfg.Created,
		Hint:                cfg.Hint,
		PassphraseProtected: cfg.KDF != nil,
	}
	if cfg.KDF == nil {
		crypt, err := cfg.CryptConfig(nil)
		if err != nil {
			return KeyImport{Problem: "damaged", Detail: err.Error()}, nil
		}
		// A key file without a fingerprint cannot be checked for typing
		// mistakes: show the one it computes to, unchecked.
		ki.Checked = cfg.Fingerprint != ""
		ki.Fingerprint = crypt.Fingerprint()
	}
	return ki, nil
}

// readKeyImportFile reads a key file chosen for import.
func readKeyImportFile(path string) (KeyImport, error) {
	if err := security.ValidatePath(path); err != nil {
		return KeyImport{}, fmt.Errorf("invalid path: %w", err)
	}
	raw, err := readLimited(path, keyImportMaxFile)
	if err != nil {
		return KeyImport{}, err
	}
	ki, err := keyImportFromText(string(raw))
	ki.SourcePath = path
	return ki, err
}

// readLimited reads a file of at most max bytes.
func readLimited(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if int64(len(raw)) > max {
		limit := fmt.Sprintf("%d KiB", max>>10)
		if max >= 1<<20 {
			limit = fmt.Sprintf("%d MB", max>>20)
		}
		return nil, fmt.Errorf("%s is too large (over %s) to be what was expected", path, limit)
	}
	return raw, nil
}

// pictureSizes are the sizes (longer side, px) a picture is also tried at
// before its own: a phone photo is far larger than a QR code needs, and a
// smaller copy is faster and often reads better. A camera frame is small
// already and is tried as it is (nil).
var pictureSizes = []int{1600, 1000}

// decodeKeyQR finds a QR code in img and returns its text. It tries the
// picture at each of sizes that is clearly smaller than it, then as it is
// (or, for a very large picture, at 3200 px: the full size would take
// seconds and hundreds of MB for no gain), each with two ways of telling
// black from white.
func decodeKeyQR(img image.Image, sizes []int) (string, bool) {
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	try := func(im image.Image) (string, bool) {
		src := gozxing.NewLuminanceSourceFromImage(im)
		for _, bin := range []gozxing.Binarizer{gozxing.NewHybridBinarizer(src), gozxing.NewGlobalHistgramBinarizer(src)} {
			bmp, err := gozxing.NewBinaryBitmap(bin)
			if err != nil {
				continue
			}
			if res, err := qrcode.NewQRCodeReader().Decode(bmp, hints); err == nil {
				return res.GetText(), true
			}
		}
		return "", false
	}
	b := img.Bounds()
	long := b.Dx()
	if b.Dy() > long {
		long = b.Dy()
	}
	for _, size := range sizes {
		if long > size*5/4 {
			if text, ok := try(scaleTo(img, size)); ok {
				return text, true
			}
		}
	}
	if long > 4000 {
		return try(scaleTo(img, 3200))
	}
	return try(img)
}

// scaleTo returns img scaled so that its longer side is size pixels.
func scaleTo(img image.Image, size int) image.Image {
	b := img.Bounds()
	w, h := size, b.Dy()*size/b.Dx()
	if b.Dy() > b.Dx() {
		w, h = b.Dx()*size/b.Dy(), size
	}
	dst := image.NewGray(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// keyImportFromImageBytes decodes a picture and reads the key from its QR
// code. A picture that cannot be read or has no QR code is a Problem.
func keyImportFromImageBytes(raw []byte) (KeyImport, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		detail := err.Error()
		if isHEIC(raw) {
			detail = "HEIC/HEIF picture (the iPhone camera's format)"
		}
		return KeyImport{Problem: "unsupported_image", Detail: detail}, nil
	}
	if int64(cfg.Width)*int64(cfg.Height) > keyImportMaxPixels {
		return KeyImport{Problem: "unsupported_image", Detail: fmt.Sprintf("the picture is %dx%d pixels, too large to read", cfg.Width, cfg.Height)}, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return KeyImport{Problem: "unsupported_image", Detail: fmt.Sprintf("%s: %v", format, err)}, nil
	}
	text, ok := decodeKeyQR(img, pictureSizes)
	if !ok {
		return KeyImport{Problem: "no_qr", Detail: fmt.Sprintf("no readable QR code in the %dx%d %s picture", cfg.Width, cfg.Height, format)}, nil
	}
	return keyImportFromText(text)
}

// isHEIC reports whether raw starts like a HEIC/HEIF file (an ISO media
// file of brand heic, heix, mif1 and the like).
func isHEIC(raw []byte) bool {
	if len(raw) < 12 || string(raw[4:8]) != "ftyp" {
		return false
	}
	switch string(raw[8:12]) {
	case "heic", "heix", "hevc", "heim", "heis", "mif1", "msf1", "avif":
		return true
	}
	return false
}

// keysFolder is where imported (and generated) keys are saved.
func keysFolder() (string, error) {
	p, err := DefaultEncryptionKeyPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// sameKeyInFolder returns a key file in dir that holds exactly the key in
// cfg (whose fingerprint, checked, is fp): the same key data, protected the
// same way with the same salt, and naming that fingerprint. A file merely
// naming the fingerprint is not enough: a protected file's fingerprint is
// not checked without its passphrase, and the user may be importing an
// unprotected copy because a protected one (say, with a forgotten
// passphrase) cannot be used.
func sameKeyInFolder(dir string, cfg *pbscommon.KeyConfig, fp string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	norm := func(s string) string { return strings.Join(strings.Fields(s), "") }
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		have, err := pbscommon.LoadKeyConfig(p)
		if err != nil || norm(have.Data) != cfg.Data || strings.ToLower(have.Fingerprint) != fp {
			continue
		}
		switch {
		case have.KDF == nil && cfg.KDF == nil:
		case have.KDF != nil && cfg.KDF != nil:
			k := *have.KDF
			k.Salt = norm(k.Salt)
			if k != *cfg.KDF {
				continue
			}
		default:
			continue
		}
		return p
	}
	return ""
}

// saveImportedKey checks the key in text (with passphrase, for a protected
// key) and saves it as a new file in dir, named after its fingerprint like a
// saved copy. A key already in dir is not saved again: that file is
// returned, with existed true. Nothing is ever overwritten.
func saveImportedKey(dir, text, passphrase string) (path string, existed bool, err error) {
	out, cfg, err := parseKeyText(text)
	if err != nil {
		return "", false, err
	}
	crypt, err := cfg.CryptConfig([]byte(passphrase))
	if err != nil {
		if cfg.KDF != nil && !isFingerprintMismatch(err) {
			return "", false, fmt.Errorf("the passphrase does not unlock this key: %w", err)
		}
		return "", false, fmt.Errorf("the key cannot be read: %w", err)
	}
	fp := crypt.Fingerprint()
	if cfg.Fingerprint == "" {
		// Name it like every other key file does, so it can be checked later.
		cfg.Fingerprint = fp
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return "", false, err
		}
		out = string(b) + "\n"
	}

	if existing := sameKeyInFolder(dir, cfg, fp); existing != "" {
		return existing, true, nil
	}
	if st, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("folder %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", false, fmt.Errorf("creating the folder %s: %w", dir, err)
		}
		if err := restrictKeyAccess(dir, true); err != nil {
			writeDebugLog(fmt.Sprintf("saveImportedKey: could not restrict access to %s: %v", dir, err))
		}
	} else if !st.IsDir() {
		return "", false, fmt.Errorf("%s is not a folder", dir)
	}

	base := "encryption-key-" + strings.ReplaceAll(fp, ":", "")[:8]
	for n := 1; n < 100; n++ {
		name := base + ".json"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.json", base, n)
		}
		path = filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", false, fmt.Errorf("creating %s: %w", path, err)
		}
		_, werr := f.WriteString(out)
		if serr := f.Sync(); werr == nil {
			werr = serr
		}
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(path)
			return "", false, fmt.Errorf("writing %s: %w", path, werr)
		}
		// Best effort, as for a generated key: a FAT/exFAT folder has no
		// permissions to set.
		if err := restrictKeyAccess(path, false); err != nil {
			writeDebugLog(fmt.Sprintf("saveImportedKey: could not restrict access to %s: %v", path, err))
		}
		if back := inspectKeyFile(path); !back.Usable || back.Fingerprint != fp {
			os.Remove(path)
			return "", false, fmt.Errorf("the key was written to %s but did not read back as the same key (%s)", path, back.Reason)
		}
		return path, false, nil
	}
	return "", false, fmt.Errorf("could not find a free file name for the key in %s", dir)
}

// ReadImportKeyText reads a key typed or pasted into the import dialog.
func (a *App) ReadImportKeyText(text string) (KeyImport, error) {
	return keyImportFromText(text)
}

// ReadImportKeyFile asks for a key file and reads it for import. An empty
// KeyImport (no key text, no problem) means the dialog was cancelled.
func (a *App) ReadImportKeyFile() (KeyImport, error) {
	path, err := a.OpenEncryptionKeyDialog()
	if err != nil || path == "" {
		return KeyImport{}, err
	}
	return readKeyImportFile(path)
}

// ChooseImportKeyPicture asks for a scan or photo of a paper key and returns
// its path, or "" when the dialog was cancelled. ReadImportKeyPicture then
// reads it, so the import dialog can say it is reading only once there is
// something to read.
func (a *App) ChooseImportKeyPicture() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("the window is not ready yet")
	}
	defaultDir, err := os.UserHomeDir()
	if err != nil || defaultDir == "" {
		defaultDir = os.TempDir()
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose a scan or photo of the paper key",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{DisplayName: "Pictures (*.jpg, *.png, ...)", Pattern: "*.jpg;*.jpeg;*.png;*.gif;*.bmp;*.tif;*.tiff;*.webp;*.heic;*.heif"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
	return path, err
}

// ReadImportKeyPicture reads the key from the QR code in a scan or photo of
// a paper key (see ChooseImportKeyPicture).
func (a *App) ReadImportKeyPicture(path string) (KeyImport, error) {
	if err := security.ValidatePath(path); err != nil {
		return KeyImport{}, fmt.Errorf("invalid path: %w", err)
	}
	raw, err := readLimited(path, keyImportMaxPicture)
	if err != nil {
		return KeyImport{}, err
	}
	ki, err := keyImportFromImageBytes(raw)
	ki.SourcePath = path
	writeDebugLog(fmt.Sprintf("ReadImportKeyPicture: %s: found=%v problem=%q", path, ki.KeyText != "", ki.Problem))
	return ki, err
}

// ReadImportKeyCameraFrame looks for the key's QR code in one camera frame
// (a JPEG or PNG, base64, from the import dialog's camera view). No QR code
// in the frame is not a problem: the dialog sends the next frame.
func (a *App) ReadImportKeyCameraFrame(frame string) (KeyImport, error) {
	if i := strings.Index(frame, ","); i >= 0 && strings.HasPrefix(frame, "data:") {
		frame = frame[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(frame)
	if err != nil {
		return KeyImport{}, fmt.Errorf("bad camera frame: %w", err)
	}
	if len(raw) > 8<<20 {
		return KeyImport{}, fmt.Errorf("camera frame too large")
	}
	if c, _, err := image.DecodeConfig(bytes.NewReader(raw)); err != nil {
		return KeyImport{}, fmt.Errorf("bad camera frame: %w", err)
	} else if c.Width > keyImportMaxFrame || c.Height > keyImportMaxFrame {
		return KeyImport{}, fmt.Errorf("camera frame too large (%dx%d)", c.Width, c.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return KeyImport{}, fmt.Errorf("bad camera frame: %w", err)
	}
	text, ok := decodeKeyQR(img, nil)
	if !ok {
		return KeyImport{}, nil
	}
	return keyImportFromText(text)
}

// The camera is allowed (on Linux, see camera_linux.go) from PrepareCamera
// to CameraClosed, while the import dialog is open. Each PrepareCamera gets
// a number and camOpen holds the latest, so a PrepareCamera still waiting for
// the window when its dialog closed, or a close that arrives after the next
// dialog opened, changes nothing it should not. Tests swap the two funcs.
var (
	camMu         sync.Mutex
	camLast       int
	camOpen       int // the open dialog's number; 0: none
	cameraPrepare = prepareCamera
	cameraAllow   = allowCamera
)

// PrepareCamera is called by the import dialog when it opens, before it
// looks for or asks for the camera: on Linux the app's web view must be
// told to allow it. It returns the number to give CameraClosed when the
// dialog closes. A failure is only logged: the dialog then finds no camera.
func (a *App) PrepareCamera() int {
	camMu.Lock()
	camLast++
	id := camLast
	camOpen = id
	cameraAllow(false) // until this dialog's window is ready
	camMu.Unlock()
	if err := cameraPrepare(); err != nil {
		writeDebugLog(fmt.Sprintf("PrepareCamera: %v", err))
		return id
	}
	camMu.Lock()
	defer camMu.Unlock()
	if camOpen == id {
		cameraAllow(true)
	}
	return id
}

// CameraClosed is called by the import dialog when it closes, with the
// number PrepareCamera gave it.
func (a *App) CameraClosed(id int) {
	camMu.Lock()
	defer camMu.Unlock()
	if camOpen == id {
		camOpen = 0
		cameraAllow(false)
	}
}

// isFingerprintMismatch reports whether err is KeyConfig.CryptConfig's
// "key fingerprint mismatch": the key unlocked but does not compute to the
// fingerprint its file names, a mistake in one or the other.
func isFingerprintMismatch(err error) bool {
	return err != nil && strings.Contains(err.Error(), "key fingerprint mismatch")
}

// VerifyImportedKey checks a passphrase-protected key read for import: the
// passphrase must unlock it, which also proves its text has no mistake in
// it. The passphrase is not stored.
func (a *App) VerifyImportedKey(text, passphrase string) (KeyImport, error) {
	ki, err := keyImportFromText(text)
	if err != nil || ki.Problem != "" || !ki.PassphraseProtected {
		return ki, err
	}
	_, cfg, err := parseKeyText(text)
	if err != nil {
		return KeyImport{}, err
	}
	crypt, err := cfg.CryptConfig([]byte(passphrase))
	if err != nil {
		ki.Problem, ki.Detail = "wrong_passphrase", err.Error()
		if isFingerprintMismatch(err) {
			ki.Problem = "damaged"
		}
		return ki, nil
	}
	ki.Checked = true
	ki.Fingerprint = crypt.Fingerprint()
	return ki, nil
}

// SaveImportedKey saves a key read for import into this computer's keys
// folder (see saveImportedKey) and returns the saved file's info.
func (a *App) SaveImportedKey(text, passphrase string) (ImportedKeyResult, error) {
	dir, err := keysFolder()
	if err != nil {
		return ImportedKeyResult{}, err
	}
	path, existed, err := saveImportedKey(dir, text, passphrase)
	if err != nil {
		return ImportedKeyResult{}, err
	}
	info := inspectKeyFile(path)
	writeDebugLog(fmt.Sprintf("SaveImportedKey: %s (fingerprint %s, already there: %v)", path, info.Fingerprint, existed))
	return ImportedKeyResult{Info: info, AlreadyThere: existed}, nil
}

// ImportedKeyResult is SaveImportedKey's answer.
type ImportedKeyResult struct {
	Info EncryptionKeyInfo `json:"info"`
	// AlreadyThere: the keys folder already held this key, so nothing was
	// written and Info is that file.
	AlreadyThere bool `json:"already_there"`
}
