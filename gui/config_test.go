package main

import (
	"os"
	"path/filepath"
	"runtime"
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

// useTempDefaultConfigDir points getConfigDir's default location at a temp
// directory and returns the config.json path it should then resolve to. On
// Windows the default is %ProgramData%\ProxmoxBackupClient, not the home
// directory, so setting HOME alone left these tests writing the real
// C:\ProgramData\ProxmoxBackupClient\config.json of the machine running them.
func useTempDefaultConfigDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	old := configDirOverride
	SetConfigDir("")
	t.Cleanup(func() { SetConfigDir(old) })
	t.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("ProgramData", tmpDir)
		return filepath.Join(tmpDir, "ProxmoxBackupClient", "config.json")
	}
	t.Setenv("ProgramData", "")
	t.Setenv("SystemDrive", "")
	return filepath.Join(tmpDir, ".proxmox-backup-guardian", "config.json")
}

func TestConfigSaveLoad(t *testing.T) {
	configPath := useTempDefaultConfigDir(t)

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
	expectedPath := useTempDefaultConfigDir(t)

	configPath, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath() error = %v", err)
	}

	if configPath != expectedPath {
		t.Errorf("getConfigPath() = %v, want %v", configPath, expectedPath)
	}

	// Verify directory is created
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Errorf("Config directory not created at %s", configDir)
	}
}
