package clientcommon

import (
	"fmt"
	"os"

	"pbscommon"
)

// NoCompressEncryptedFlagHelp is the usage text of -no-compress-encrypted,
// shared by the backup tools.
const NoCompressEncryptedFlagHelp = "With -keyfile: write encrypted chunks uncompressed, as version 2.0 and earlier wrote them and the only form those versions (and restore ISOs made with them) can restore. Without it chunks are compressed before they are encrypted, as proxmox-backup-client does, and the backup needs version 2.1 or later to restore"

// BackupCrypt sets how a backup run of the command-line tools writes with
// crypt (nil, no encryption, stays nil). The tools compress before they
// encrypt, as proxmox-backup-client does, unless told not to
// (-no-compress-encrypted or the config file's "no-compress-encrypted"),
// which writes what version 2.0 and its restore ISOs can read. Either way
// the run says in one line on standard output which form it writes and which
// versions restore it, so a script carried over from version 2.0 shows the
// change in its log.
func BackupCrypt(crypt *pbscommon.CryptConfig, compress bool) *pbscommon.CryptConfig {
	if crypt == nil {
		return nil
	}
	if compress {
		fmt.Println("Encrypted chunks are compressed before they are encrypted: restoring this backup needs version 2.1 or later. Add -no-compress-encrypted to write them as version 2.0 did.")
	} else {
		fmt.Println("Encrypted chunks are written uncompressed (-no-compress-encrypted), so version 2.0 and earlier can restore them.")
	}
	return crypt.WithCompression(compress)
}

// LoadCryptConfig resolves a PBS encryption key file into the CryptConfig the
// backup and restore paths put on pbscommon.PBSClient.
//
// An empty keyPath means "no encryption" and yields (nil, nil), which is the
// caller's signal to keep the plain sha256 / magic+CRC32 framing. A non-empty
// keyPath is loaded with pbscommon.LoadKeyFile.
//
// Passphrase handling mirrors proxmox-backup-client --keyfile: a key file
// written with `--kdf none` carries the raw key and needs nothing, while a
// scrypt/PBKDF2 key file is unwrapped with a passphrase. If one is required
// and passphrase is empty, PBS_ENCRYPTION_PASSWORD is used (the official
// client's variable), and without that the user is prompted (echo-disabled on
// a terminal, plain line otherwise, so scripted runs keep working). Passphrase
// is never logged or echoed back.
func LoadCryptConfig(keyPath, passphrase string) (*pbscommon.CryptConfig, error) {
	if keyPath == "" {
		return nil, nil
	}

	keyCfg, err := pbscommon.LoadKeyConfig(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key file %s: %w", keyPath, err)
	}

	if passphrase == "" && keyCfg.KDF != nil {
		passphrase = os.Getenv("PBS_ENCRYPTION_PASSWORD")
	}
	if passphrase == "" && keyCfg.KDF != nil {
		passphrase, err = PromptPassword(fmt.Sprintf("Passphrase for %s: ", keyPath))
		if err != nil {
			return nil, fmt.Errorf("cannot read key passphrase from console: %w", err)
		}
	}

	crypt, err := keyCfg.CryptConfig([]byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf("unlocking key file %s: %w", keyPath, err)
	}
	return crypt, nil
}
