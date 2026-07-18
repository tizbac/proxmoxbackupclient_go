package pbscommon

// A repository string locates a Proxmox Backup Server datastore
// and has the format:  [[auth-id@]host[:port]:]datastore
//
//   auth-id   - user@realm or user@realm!tokenid (default: root@pam)
//   host      - hostname, IPv4, or bracketed IPv6 (default: localhost)
//   port      - 1-65535 (default: 8007)
//   datastore - required name of the datastore on the server
//
// Examples:
//
//	mystore
//	myhost:mystore
//	admin@pam@backuphost:8008:tank
//	[ff80::1]:9007:mystore
//	user@pbs!token@192.168.1.1:1234:store
//
// This file mirrors the Rust implementation from
// proxmox-backup (https://git.proxmox.com/?p=proxmox-backup.git):
//   - backup_repo.rs  (BackupRepository::from_str / Display)
//   - pbs-api-types/src/lib.rs  (BACKUP_REPO_URL_REGEX)
//   - proxmox-auth-api/src/types.rs  (USER_ID_REGEX_STR, APITOKEN_ID_REGEX_STR)
//   - proxmox-schema/src/api_types.rs  (DNS_NAME_STR, IPRE_BRACKET_STR, SAFE_ID_REGEX_STR)

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

const (
	DefaultHost   = "localhost"
	DefaultPort   = "8007"
	DefaultAuthID = "root@pam"
)

// Sub-regex constants copied verbatim from the Rust source.
// proxmox-schema/src/api_types.rs
const (
	_ipv4Octet = `(?:25[0-5]|(?:2[0-4]|1[0-9]|[1-9])?[0-9])`
	_ipv4REStr = `(?:(?:` + _ipv4Octet + `\.){3}` + _ipv4Octet + `)`
	_ipv6H16   = `(?:[0-9a-fA-F]{1,4})`
	_ipv6LS32  = `(?:(?:` + _ipv4REStr + `|` + _ipv6H16 + `:` + _ipv6H16 + `))`

	// IPV6RE_STR from proxmox-schema — all 9 IPv6 compaction alternatives
	_ipv6REStr = `(?:` +
		`(?:(?:(?:` + _ipv6H16 + `:){6})` + _ipv6LS32 + `)|` +
		`(?:(?:::` + `(?:` + _ipv6H16 + `:){5})` + _ipv6LS32 + `)|` +
		`(?:(?:(?:` + _ipv6H16 + `)?::(?:` + _ipv6H16 + `:){4})` + _ipv6LS32 + `)|` +
		`(?:(?:(?:(?:` + _ipv6H16 + `:){0,1}` + _ipv6H16 + `)?::(?:` + _ipv6H16 + `:){3})` + _ipv6LS32 + `)|` +
		`(?:(?:(?:(?:` + _ipv6H16 + `:){0,2}` + _ipv6H16 + `)?::(?:` + _ipv6H16 + `:){2})` + _ipv6LS32 + `)|` +
		`(?:(?:(?:(?:` + _ipv6H16 + `:){0,3}` + _ipv6H16 + `)?::(?:` + _ipv6H16 + `:){1})` + _ipv6LS32 + `)|` +
		`(?:(?:(?:(?:` + _ipv6H16 + `:){0,4}` + _ipv6H16 + `)?::` + `)` + _ipv6LS32 + `)|` +
		`(?:(?:(?:(?:` + _ipv6H16 + `:){0,5}` + _ipv6H16 + `)?::` + `)` + _ipv6H16 + `)|` +
		`(?:(?:(?:(?:` + _ipv6H16 + `:){0,6}` + _ipv6H16 + `)?::` + `)))`

	// IPRE_BRACKET_STR: IPv4 bare or IPv6 inside brackets
	_ipreBracketStr = `(?:` + _ipv4REStr + `|\[(?:` + _ipv6REStr + `)\])`

	// DNS_NAME_STR
	_dnsLabelStr = `(?:[a-zA-Z0-9](?:[a-zA-Z0-9\-]*[a-zA-Z0-9])?)`
	_dnsNameStr  = `(?:(?:` + _dnsLabelStr + `\.)*(?:` + _dnsLabelStr + `))`
)

