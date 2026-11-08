//go:build linux

package api

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
)

// defaultServicePeerCheck accepts conn when root owns its other end and
// every socket listening on the port (checkRootService). The service runs
// as root under systemd, and only root can listen as root.
func defaultServicePeerCheck(conn net.Conn) error {
	addr := conn.RemoteAddr().String()
	end, table, err := serviceEnd(conn, readProcNetTCP)
	if err != nil {
		return &ServicePeerError{Addr: addr, Err: err}
	}
	return checkRootService(addr, end, table)
}

// readProcNetTCP reads the IPv4 and IPv6 socket tables. A dual-stack socket
// listening on [::]:18765 takes IPv4 connections too and lists them in tcp6.
// A system without IPv6 has no tcp6.
func readProcNetTCP() ([]TCPRow, error) {
	var all []TCPRow
	for _, p := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) && p == "/proc/net/tcp6" {
			continue
		}
		if err != nil {
			return nil, err
		}
		rows, err := parseProcNetTCP(string(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		all = append(all, rows...)
	}
	return all, nil
}
