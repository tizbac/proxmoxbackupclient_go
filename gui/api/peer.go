package api

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Who answers on the service's port. While the background service is
// stopped, any program on this computer may listen on 127.0.0.1:18765 and
// pose as it: the app would send it the service's token and, worse, any PBS
// secret typed into Preferences. So before the app sends a byte on a new
// connection to the service's address it checks who holds the other end of
// that very connection:
//
//   - Windows: the process that owns it (the system's TCP table) must be the
//     running service's, as the Service Control Manager reports it. The gui
//     package installs this check (service_peer_windows.go: it needs
//     golang.org/x/sys); until then every connection is refused.
//   - Linux: the connection's socket and every socket listening on the port
//     must belong to root (/proc/net/tcp and tcp6; the Linux notes below
//     say why both). A program without root cannot open a socket as root, and
//     root can do anything anyway.
//   - macOS and other systems: no check yet (peer_other.go).
//
// A connection that fails the check is closed before anything is sent, and
// the dial fails with a *ServicePeerError: the app sees a service that is not
// running. One table lookup per new connection; connections are kept alive.

// servicePort is the port of the service's API. A var so that tests can point
// the check at a listener of their own.
var servicePort = "18765"

// ServicePeerError is why a connection to the service's address was refused:
// something other than the background service holds its other end (Owner
// says what), or the check could not tell (Err).
type ServicePeerError struct {
	Addr  string // the address dialled, 127.0.0.1:18765
	Owner string // "process 1234", "user 1000 (alice), not root"
	Err   error  // the check failed: nobody could be named
}

func (e *ServicePeerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("could not tell who is answering on %s (%v): not connecting", e.Addr, e.Err)
	}
	return fmt.Sprintf("something other than the background service is answering on %s (%s): not connecting", e.Addr, e.Owner)
}

func (e *ServicePeerError) Unwrap() error { return e.Err }

var (
	peerCheckMu sync.RWMutex
	peerCheck   = defaultServicePeerCheck
	peerLogMu   sync.RWMutex
	peerLog     = func(msg string) { log.Print(msg) }
)

// SetServicePeerCheck installs the check run on every new connection to the
// service's address, before anything is sent on it, and returns the one it
// replaces. It returns nil when conn's other end is the background service,
// otherwise a *ServicePeerError. nil turns the check off (tests only).
func SetServicePeerCheck(check func(conn net.Conn) error) func(conn net.Conn) error {
	peerCheckMu.Lock()
	defer peerCheckMu.Unlock()
	old := peerCheck
	peerCheck = check
	return old
}

func servicePeerCheck() func(conn net.Conn) error {
	peerCheckMu.RLock()
	defer peerCheckMu.RUnlock()
	return peerCheck
}

// SetPeerLog sends the line written when a connection is refused to logf
// (the app's debug log) instead of the standard logger.
func SetPeerLog(logf func(msg string)) {
	peerLogMu.Lock()
	peerLog = logf
	peerLogMu.Unlock()
}

func logPeer(msg string) {
	peerLogMu.RLock()
	logf := peerLog
	peerLogMu.RUnlock()
	if logf != nil {
		logf(msg)
	}
}

// isServiceAddr reports whether addr (host:port, as dialled) is the
// service's: a loopback address on the service's port. Other addresses (an
// httptest server in tests) are not checked.
func isServiceAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port != servicePort {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.Unmap().IsLoopback()
}

// dialService dials addr and, when it is the service's address, runs the
// peer check on the new connection before handing it to the HTTP client.
func dialService(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: ConnectionTimeout, KeepAlive: 30 * time.Second}
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil || !isServiceAddr(addr) {
		return conn, err
	}
	check := servicePeerCheck()
	if check == nil {
		return conn, nil
	}
	if err := check(conn); err != nil {
		_ = conn.Close()
		var pe *ServicePeerError
		if !errors.As(err, &pe) {
			err = &ServicePeerError{Addr: addr, Err: err}
		}
		logPeer(fmt.Sprintf("[API] %v", err))
		return nil, err
	}
	return conn, nil
}

// serviceTransport is the HTTP transport of every Client: http.DefaultTransport's
// settings, no proxy (the service is on this computer), and dialService.
var serviceTransport = newServiceTransport()

func newServiceTransport() *http.Transport {
	var t *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t = dt.Clone()
	} else {
		t = &http.Transport{MaxIdleConns: 100, IdleConnTimeout: 90 * time.Second}
	}
	t.Proxy = nil
	t.DialContext = dialService
	return t
}

// TCPRow is one socket in the system's TCP table: its own endpoint (Local),
// the other end (Remote), whether it is listening or in TIME_WAIT (neither
// carries a live connection), and its owner: a process ID on Windows, a
// user ID on Linux.
type TCPRow struct {
	Local, Remote       netip.AddrPort
	Listening, TimeWait bool
	Owner               uint32
}

