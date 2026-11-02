//go:build linux

package api

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
)

// The real tables: a connection this test makes to its own listener (IPv4,
// or dual-stack, which /proc/net/tcp6 lists) is found, and refused unless
// the test runs as root, naming this test's user. The listener never
// accepts: many kernels then show uid 0 for the connection itself, so the
// refusal must come from the listening socket.
func TestLinuxPeerCheckOwnListener(t *testing.T) {
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
			end, _, err := serviceEnd(conn, readProcNetTCP)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("the connection's end shows uid %d", end.Owner)
			err = defaultServicePeerCheck(conn)
			if os.Geteuid() == 0 {
				if err != nil {
					t.Fatalf("refused root: %v", err)
				}
				return
			}
			var pe *ServicePeerError
			if !errors.As(err, &pe) || !strings.Contains(pe.Owner, fmt.Sprintf("user %d", os.Geteuid())) {
				t.Fatalf("check = %v, want a refusal naming user %d", err, os.Geteuid())
			}
			t.Log(err)
		})
	}
}
