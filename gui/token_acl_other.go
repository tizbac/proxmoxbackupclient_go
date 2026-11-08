//go:build !windows

package main

import "github.com/salvanza/salvanza_client/gui/api"

// secureTokenFile is the API package's own: the file 0600, root's.
var secureTokenFile = api.SecureTokenFile
