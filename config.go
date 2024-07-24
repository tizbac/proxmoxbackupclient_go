package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type Config struct {
	BaseURL         string `json:"baseurl"`
	CertFingerprint string `json:"certfingerprint"`
	AuthID          string `json:"authid"`
	Secret          string `json:"secret"`
	Datastore       string `json:"datastore"`
	Namespace       string `json:"namespace"`
	BackupID        string `json:"backup-id"`
	BackupSourceDir string `json:"backupdir"`
	PxarOut         string `json:"pxarout"`
}

func (c *Config) valid() bool {
	return c.BaseURL != "" && c.CertFingerprint != "" && c.AuthID != "" && c.Secret != "" && c.Datastore != "" && c.BackupSourceDir != ""
}

func loadConfig() *Config {
	// Define flags
	baseURLFlag := flag.String("baseurl", "", "Base URL for the proxmox backup server, example: https://192.168.1.10:8007")
	certFingerprintFlag := flag.String("certfingerprint", "", "Certificate fingerprint for SSL connection, example: ea:7d:06:f9...")
	authIDFlag := flag.String("authid", "", "Authentication ID (PBS Api token)")
	secretFlag := flag.String("secret", "", "Secret for authentication")
	datastoreFlag := flag.String("datastore", "", "Datastore name")
	namespaceFlag := flag.String("namespace", "", "Namespace (optional)")
	backupIDFlag := flag.String("backup-id", "", "Backup ID (optional - if not specified, the hostname is used as the default)")
	backupSourceDirFlag := flag.String("backupdir", "", "Backup source directory, must not be symlink")
	pxarOutFlag := flag.String("pxarout", "", "Output PXAR archive for debug purposes (optional)")
	configFile := flag.String("config", "", "Path to JSON config file")

	// Parse command line flags
	flag.Parse()

	// Create a config struct and try to load values from the JSON file if specified
	config := &Config{}
	if *configFile != "" {
		file, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Printf("Error reading config file: %v\n", err)
			os.Exit(1)
		}
		err = json.Unmarshal(file, config)
		if err != nil {
			fmt.Printf("Error parsing config file: %v\n", err)
			os.Exit(1)
		}
	}

	// Override JSON config with command line flags if provided
	if *baseURLFlag != "" {
		config.BaseURL = *baseURLFlag
	}
	if *certFingerprintFlag != "" {
		config.CertFingerprint = *certFingerprintFlag
	}
	if *authIDFlag != "" {
		config.AuthID = *authIDFlag
	}
	if *secretFlag != "" {
		config.Secret = *secretFlag
	}
	if *datastoreFlag != "" {
		config.Datastore = *datastoreFlag
	}
	if *namespaceFlag != "" {
		config.Namespace = *namespaceFlag
	}
	if *backupIDFlag != "" {
		config.BackupID = *backupIDFlag
	}
	if *backupSourceDirFlag != "" {
		config.BackupSourceDir = *backupSourceDirFlag
	}
	if *pxarOutFlag != "" {
		config.PxarOut = *pxarOutFlag
	}

	return config
}
