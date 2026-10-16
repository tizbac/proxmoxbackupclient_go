package pbscommon

import (
	"testing"
)

// The key files in testdata were created by the real
// `proxmox-backup-client key create --kdf scrypt|pbkdf2` (client 3.4.9) with
// the throwaway passphrase below. They guard against reading the protected
// key format wrongly, which is a tagged enum: {"kdf":{"Scrypt":{...}}}.
const officialKeyTestPassphrase = "throwaway-passphrase-for-test"

func TestLoadOfficialPassphraseKeyFiles(t *testing.T) {
	cases := []struct {
		file        string
		fingerprint string
	}{
		{"testdata/official-scrypt.key", "be:6c:3e:ae:84:21:9c:ef:a0:89:aa:b2:71:b5:96:7b:91:97:f6:71:6f:1b:ac:b5:ae:e9:cc:d0:d6:2a:b7:97"},
		{"testdata/official-pbkdf2.key", "70:44:ba:5f:a7:22:e3:a1:d1:3e:32:d7:3f:58:d6:95:83:44:c3:7d:d6:1c:02:b7:7b:5a:c9:41:8f:9e:c1:ca"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			cfg, err := LoadKeyFile(c.file, []byte(officialKeyTestPassphrase))
			if err != nil {
				t.Fatalf("could not unlock key file made by proxmox-backup-client: %v", err)
			}
			if got := cfg.Fingerprint(); got != c.fingerprint {
				t.Fatalf("fingerprint %s, want %s", got, c.fingerprint)
			}
			if _, err := LoadKeyFile(c.file, []byte("wrong passphrase")); err == nil {
				t.Fatal("a wrong passphrase must not unlock the key")
			}
		})
	}
}