// errNoServiceEnd: the table has no row for the service's end of the
// connection.
var errNoServiceEnd = errors.New("no socket in the TCP table holds the other end of the connection")

// peerLookupTries and peerLookupPause bound the wait for the service's end
// of a new connection to show up in the table: on loopback it is there when
// the dial returns, or a moment later.
const (
	peerLookupTries = 25
	peerLookupPause = 10 * time.Millisecond
)

// ServiceEndOwner returns the owner of the other end of conn: the row of the
// TCP table (rows, read again while it is not there yet) whose own endpoint is
// conn's remote address and whose other end is conn's local address.
func ServiceEndOwner(conn net.Conn, rows func() ([]TCPRow, error)) (uint32, error) {
	end, _, err := serviceEnd(conn, rows)
	return end.Owner, err
}

// serviceEnd is ServiceEndOwner, returning the row and the table it was
// found in.
func serviceEnd(conn net.Conn, rows func() ([]TCPRow, error)) (TCPRow, []TCPRow, error) {
	server, err := addrPortOf(conn.RemoteAddr())
	if err != nil {
		return TCPRow{}, nil, err
	}
	client, err := addrPortOf(conn.LocalAddr())
	if err != nil {
		return TCPRow{}, nil, err
	}
	for i := 0; ; i++ {
		table, err := rows()
		if err != nil {
			return TCPRow{}, nil, err
		}
		if row, ok := findServiceEnd(table, server, client); ok {
			return row, table, nil
		}
		if i == peerLookupTries-1 {
			return TCPRow{}, nil, errNoServiceEnd
		}
		time.Sleep(peerLookupPause)
	}
}

func addrPortOf(a net.Addr) (netip.AddrPort, error) {
	if ta, ok := a.(*net.TCPAddr); ok {
		return ta.AddrPort(), nil
	}
	if a == nil {
		return netip.AddrPort{}, errors.New("connection has no address")
	}
	return netip.ParseAddrPort(a.String())
}

// unmapped is ap with an IPv4-mapped IPv6 address (a dual-stack socket's
// view of an IPv4 peer) as plain IPv4.
func unmapped(ap netip.AddrPort) netip.AddrPort {
	return netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
}

// findServiceEnd finds the server's end of the connection from client to
// server among rows: Local is server and Remote is client. Listening and
// TIME_WAIT sockets never carry the live connection and are skipped.
func findServiceEnd(rows []TCPRow, server, client netip.AddrPort) (TCPRow, bool) {
	server, client = unmapped(server), unmapped(client)
	for _, r := range rows {
		if r.Listening || r.TimeWait {
			continue
		}
		if unmapped(r.Local) == server && unmapped(r.Remote) == client {
			return r, true
		}
	}
	return TCPRow{}, false
}

// Linux: /proc/net/tcp and /proc/net/tcp6 list every socket, one per line:
//
//	sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
//	 0: 0100007F:494D 0100007F:D431 01 00000000:00000000 00:00000000 00000000     0        0 23456 ...
//
// An address is the hex of its 32-bit words as this computer stores them
// (native byte order), the port is plain hex, st is the state (0A
// listening, 06 TIME_WAIT) and uid the socket's owner. The parser and the
// verdict are here, not in peer_linux.go, so that their tests run
// everywhere.
//
// The uid of the service's end of a connection is not enough. Until the
// listening program accepts the connection, many kernels (5.15 among them)
// show uid 0 for it whoever listens, so a squatter that is slow to accept
// would pass as root. The listening socket's uid is always its creator's. So the verdict
// asks for root on the connection's end AND on every socket listening on the
// port: a program without root can neither listen as root nor share a port
// with root's socket (SO_REUSEPORT needs the same user).

const (
	procStateTimeWait = 0x06
	procStateListen   = 0x0A
)