// proxmox-auth-api/src/types.rs
const (
	_userNameRegexStr = `(?:[^\s:/[:cntrl:]]+)`
	_safeIDRegexStr   = `(?:[A-Za-z0-9_][A-Za-z0-9._\-]*)`
	_userIDRegexStr   = _userNameRegexStr + `@` + _safeIDRegexStr
	_apiTokenIDRegexStr = _userIDRegexStr + `!` + _safeIDRegexStr
)

// Combined regex matching pbs-api-types BACKUP_REPO_URL_REGEX:
//   ^^(?:(?:(USER_ID|APITOKEN_ID)@)?(DNS_NAME|IPRE_BRACKET):)?(?:([0-9]{1,5}):)?(SAFE_ID)$
var repoURLRegex = regexp.MustCompile(
	`^^(?:(?:` +
		`(` + _userIDRegexStr + `|` + _apiTokenIDRegexStr + `)@` +
		`)?(` + _dnsNameStr + `|` + _ipreBracketStr + `):` +
		`)?(?:([0-9]{1,5}):)?(` + _safeIDRegexStr + `)$`,
)

// Matches bare IPv6 addresses (no brackets), mirroring pbs_api_types::IP_V6_REGEX
var ipv6Regex = regexp.MustCompile(`^` + _ipv6REStr + `$`)

func ParseRepoURL(repoURL string) (authID, host, port, datastore string, err error) {
	m := repoURLRegex.FindStringSubmatch(repoURL)
	if m == nil {
		return "", "", "", "", fmt.Errorf("unable to parse repository url '%s'", repoURL)
	}

	authID = m[1] // group 1: auth_id (empty string if not captured)

	// group 3: port — validate range like Rust's str::parse::<u16>()
	if m[3] != "" {
		p, err := strconv.Atoi(m[3])
		if err != nil || p < 1 || p > 65535 {
			return "", "", "", "", fmt.Errorf("unable to parse repository url '%s'", repoURL)
		}
		port = m[3]
	}
	if port == "" {
		port = DefaultPort
	}

	// group 2: host (empty when outer group did not match, e.g. bare datastore)
	if m[2] != "" {
		host = m[2]
	} else {
		host = DefaultHost
	}

	// group 4: datastore — guaranteed non-empty by regex
	datastore = m[4]

	// Mirror Rust's auth_id() accessor: defaults to root@pam
	if authID == "" {
		authID = DefaultAuthID
	}

	return
}

// ApplyPBSEnvVars reads PBS_REPOSITORY, PBS_SERVER/PBS_PORT/PBS_DATASTORE/PBS_AUTH_ID,
// PBS_PASSWORD, and PBS_FINGERPRINT from the environment and populates the corresponding
// config fields. PBS_REPOSITORY takes precedence over the individual atom vars
// (PBS_SERVER, PBS_PORT, PBS_DATASTORE, PBS_AUTH_ID) and is tried first.
// Atom vars are only used when PBS_REPOSITORY is unset or fails to parse.
// This is designed to be called before config file and CLI flag processing,
// matching the resolve_repository flow in the Rust proxmox-backup-client.
func ApplyPBSEnvVars(baseURL, authID, secret, datastore, certFingerprint *string) {
	repoUsed := false
	if repo := os.Getenv("PBS_REPOSITORY"); repo != "" {
		a, h, p, d, err := ParseRepoURL(repo)
		if err != nil {
			fmt.Printf("Warning: ignoring PBS_REPOSITORY: %v\n", err)
		} else {
			repoUsed = true
			*authID = a
			*baseURL = fmt.Sprintf("https://%s:%s", h, p)
			*datastore = d
		}
	}

	if !repoUsed {
		if server := os.Getenv("PBS_SERVER"); server != "" {
			if ipv6Regex.MatchString(server) {
				server = "[" + server + "]"
			}
			port := os.Getenv("PBS_PORT")
			if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
				port = DefaultPort
			}
			*baseURL = fmt.Sprintf("https://%s:%s", server, port)
		}
		if ds := os.Getenv("PBS_DATASTORE"); ds != "" {
			*datastore = ds
		}
		if aid := os.Getenv("PBS_AUTH_ID"); aid != "" {
			*authID = aid
		}
	}

	if pw := os.Getenv("PBS_PASSWORD"); pw != "" {
		*secret = pw
	}
	if fp := os.Getenv("PBS_FINGERPRINT"); fp != "" {
		*certFingerprint = fp
	}
}
