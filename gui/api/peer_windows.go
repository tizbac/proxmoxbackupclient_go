//go:build windows

package api

import (
	"errors"
	"net"
)

// defaultServicePeerCheck refuses: the check needs the Service Control
// Manager and the TCP table (golang.org/x/sys), which the gui package has and
// installs with SetServicePeerCheck (service_peer_windows.go). A program that
// forgot to install it does not connect (fail closed), as SecureTokenFile.
func defaultServicePeerCheck(conn net.Conn) error {
	return &ServicePeerError{Addr: conn.RemoteAddr().String(), Err: errors.New("the check of the service's process is not set up")}
}