// parseProcNetTCP reads the rows of /proc/net/tcp or tcp6 text. Owner is
// the uid.
func parseProcNetTCP(text string) ([]TCPRow, error) {
	var rows []TCPRow
	for i, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 8 {
			continue // the heading, or a blank line
		}
		local, err := parseProcAddr(f[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		remote, err := parseProcAddr(f[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		st, err := strconv.ParseUint(f[3], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("line %d: state %q: %w", i+1, f[3], err)
		}
		uid, err := strconv.ParseUint(f[7], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: uid %q: %w", i+1, f[7], err)
		}
		rows = append(rows, TCPRow{
			Local:     local,
			Remote:    remote,
			Listening: st == procStateListen,
			TimeWait:  st == procStateTimeWait,
			Owner:     uint32(uid), // #nosec G115 -- ParseUint above bounds it to 32 bits
		})
	}
	return rows, nil
}

// errNoListener: nothing listens on the service's port any more (it closed
// after the connection was made).
var errNoListener = errors.New("no socket listens on the port")

// checkRootService is the Linux verdict on end, the service's end of the
// connection, found in table: nil when root owns it and every socket
// listening on its port, otherwise a *ServicePeerError naming the first
// user that is not root.
func checkRootService(addr string, end TCPRow, table []TCPRow) error {
	port := end.Local.Port()
	listeners := 0
	owner := end.Owner
	for _, r := range table {
		if !r.Listening || r.Local.Port() != port {
			continue
		}
		listeners++
		if owner == 0 {
			owner = r.Owner
		}
	}
	if owner != 0 {
		return &ServicePeerError{Addr: addr, Owner: userLabel(owner)}
	}
	if listeners == 0 {
		return &ServicePeerError{Addr: addr, Err: errNoListener}
	}
	return nil
}

// userLabel names uid for the message and the log: "a program of user 1000
// (alice), not root".
func userLabel(uid uint32) string {
	if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
		return fmt.Sprintf("a program of user %d (%s), not root", uid, u.Username)
	}
	return fmt.Sprintf("a program of user %d, not root", uid)
}

// parseProcAddr reads "0100007F:494D" (IPv4) or 32 hex digits and a port
// (IPv6).
func parseProcAddr(s string) (netip.AddrPort, error) {
	h, p, ok := strings.Cut(s, ":")
	if !ok {
		return netip.AddrPort{}, fmt.Errorf("address %q", s)
	}
	port, err := strconv.ParseUint(p, 16, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("port %q: %w", p, err)
	}
	raw, err := hex.DecodeString(h)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return netip.AddrPort{}, fmt.Errorf("address %q", h)
	}
	b := make([]byte, len(raw))
	for i := 0; i < len(raw); i += 4 {
		binary.NativeEndian.PutUint32(b[i:], binary.BigEndian.Uint32(raw[i:]))
	}
	addr, _ := netip.AddrFromSlice(b)
	return netip.AddrPortFrom(addr, uint16(port)), nil // #nosec G115 -- ParseUint above bounds it to 16 bits
}

// Windows: GetExtendedTcpTable with TCP_TABLE_OWNER_PID_ALL fills a buffer
// with a count (DWORD) and then the rows, MIB_TCPROW_OWNER_PID (IPv4) or
// MIB_TCP6ROW_OWNER_PID (IPv6). Addresses are in network order, ports in the
// first two bytes of their DWORD in network order, the other DWORDs in the
// machine's order (little-endian on every Windows). Parsed here, not in the
// gui package, so that the tests run everywhere.

const (
	winTCPRowSize    = 24 // state, local addr, local port, remote addr, remote port, pid
	winTCP6RowSize   = 56 // local addr[16], scope, port, remote addr[16], scope, port, state, pid
	winStateListen   = 2
	winStateTimeWait = 11
)

// ParseTCPTableOwnerPID reads the rows of a buffer GetExtendedTcpTable
// filled (TCP_TABLE_OWNER_PID_ALL; ipv6 for AF_INET6). Owner is the process
// ID.
func ParseTCPTableOwnerPID(buf []byte, ipv6 bool) ([]TCPRow, error) {
	if len(buf) < 4 {
		return nil, errors.New("TCP table too short")
	}
	n := int(binary.LittleEndian.Uint32(buf))
	size := winTCPRowSize
	if ipv6 {
		size = winTCP6RowSize
	}
	if n > (len(buf)-4)/size {
		return nil, fmt.Errorf("TCP table holds %d bytes, too few for %d rows", len(buf), n)
	}
	rows := make([]TCPRow, 0, n)
	for i := 0; i < n; i++ {
		r := buf[4+i*size : 4+(i+1)*size]
		var row TCPRow
		var state uint32
		if ipv6 {
			row.Local = netip.AddrPortFrom(netip.AddrFrom16([16]byte(r[0:16])), binary.BigEndian.Uint16(r[20:22]))
			row.Remote = netip.AddrPortFrom(netip.AddrFrom16([16]byte(r[24:40])), binary.BigEndian.Uint16(r[44:46]))
			state = binary.LittleEndian.Uint32(r[48:52])
			row.Owner = binary.LittleEndian.Uint32(r[52:56])
		} else {
			state = binary.LittleEndian.Uint32(r[0:4])
			row.Local = netip.AddrPortFrom(netip.AddrFrom4([4]byte(r[4:8])), binary.BigEndian.Uint16(r[8:10]))
			row.Remote = netip.AddrPortFrom(netip.AddrFrom4([4]byte(r[12:16])), binary.BigEndian.Uint16(r[16:18]))
			row.Owner = binary.LittleEndian.Uint32(r[20:24])
		}
		row.Listening = state == winStateListen
		row.TimeWait = state == winStateTimeWait
		rows = append(rows, row)
	}
	return rows, nil
}
