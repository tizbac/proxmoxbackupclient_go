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
	// Create temp directory for test
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Set temp HOME for test
	_ = os.Setenv("HOME", tmpDir)

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

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".proxmox-backup-guardian", "config.json")
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
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	_ = os.Setenv("HOME", tmpDir)

	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".proxmox-backup-guardian", "config.json")
	if configPath != expectedPath {
		t.Errorf("getConfigPath() = %v, want %v", configPath, expectedPath)
	}

	// Verify directory is created
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("Config directory not created at %s", configDir)
	}
}
