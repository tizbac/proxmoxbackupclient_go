package pbscommon

import (
	"os"
	"testing"
)

type parseCase struct {
	url           string
	wantAuthID    string
	wantHost      string
	wantPort      string
	wantDatastore string
	wantErr       bool
}

func TestParseRepoURL(t *testing.T) {
	// Test cases from Rust backup_repo.rs tests
	cases := []parseCase{
		// parse_datastore_only
		{"mystore", "root@pam", "localhost", "8007", "mystore", false},
		// parse_host_and_datastore
		{"myhost:mystore", "root@pam", "myhost", "8007", "mystore", false},
		// parse_full_with_port
		{"admin@pam@backuphost:8008:tank", "admin@pam", "backuphost", "8008", "tank", false},
		// parse_ipv4_with_port
		{"192.168.1.1:1234:mystore", "root@pam", "192.168.1.1", "1234", "mystore", false},
		// parse_ipv6_with_port
		{"[ff80::1]:9007:mystore", "root@pam", "[ff80::1]", "9007", "mystore", false},
		// parse_api_token
		{"user@pbs!token@myhost:mystore", "user@pbs!token", "myhost", "8007", "mystore", false},
		// parse_invalid_url_errors
		{"", "", "", "", "", true},
		// empty datastore
		{"host:", "", "", "", "", true},
		// empty auth_id before @
		{"@host:mystore", "", "", "", "", true},
		// non-numeric port
		{"host:abc:mystore", "", "", "", "", true},
		// port out of range
		{"192.168.1.1:70000:mystore", "", "", "", "", true},
		// trailing colon (empty datastore)
		{":mystore", "", "", "", "", true},
		// extra colon after port
		{"host:port:", "", "", "", "", true},
		// unclosed IPv6 bracket
		{"[bad:ipv6:mystore", "", "", "", "", true},
	}

	for _, c := range cases {
		authID, host, port, datastore, err := ParseRepoURL(c.url)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRepoURL(%q) expected error", c.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRepoURL(%q) unexpected error: %v", c.url, err)
			continue
		}
		if authID != c.wantAuthID {
			t.Errorf("ParseRepoURL(%q) authID = %q, want %q", c.url, authID, c.wantAuthID)
		}
		if host != c.wantHost {
			t.Errorf("ParseRepoURL(%q) host = %q, want %q", c.url, host, c.wantHost)
		}
		if port != c.wantPort {
			t.Errorf("ParseRepoURL(%q) port = %q, want %q", c.url, port, c.wantPort)
		}
		if datastore != c.wantDatastore {
			t.Errorf("ParseRepoURL(%q) datastore = %q, want %q", c.url, datastore, c.wantDatastore)
		}
	}
}

type applyCase struct {
	name          string
	envs          map[string]string
	inBaseURL     string
	inAuthID      string
	inSecret      string
	inDatastore   string
	inFingerprint string
	wantBaseURL   string
	wantAuthID    string
	wantSecret    string
	wantDatastore string
	wantFP        string
}

