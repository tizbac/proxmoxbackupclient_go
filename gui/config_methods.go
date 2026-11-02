package main

// The settings and PBS server methods the window calls. Compiled into both
// programs: with the service, the app hands each one to the service
// (serviceCall), which runs its own copy against the computer's settings.

import (
	"errors"
	"fmt"

	"pbscommon"
	"security"
)

// SaveConfig saves the configuration
func (a *App) SaveConfig(config *Config) error {
	if a.delegated() {
		return a.serviceCall("SaveConfig", config)
	}
	// M-04: the frontend never receives the stored secrets (GetConfigWithHostname
	// returns "" + a *_set marker), so an empty value here means "keep the existing
	// one", not "clear it". Only overwrite when the user supplied a new value.
	if a.config != nil {
		if config.Secret == "" {
			config.Secret = a.config.Secret
		}
		if config.Theme == nil {
			config.Theme = a.config.Theme
		}
		// Not part of the settings form: the disk table of an earlier version.
		if config.UpstreamPinnedDisks == nil {
			config.UpstreamPinnedDisks = a.config.UpstreamPinnedDisks
		}
		if config.SMTPPassword == "" {
			config.SMTPPassword = a.config.SMTPPassword
		}
		config.EncryptionKeyPassphrase = mergeStoredPassphrase(a.config.EncryptionKeyFile, a.config.EncryptionKeyPassphrase,
			config.EncryptionKeyFile, config.EncryptionKeyPassphrase, config.ClearEncryptionKeyPassphrase)
	}
	config.ClearEncryptionKeyPassphrase = false
	config.EncryptionKeyPassphraseSet = false

	// Log sanitized config (no secrets)
	writeDebugLog(fmt.Sprintf("SaveConfig() called: URL=%s, AuthID=%s, Datastore=%s, BackupID=%s",
		security.SanitizeURL(config.BaseURL),
		config.AuthID,
		config.Datastore,
		config.BackupID))

	// Validate before saving
	if err := config.Validate(); err != nil {
		writeDebugLog(fmt.Sprintf("Config validation failed: %v", err))
		return err
	}

	// Unlock the key BEFORE persisting so the runtime-only Crypt matches what is
	// about to be written, and so an unusable key is reported without having
	// saved a config that cannot be used.
	// Saving never asks for a passphrase: a protected key without a stored one
	// is unlocked when a backup or restore first needs it.
	if err := config.unlockCrypt(false); err != nil && !errors.Is(err, errPassphraseRequired) {
		writeDebugLog(fmt.Sprintf("Encryption key load failed: %v", err))
		return err
	}

	// Save to disk
	if err := config.Save(); err != nil {
		writeDebugLog(fmt.Sprintf("Config save to disk failed: %v", err))
		return err
	}

	// Update in-memory config
	a.config = config
	writeDebugLog("Config saved successfully and loaded into app")
	return nil
}

// TestConnection tests the PBS connection with the provided config (or current if nil)
func (a *App) TestConnection(config *Config) error {
	writeDebugLog("TestConnection() called")
	if a.delegated() {
		// The service holds the stored secret an empty one stands for.
		return a.apiClient.Call("TestConnection", nil, config)
	}

	// Use provided config or fallback to current app config
	testConfig := config
	if testConfig == nil {
		testConfig = a.config
	}

	// M-04: the frontend no longer holds the secret, so an empty secret in the
	// submitted config means "use the stored one" (test the existing connection).
	if testConfig != nil && testConfig.Secret == "" && a.config != nil {
		testConfig.Secret = a.config.Secret
	}

	// Validate config first
	if err := testConfig.Validate(); err != nil {
		return err
	}

	// Mint a fresh PBS session ticket for this operation (username/password).
	testConfig, err := a.withAuth(testConfig)
	if err != nil {
		return err
	}

	// Create PBS client
	client := &pbscommon.PBSClient{
		BaseURL:          testConfig.BaseURL,
		CertFingerPrint:  testConfig.CertFingerprint,
		AuthID:           testConfig.AuthID,
		Secret:           testConfig.Secret,
		Ticket:           testConfig.Ticket,
		CSRFToken:        testConfig.CSRFToken,
		Datastore:        testConfig.Datastore,
		Namespace:        testConfig.Namespace,
		Insecure:         testConfig.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest, // Default for test connections
		Manifest: pbscommon.BackupManifest{
			BackupID: testConfig.BackupID,
		},
	}

	// Debug log with sanitized credentials
	writeDebugLog(fmt.Sprintf("Testing connection: URL=%s, AuthID=%s, Secret=%s, Datastore=%s",
		security.SanitizeURL(testConfig.BaseURL),
		testConfig.AuthID,
		security.SanitizeSecret(testConfig.Secret),
		testConfig.Datastore))

	// Perform real HTTP test (checks DNS, connectivity, auth, datastore access)
	if err := client.TestConnection(); err != nil {
		writeDebugLog(fmt.Sprintf("Connection test failed: %v", err))
		return err
	}

	writeDebugLog("Connection test successful (authenticated + datastore accessible)")
	return nil
}

