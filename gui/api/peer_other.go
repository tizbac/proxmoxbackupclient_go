//go:build !linux && !windows

package api

import "net"

// defaultServicePeerCheck is nil on macOS and the other systems: they have
// no check yet, and the app connects to whatever answers on 127.0.0.1:18765,
// as before.
var defaultServicePeerCheck func(conn net.Conn) error
