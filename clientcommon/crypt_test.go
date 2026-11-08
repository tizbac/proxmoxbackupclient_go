package clientcommon

import "testing"

// A PBKDF2 key file made by proxmox-backup-client (see
// pbscommon/official_keyfile_test.go), with its throwaway passphrase.
const (
	officialPBKDF2Key         = "../pbscommon/testdata/official-pbkdf2.key"
	officialPBKDF2Passphrase  = "throwaway-passphrase-for-test"
	officialPBKDF2Fingerprint = "70:44:ba:5f:a7:22:e3:a1:d1:3e:32:d7:3f:58:d6:95:83:44:c3:7d:d6:1c:02:b7:7b:5a:c9:41:8f:9e:c1:ca"
)

func TestLoadCryptConfigPassphraseFromEnvironment(t *testing.T) {
	t.Setenv("PBS_ENCRYPTION_PASSWORD", officialPBKDF2Passphrase)
	crypt, err := LoadCryptConfig(officialPBKDF2Key, "")
	if err != nil {
		t.Fatalf("PBS_ENCRYPTION_PASSWORD did not unlock the key: %v", err)
	}
	if got := crypt.Fingerprint(); got != officialPBKDF2Fingerprint {
		t.Fatalf("fingerprint %s, want %s", got, officialPBKDF2Fingerprint)
	}
}

func TestLoadCryptConfigPassphraseArgumentBeatsEnvironment(t *testing.T) {
	t.Setenv("PBS_ENCRYPTION_PASSWORD", "wrong passphrase")
	if _, err := LoadCryptConfig(officialPBKDF2Key, officialPBKDF2Passphrase); err != nil {
		t.Fatalf("the passphrase argument must be used before the environment: %v", err)
	}
}
