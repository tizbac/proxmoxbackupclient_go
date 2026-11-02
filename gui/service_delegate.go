package main

import (
	"encoding/json"
	"fmt"
)

// serviceCall runs the app method name in the service with args (POST
// /call/{name}, one of apiCalls) and then reloads the app's copy of the
// service's settings, so the window shows what the service kept.
func (a *App) serviceCall(name string, args ...interface{}) error {
	if err := a.apiClient.Call(name, nil, args...); err != nil {
		return err
	}
	if err := a.hydrateFromService(); err != nil {
		writeDebugLog(fmt.Sprintf("[Service] reading the settings back after %s: %v", name, err))
	}
	return nil
}

// hydrateFromService replaces the app's copy of the settings with the
// service's (no secrets in it; storedSecret where one is held).
func (a *App) hydrateFromService() error {
	doc, err := a.apiClient.GetFullConfig()
	if err != nil {
		return err
	}
	cfg, err := configFromServiceDoc(doc)
	if err != nil {
		return err
	}
	a.config = cfg
	return nil
}

// configFromServiceDoc builds the app's copy of the settings from the
// service's settings document (GetFullConfigForAPI).
func configFromServiceDoc(doc map[string]interface{}) (*Config, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	cfg := &Config{PBSServers: map[string]*PBSServer{}}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("the service's settings: %w", err)
	}
	flag := func(k string) bool { v, _ := doc[k].(bool); return v }
	if flag("secret_set") {
		cfg.Secret = storedSecret
	}
	if flag("smtp_password_set") {
		cfg.SMTPPassword = storedSecret
	}
	if cfg.EncryptionKeyPassphraseSet {
		cfg.EncryptionKeyPassphrase = storedSecret
	}
	cfg.EncryptionKeyPassphraseSet = false
	if cfg.PBSServers == nil {
		cfg.PBSServers = map[string]*PBSServer{}
	}
	for id, s := range cfg.PBSServers {
		if s == nil {
			delete(cfg.PBSServers, id)
			continue
		}
		if s.SecretSet {
			s.Secret = storedSecret
		}
		if s.PasswordSet {
			s.Password = storedSecret
		}
		if s.EncryptionKeyPassphraseSet {
			s.EncryptionKeyPassphrase = storedSecret
		}
		s.SecretSet, s.PasswordSet, s.EncryptionKeyPassphraseSet = false, false, false
	}
	return cfg, nil
}
