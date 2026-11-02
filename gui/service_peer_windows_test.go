//go:build windows && !service

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// The real TCP table: a connection this test makes to its own listener is
// found, owned by this test's process, and refused, since this test is not
// the background service.
func TestServicePeerOwnListener(t *testing.T) {
	for _, tc := range []struct{ name, listen string }{
		{"IPv4", "127.0.0.1:0"},
		{"dual-stack", "[::]:0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", tc.listen)
			if err != nil {
				t.Skipf("listen %s: %v", tc.listen, err)
			}
			defer func() { _ = ln.Close() }()
			_, port, _ := net.SplitHostPort(ln.Addr().String())
			conn, err := net.Dial("tcp4", "127.0.0.1:"+port)
			if err != nil {
				t.Skipf("dial: %v", err)
			}
			defer func() { _ = conn.Close() }()
			pid, err := api.ServiceEndOwner(conn, tcpTableRows)
			if err != nil {
				t.Fatal(err)
			}
			if int(pid) != os.Getpid() {
				t.Fatalf("owner process %d, want this test's %d", pid, os.Getpid())
			}
			err = checkServicePeer(conn)
			var pe *api.ServicePeerError
			if !errors.As(err, &pe) || !strings.HasPrefix(pe.Owner, fmt.Sprintf("process %d", os.Getpid())) {
				t.Fatalf("check = %v, want a refusal naming process %d", err, os.Getpid())
			}
			t.Log(err)
		})
	}
}
