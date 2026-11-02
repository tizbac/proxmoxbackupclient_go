//go:build !windows

package main

import "github.com/tizbac/proxmoxbackupclient_go/gui/api"

// secureTokenFile is the API package's own: the file 0600, root's.
var secureTokenFile = api.SecureTokenFile