func TestApplyPBSEnvVars(t *testing.T) {
	cases := []applyCase{
		{
			name: "no env vars set",
			envs: map[string]string{},
			wantBaseURL:   "",
			wantAuthID:    "",
			wantSecret:    "",
			wantDatastore: "",
			wantFP:        "",
		},
		{
			name: "PBS_REPOSITORY sets all three",
			envs: map[string]string{
				"PBS_REPOSITORY": "admin@pam@backuphost:8008:tank",
			},
			wantBaseURL:   "https://backuphost:8008",
			wantAuthID:    "admin@pam",
			wantSecret:    "",
			wantDatastore: "tank",
			wantFP:        "",
		},
		{
			name: "PBS_REPOSITORY IPv6",
			envs: map[string]string{
				"PBS_REPOSITORY": "[ff80::1]:9007:mystore",
			},
			wantBaseURL:   "https://[ff80::1]:9007",
			wantAuthID:    "root@pam",
			wantSecret:    "",
			wantDatastore: "mystore",
			wantFP:        "",
		},
		{
			name: "PBS_REPOSITORY without auth_id defaults to root@pam",
			envs: map[string]string{
				"PBS_REPOSITORY": "myhost:mystore",
			},
			wantBaseURL:   "https://myhost:8007",
			wantAuthID:    "root@pam",
			wantDatastore: "mystore",
		},
		{
			name: "PBS_SERVER and PBS_PORT set baseURL",
			envs: map[string]string{
				"PBS_SERVER": "pbs.example.com",
				"PBS_PORT":   "9000",
			},
			wantBaseURL: "https://pbs.example.com:9000",
			wantAuthID:  "",
		},
		{
			name: "PBS_SERVER without PBS_PORT defaults to 8007",
			envs: map[string]string{
				"PBS_SERVER": "pbs.example.com",
			},
			wantBaseURL: "https://pbs.example.com:8007",
			wantAuthID:  "",
		},
		{
			name: "PBS_SERVER bare IPv6 gets bracketed",
			envs: map[string]string{
				"PBS_SERVER": "ff80::1",
				"PBS_PORT":   "9007",
			},
			wantBaseURL: "https://[ff80::1]:9007",
			wantAuthID:  "",
		},
		{
			name: "PBS_SERVER already-bracketed IPv6 passes through",
			envs: map[string]string{
				"PBS_SERVER": "[ff80::1]",
				"PBS_PORT":   "9007",
			},
			wantBaseURL: "https://[ff80::1]:9007",
			wantAuthID:  "",
		},
		{
			name: "PBS_DATASTORE sets datastore",
			envs: map[string]string{
				"PBS_DATASTORE": "myds",
			},
			wantDatastore: "myds",
			wantAuthID:    "",
		},
		{
			name: "PBS_AUTH_ID sets authID",
			envs: map[string]string{
				"PBS_AUTH_ID": "backup@pam",
			},
			wantAuthID: "backup@pam",
		},
		{
			name: "PBS_REPOSITORY takes precedence over individual vars",
			envs: map[string]string{
				"PBS_REPOSITORY": "repohost:repostore",
				"PBS_SERVER":     "envhost",
				"PBS_DATASTORE":  "envstore",
				"PBS_AUTH_ID":    "envuser@pam",
			},
			wantBaseURL:   "https://repohost:8007",
			wantAuthID:    "root@pam",
			wantDatastore: "repostore",
		},
		{
			name: "individual atom vars combine",
			envs: map[string]string{
				"PBS_SERVER":    "atomhost",
				"PBS_DATASTORE": "atomstore",
				"PBS_AUTH_ID":   "atomuser@pam",
			},
			wantBaseURL:   "https://atomhost:8007",
			wantAuthID:    "atomuser@pam",
			wantDatastore: "atomstore",
		},
		{
			name: "PBS_REPOSITORY parse error falls back to individual vars",
			envs: map[string]string{
				"PBS_REPOSITORY": "@host:mystore",
				"PBS_SERVER":     "fallback",
				"PBS_DATASTORE":  "ds",
			},
			wantBaseURL:   "https://fallback:8007",
			wantAuthID:    "",
			wantDatastore: "ds",
		},
		{
			name: "PBS_PASSWORD sets secret",
			envs: map[string]string{
				"PBS_PASSWORD": "my-api-token",
			},
			wantSecret: "my-api-token",
			wantAuthID: "",
		},
		{
			name: "PBS_FINGERPRINT sets cert fingerprint",
			envs: map[string]string{
				"PBS_FINGERPRINT": "ab:cd:ef",
			},
			wantFP:    "ab:cd:ef",
			wantAuthID: "",
		},
		{
			name: "all env vars set",
			envs: map[string]string{
				"PBS_REPOSITORY":  "user@pbs!token@myhost:mystore",
				"PBS_PASSWORD":    "token-secret",
				"PBS_FINGERPRINT": "12:34:56:78",
			},
			wantBaseURL:   "https://myhost:8007",
			wantAuthID:    "user@pbs!token",
			wantSecret:    "token-secret",
			wantDatastore: "mystore",
			wantFP:        "12:34:56:78",
		},
		{
			name: "bad PBS_PORT is silently ignored",
			envs: map[string]string{
				"PBS_SERVER": "myhost",
				"PBS_PORT":   "notanumber",
			},
			wantBaseURL: "https://myhost:8007",
			wantAuthID:  "",
		},
		{
			name: "out of range PBS_PORT is silently ignored",
			envs: map[string]string{
				"PBS_SERVER": "myhost",
				"PBS_PORT":   "70000",
			},
			wantBaseURL: "https://myhost:8007",
			wantAuthID:  "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.envs {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			baseURL := c.inBaseURL
			authID := c.inAuthID
			secret := c.inSecret
			datastore := c.inDatastore
			fp := c.inFingerprint
			ApplyPBSEnvVars(&baseURL, &authID, &secret, &datastore, &fp)
			if baseURL != c.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", baseURL, c.wantBaseURL)
			}
			if authID != c.wantAuthID {
				t.Errorf("authID = %q, want %q", authID, c.wantAuthID)
			}
			if secret != c.wantSecret {
				t.Errorf("secret = %q, want %q", secret, c.wantSecret)
			}
			if datastore != c.wantDatastore {
				t.Errorf("datastore = %q, want %q", datastore, c.wantDatastore)
			}
			if fp != c.wantFP {
				t.Errorf("fingerprint = %q, want %q", fp, c.wantFP)
			}
		})
	}
}
