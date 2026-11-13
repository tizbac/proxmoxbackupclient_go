package clientcommon

import (
	"fmt"
	"os"

	"pbscommon"
)

// CompressEncryptedFlagHelp is the usage text of -compress-encrypted, shared by
// the backup tools.
const CompressEncryptedFlagHelp = "With -keyfile: compress chunks before they are encrypted, as proxmox-backup-client does. Such backups need version 2.1 or later to restore. Without it encrypted chunks are written uncompressed, as version 2.0 and earlier wrote them and the only form those versions (and restore ISOs made with them) can restore"

// BackupCrypt sets how a backup run of the command-line tools writes with
// crypt (nil, no encryption, stays nil): compressed before encryption only
// when asked for (-compress-encrypted or the config file's
// "compress-encrypted"), since a script that ran with version 2.0 must keep
// writing what version 2.0 and its restore ISOs can read until its owner
// says otherwise. Without it the run says so once, on standard output.
func BackupCrypt(crypt *pbscommon.CryptConfig, compress bool) *pbscommon.CryptConfig {
	if crypt == nil {
		return nil
	}
	if !compress {
		fmt.Println("Encrypted chunks are written uncompressed, so version 2.0 and earlier can restore them. Add -compress-encrypted to compress them (restoring then needs version 2.1 or later).")
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
