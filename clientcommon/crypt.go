package clientcommon

import (
	"fmt"

	"pbscommon"
)

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
// and passphrase is empty, the user is prompted (echo-disabled on a terminal,
// plain line otherwise, so scripted runs keep working). Passphrase is never
// logged or echoed back.
func LoadCryptConfig(keyPath, passphrase string) (*pbscommon.CryptConfig, error) {
	if keyPath == "" {
		return nil, nil
	}

	keyCfg, err := pbscommon.LoadKeyConfig(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key file %s: %w", keyPath, err)
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
