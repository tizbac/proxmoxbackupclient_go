package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				BaseURL:   "https://pbs.example.com:8007",
				AuthID:    "test@pbs!token",
				Secret:    "secret123",
				Datastore: "backup",
				BackupDir: "/tmp/backup",
			},
			wantErr: false,
		},
		{
			name: "missing baseurl",
			config: Config{
				AuthID:    "test@pbs!token",
				Secret:    "secret123",
				Datastore: "backup",
				BackupDir: "/tmp/backup",
			},
			wantErr: true,
		},
		{
			name: "missing authid",
			config: Config{
				BaseURL:   "https://pbs.example.com:8007",
				Secret:    "secret123",
				Datastore: "backup",
				BackupDir: "/tmp/backup",
			},
			wantErr: true,
		},
		{
			name: "missing secret",
			config: Config{
				BaseURL:   "https://pbs.example.com:8007",
				AuthID:    "test@pbs!token",
				Datastore: "backup",
				BackupDir: "/tmp/backup",
			},
			wantErr: true,
		},
		{
			name: "missing datastore",
			config: Config{
				BaseURL:   "https://pbs.example.com:8007",
				AuthID:    "test@pbs!token",
				Secret:    "secret123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// getConfigDir() resolves via %ProgramData% first (so the GUI, running as
	// the logged-in user, and the Windows Service, running as LocalSystem,
	// agree on one shared config location) — NOT via $HOME, which this test
	// used to set with zero effect. Un-redirected, Config.Save() below wrote
	// straight to the REAL C:\ProgramData\ProxmoxBackupClient\config.json on
	// whatever machine ran this test (confirmed live 2026-10-01: it clobbered
	// a real dev machine's config with these exact dummy values). Redirect
	// %ProgramData% itself to a temp dir instead, so the real
	// getConfigDir() logic runs unchanged but resolves somewhere safe.
	tmpDir := t.TempDir()
	oldProgramData := os.Getenv("ProgramData")
	defer func() { _ = os.Setenv("ProgramData", oldProgramData) }()
	_ = os.Setenv("ProgramData", tmpDir)

	config := &Config{
		BaseURL:   "https://pbs.example.com:8007",
		AuthID:    "test@pbs!token",
		Secret:    "secret123",
		Datastore: "backup",
		BackupDir: "/tmp/backup",
		UseVSS:    true,
	}

	// Test Save
	if err := config.Save(); err != nil {
		t.Fatalf("Config.Save() error = %v", err)
	}

	// Verify file exists, at the REAL path getConfigDir() actually resolves
	// to (ProgramData-first), not the old home-dir fallback.
	configPath := filepath.Join(tmpDir, "ProxmoxBackupClient", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file not created at %s", configPath)
	}

	// Test Load
	loadedConfig := LoadConfig()

	if loadedConfig.BaseURL != config.BaseURL {
		t.Errorf("BaseURL = %v, want %v", loadedConfig.BaseURL, config.BaseURL)
	}
	if loadedConfig.AuthID != config.AuthID {
		t.Errorf("AuthID = %v, want %v", loadedConfig.AuthID, config.AuthID)
	}
	if loadedConfig.Secret != config.Secret {
		t.Errorf("Secret = %v, want %v", loadedConfig.Secret, config.Secret)
	}
	if loadedConfig.Datastore != config.Datastore {
		t.Errorf("Datastore = %v, want %v", loadedConfig.Datastore, config.Datastore)
	}
}

func TestGetConfigPath(t *testing.T) {
	// See TestConfigSaveLoad's comment: getConfigDir() resolves via
	// %ProgramData% first, not $HOME — redirect that instead so this
	// doesn't touch (or depend on) the real system config path.
	tmpDir := t.TempDir()
	oldProgramData := os.Getenv("ProgramData")
	defer func() { _ = os.Setenv("ProgramData", oldProgramData) }()
	_ = os.Setenv("ProgramData", tmpDir)

	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "ProxmoxBackupClient", "config.json")
	if configPath != expectedPath {
		t.Errorf("getConfigPath() = %v, want %v", configPath, expectedPath)
	}

	// Verify directory is created
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("Config directory not created at %s", configDir)
	}
}