// AddPBSServer adds a new PBS server to the configuration
func (a *App) AddPBSServer(pbs *PBSServer) error {
	writeDebugLog(fmt.Sprintf("AddPBSServer(%s) called", pbs.ID))
	if a.delegated() {
		return a.serviceCall("AddPBSServer", pbs)
	}
	pbs.ClearEncryptionKeyPassphrase = false
	return a.config.AddPBSServer(pbs)
}

// UpdatePBSServer updates an existing PBS server
func (a *App) UpdatePBSServer(pbs *PBSServer) error {
	writeDebugLog(fmt.Sprintf("UpdatePBSServer(%s) called", pbs.ID))
	if a.delegated() {
		return a.serviceCall("UpdatePBSServer", pbs)
	}
	// M-04: the frontend never receives the token (sanitized), so an empty secret
	// on update means "keep the stored one", not "clear it". Only for the same
	// token ID, though: the stored secret belongs to the old one, so with a new
	// ID it stays empty and validation asks for its secret (upstream b058ed0).
	if pbs.Secret == "" {
		if existing, err := a.config.GetPBSServer(pbs.ID); err == nil && existing != nil && existing.AuthID == pbs.AuthID {
			pbs.Secret = existing.Secret
		}
	}
	prevPath, prevPass := "", ""
	if existing, err := a.config.GetPBSServer(pbs.ID); err == nil && existing != nil {
		prevPath, prevPass = existing.EncryptionKeyFile, existing.EncryptionKeyPassphrase
	}
	pbs.EncryptionKeyPassphrase = mergeStoredPassphrase(prevPath, prevPass, pbs.EncryptionKeyFile, pbs.EncryptionKeyPassphrase, pbs.ClearEncryptionKeyPassphrase)
	pbs.ClearEncryptionKeyPassphrase = false
	pbs.EncryptionKeyPassphraseSet = false
	return a.config.UpdatePBSServer(pbs)
}

// DeletePBSServer removes a PBS server
func (a *App) DeletePBSServer(id string) error {
	writeDebugLog(fmt.Sprintf("DeletePBSServer(%s) called", id))
	if a.delegated() {
		return a.serviceCall("DeletePBSServer", id)
	}
	return a.config.DeletePBSServer(id)
}

// SetDefaultPBSServer sets the default PBS server
func (a *App) SetDefaultPBSServer(id string) error {
	writeDebugLog(fmt.Sprintf("SetDefaultPBSServer(%s) called", id))
	if a.delegated() {
		return a.serviceCall("SetDefaultPBSServer", id)
	}
	return a.config.SetDefaultPBS(id)
}

// TestPBSConnection tests connection to a specific PBS server
func (a *App) TestPBSConnection(pbsID string) error {
	writeDebugLog(fmt.Sprintf("TestPBSConnection(%s) called", pbsID))
	if a.delegated() {
		return a.apiClient.Call("TestPBSConnection", nil, pbsID)
	}

	pbs, err := a.config.GetPBSServer(pbsID)
	if err != nil {
		return err
	}

	// Convert to legacy Config format for existing TestConnection logic
	legacyConfig := pbs.ToConfig()
	return a.TestConnection(legacyConfig)
}
