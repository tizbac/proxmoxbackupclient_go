package api

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The app checks who answers on the service's port before it sends anything
// (peer.go): the parsers of the two systems' TCP tables, the search for the
// service's end of a connection, and the dialer that refuses an impostor.

var (
	svcEnd    = netip.MustParseAddrPort("127.0.0.1:18765")
	clientEnd = netip.MustParseAddrPort("127.0.0.1:54321")
)

func TestFindServiceEnd(t *testing.T) {
	rows := []TCPRow{
		{Local: svcEnd, Remote: netip.MustParseAddrPort("0.0.0.0:0"), Listening: true, Owner: 1}, // the listener
		{Local: svcEnd, Remote: clientEnd, TimeWait: true, Owner: 0},                             // an old TIME_WAIT
		{Local: clientEnd, Remote: svcEnd, Owner: 7},                                             // the app's own end
		{Local: svcEnd, Remote: netip.MustParseAddrPort("127.0.0.1:50000"), Owner: 8},            // another client
		{Local: svcEnd, Remote: clientEnd, Owner: 4242},                                          // the service's end
	}
	row, ok := findServiceEnd(rows, svcEnd, clientEnd)
	if !ok || row.Owner != 4242 {
		t.Fatalf("found %+v (%v), want the row owned by 4242", row, ok)
	}
	if _, ok := findServiceEnd(rows[:4], svcEnd, clientEnd); ok {
		t.Fatal("found a service end in a table without one")
	}

	// A dual-stack socket on [::]:18765 lists an IPv4 connection with
	// IPv4-mapped addresses.
	mapped := []TCPRow{{
		Local:  netip.MustParseAddrPort("[::ffff:127.0.0.1]:18765"),
		Remote: netip.MustParseAddrPort("[::ffff:127.0.0.1]:54321"),
		Owner:  1000,
	}}
	if row, ok := findServiceEnd(mapped, svcEnd, clientEnd); !ok || row.Owner != 1000 {
		t.Fatalf("IPv4-mapped row: found %+v (%v), want the row owned by 1000", row, ok)
	}
}

// /proc/net/tcp as a little-endian Linux writes it: 0100007F is 127.0.0.1,
// 494D is 18765, D431 is 54321, 0016 is 22. LUID is the listener's uid, EUID
// that of the service's end of the connection, and a user's own listener on
// port 22 does not count.
const procNetTCPSample = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:494D 00000000:0000 0A 00000000:00000000 00:00000000 00000000  LUID        0 21345 1 0000000000000000 100 0 0 10 0
   1: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 21346 1 0000000000000000 100 0 0 10 0
   2: 0100007F:D431 0100007F:494D 01 00000000:00000000 00:00000000 00000000  1000        0 31234 1 0000000000000000 20 4 30 10 -1
   3: 0100007F:494D 0100007F:D431 01 00000000:00000000 00:00000000 00000000  EUID        0 31235 1 0000000000000000 20 4 30 10 -1
`

// The same connection to a dual-stack listener, in /proc/net/tcp6.
const procNetTCP6Sample = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:494D 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  LUID        0 41000 1 0000000000000000 100 0 0 10 0
   1: 0000000000000000FFFF00000100007F:494D 0000000000000000FFFF00000100007F:D431 01 00000000:00000000 00:00000000 00000000  EUID        0 41001 1 0000000000000000 20 4 30 10 -1
`

func littleEndianHost(t *testing.T) {
	t.Helper()
	if binary.NativeEndian.Uint16([]byte{1, 0}) != 1 {
		t.Skip("the samples are a little-endian computer's")
	}
}

func TestParseProcNetTCPAndRootVerdict(t *testing.T) {
	littleEndianHost(t)
	for _, tc := range []struct {
		name, text string
		luid, euid string
		refused    string // "" for accepted, else the uid named
	}{
		{"tcp, root", procNetTCPSample, "0", "0", ""},
		// Not accepted yet: many kernels show uid 0 for the connection.
		{"tcp, user listening, not accepted", procNetTCPSample, "1000", "0", "user 1000"},
		{"tcp, user listening, accepted", procNetTCPSample, "1000", "1000", "user 1000"},
		{"tcp, root listening, user end", procNetTCPSample, "0", "1001", "user 1001"},
		{"tcp6, root", procNetTCP6Sample, "0", "0", ""},
		{"tcp6, user listening, not accepted", procNetTCP6Sample, "1000", "0", "user 1000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(strings.Replace(tc.text, "LUID", tc.luid, 1), "EUID", tc.euid, 1)
			rows, err := parseProcNetTCP(text)
			if err != nil {
				t.Fatal(err)
			}
			if !rows[0].Listening || rows[0].TimeWait {
				t.Errorf("the listener: %+v", rows[0])
			}
			end, ok := findServiceEnd(rows, svcEnd, clientEnd)
			if !ok || strconv.Itoa(int(end.Owner)) != tc.euid {
				t.Fatalf("service end %+v (%v), want uid %s", end, ok, tc.euid)
			}
			err = checkRootService("127.0.0.1:18765", end, rows)
			if tc.refused == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			var pe *ServicePeerError
			if !errors.As(err, &pe) || !strings.Contains(pe.Owner, tc.refused) || !strings.Contains(pe.Owner, "not root") {
				t.Fatalf("verdict %v, want a refusal naming %s", err, tc.refused)
			}
		})
	}

	// The listener closed after the connection was made: refused too.
	rows, err := parseProcNetTCP(strings.Replace(strings.Replace(procNetTCPSample, "LUID", "0", 1), "EUID", "0", 1))
	if err != nil {
		t.Fatal(err)
	}
	end, _ := findServiceEnd(rows, svcEnd, clientEnd)
	if err := checkRootService("127.0.0.1:18765", end, rows[1:]); !errors.Is(err, errNoListener) {
		t.Errorf("without a listener: %v, want errNoListener", err)
	}

	if _, err := parseProcNetTCP("heading\n   0: 0100007F 00000000:0000 0A 0 0 0 0 0\n"); err == nil {
		t.Error("an address without a port parsed")
	}
}

// putRow4 and putRow6 lay out MIB_TCPROW_OWNER_PID and MIB_TCP6ROW_OWNER_PID
// as Windows fills them.
func putRow4(b []byte, state uint32, local, remote netip.AddrPort, pid uint32) {
	binary.LittleEndian.PutUint32(b[0:], state)
	l, r := local.Addr().As4(), remote.Addr().As4()
	copy(b[4:8], l[:])
	binary.BigEndian.PutUint16(b[8:], local.Port())
	copy(b[12:16], r[:])
	binary.BigEndian.PutUint16(b[16:], remote.Port())
	binary.LittleEndian.PutUint32(b[20:], pid)
}

func putRow6(b []byte, state uint32, local, remote netip.AddrPort, pid uint32) {
	l, r := local.Addr().As16(), remote.Addr().As16()
	copy(b[0:16], l[:])
	binary.BigEndian.PutUint16(b[20:], local.Port())
	copy(b[24:40], r[:])
	binary.BigEndian.PutUint16(b[44:], remote.Port())
	binary.LittleEndian.PutUint32(b[48:], state)
	binary.LittleEndian.PutUint32(b[52:], pid)
}

func TestParseTCPTableOwnerPID(t *testing.T) {
	const estab = 5
	buf := make([]byte, 4+3*winTCPRowSize)
	binary.LittleEndian.PutUint32(buf, 3)
	putRow4(buf[4:], winStateListen, svcEnd, netip.MustParseAddrPort("0.0.0.0:0"), 900)
	putRow4(buf[4+winTCPRowSize:], estab, clientEnd, svcEnd, 1234)
	putRow4(buf[4+2*winTCPRowSize:], estab, svcEnd, clientEnd, 900)
	rows, err := ParseTCPTableOwnerPID(buf, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || !rows[0].Listening || rows[1].Owner != 1234 {
		t.Fatalf("rows %+v", rows)
	}
	if row, ok := findServiceEnd(rows, svcEnd, clientEnd); !ok || row.Owner != 900 {
		t.Fatalf("service end %+v (%v), want process 900", row, ok)
	}
	// A squatter's process answers instead.
	putRow4(buf[4+2*winTCPRowSize:], estab, svcEnd, clientEnd, 6666)
	rows, _ = ParseTCPTableOwnerPID(buf, false)
	if row, _ := findServiceEnd(rows, svcEnd, clientEnd); row.Owner != 6666 {
		t.Fatalf("service end %+v, want process 6666", row)
	}

	buf6 := make([]byte, 4+2*winTCP6RowSize)
	binary.LittleEndian.PutUint32(buf6, 2)
	putRow6(buf6[4:], winStateTimeWait, netip.MustParseAddrPort("[::ffff:127.0.0.1]:18765"), netip.MustParseAddrPort("[::ffff:127.0.0.1]:54321"), 0)
	putRow6(buf6[4+winTCP6RowSize:], estab, netip.MustParseAddrPort("[::ffff:127.0.0.1]:18765"), netip.MustParseAddrPort("[::ffff:127.0.0.1]:54321"), 4321)
	rows, err = ParseTCPTableOwnerPID(buf6, true)
	if err != nil {
		t.Fatal(err)
	}
	if row, ok := findServiceEnd(rows, svcEnd, clientEnd); !ok || row.Owner != 4321 {
		t.Fatalf("IPv6 table: service end %+v (%v), want process 4321", row, ok)
	}

	if _, err := ParseTCPTableOwnerPID(buf[:4+winTCPRowSize], false); err == nil {
		t.Error("a table shorter than its count parsed")
	}
}

func TestIsServiceAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:18765":    true,
		"localhost:18765":    true,
		"[::1]:18765":        true,
		"127.0.0.1:18766":    false,
		"192.168.1.10:18765": false,
		"127.0.0.1":          false,
	} {
		if got := isServiceAddr(addr); got != want {
			t.Errorf("isServiceAddr(%q) = %v, want %v", addr, got, want)
		}
	}
}

// asServicePort makes the check treat port (a test listener's) as the
// service's, and installs check.
func asServicePort(t *testing.T, port string, check func(net.Conn) error) {
	t.Helper()
	oldPort := servicePort
	servicePort = port
	old := SetServicePeerCheck(check)
	t.Cleanup(func() {
		servicePort = oldPort
		SetServicePeerCheck(old)
		serviceTransport.CloseIdleConnections()
	})
}

// An impostor on the service's port gets a connection, but not one byte: no
// token, no settings. The client reports it as a service that is not there.
func TestDialRefusesImpostor(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	got := make(chan int, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			buf := make([]byte, 4096)
			n, _ := c.Read(buf)
			_ = c.Close()
			got <- n
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	var checked atomic.Int32
	asServicePort(t, port, func(conn net.Conn) error {
		checked.Add(1)
		return &ServicePeerError{Addr: conn.RemoteAddr().String(), Owner: "process 6666"}
	})
	var logged atomic.Value
	SetPeerLog(func(msg string) { logged.Store(msg) })
	t.Cleanup(func() { SetPeerLog(nil) })
	SetTokenOverride("the-service-token")
	t.Cleanup(func() { SetTokenOverride("") })

	c := NewClientAt("http://"+ln.Addr().String(), filepath.Join(t.TempDir(), "api-token"))
	code, err := c.ProbeStatus()
	var pe *ServicePeerError
	if code != 0 || !errors.As(err, &pe) || pe.Owner != "process 6666" {
		t.Fatalf("ProbeStatus = (%d, %v), want (0, a ServicePeerError)", code, err)
	}
	if !strings.Contains(err.Error(), "something other than the background service is answering on 127.0.0.1:"+port+" (process 6666): not connecting") {
		t.Errorf("error %q", err)
	}
	if err := c.CreateJob(map[string]interface{}{"id": "j", "pbsSecret": "s3cret"}); err == nil {
		t.Error("CreateJob reached an impostor")
	}
	if c.IsServiceAvailable() {
		t.Error("IsServiceAvailable with an impostor answering")
	}
	if checked.Load() < 3 {
		t.Errorf("the check ran %d times, want once per connection", checked.Load())
	}
	for i := 0; i < int(checked.Load()); i++ {
		select {
		case n := <-got:
			if n != 0 {
				t.Errorf("the impostor read %d bytes", n)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the impostor never saw the connection")
		}
	}
	if msg, _ := logged.Load().(string); !strings.Contains(msg, "process 6666") {
		t.Errorf("logged %q, want the refusal", msg)
	}
}

// The service itself passes: the check runs once for a kept-alive
// connection, and the request goes through with its token.
func TestDialAcceptsService(t *testing.T) {
	_, c := newTestService(t, &fakeHandler{})
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(c.baseURL, "http://"))
	var checked atomic.Int32
	asServicePort(t, port, func(conn net.Conn) error {
		checked.Add(1)
		if _, p, _ := net.SplitHostPort(conn.RemoteAddr().String()); p != port {
			t.Errorf("checked a connection to %s", conn.RemoteAddr())
		}
		return nil
	})
	for i := 0; i < 3; i++ {
		resp, err := c.httpClient.Get(c.baseURL + "/status")
		if err != nil {
			t.Fatalf("GET /status: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /status: %d, want 200 (token sent)", resp.StatusCode)
		}
	}
	if n := checked.Load(); n != 1 {
		t.Errorf("the check ran %d times for one kept-alive connection", n)
	}
}

// Other addresses (httptest servers) are not checked.
func TestOtherAddressNotChecked(t *testing.T) {
	old := SetServicePeerCheck(func(net.Conn) error { return errors.New("checked") })
	t.Cleanup(func() { SetServicePeerCheck(old) })
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer hs.Close()
	c := NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token"))
	if code, err := c.ProbeStatus(); code != http.StatusUnauthorized || err != nil {
		t.Fatalf("ProbeStatus = (%d, %v), want (401, nil)", code, err)
	}
}

// A check that fails without naming anyone is still a refusal.
func TestDialCheckErrorRefuses(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the request reached the server")
	}))
	defer hs.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(hs.URL, "http://"))
	asServicePort(t, port, func(net.Conn) error { return errNoServiceEnd })
	c := NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token"))
	code, err := c.ProbeStatus()
	var pe *ServicePeerError
	if code != 0 || !errors.As(err, &pe) || !errors.Is(err, errNoServiceEnd) {
		t.Fatalf("ProbeStatus = (%d, %v), want (0, a ServicePeerError)", code, err)
	}
	if !strings.Contains(err.Error(), "could not tell who is answering") {
		t.Errorf("error %q", err)
	}
}

// ServiceEndOwner waits a moment for the service's end to show up, then
// gives up.
func TestServiceEndOwnerWaitsThenGivesUp(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	server := conn.RemoteAddr().(*net.TCPAddr).AddrPort()
	client := conn.LocalAddr().(*net.TCPAddr).AddrPort()

	reads := 0
	owner, err := ServiceEndOwner(conn, func() ([]TCPRow, error) {
		reads++
		if reads < 3 {
			return nil, nil
		}
		return []TCPRow{{Local: server, Remote: client, Owner: 77}}, nil
	})
	if err != nil || owner != 77 || reads != 3 {
		t.Fatalf("owner %d, err %v after %d reads; want 77 on the third", owner, err, reads)
	}
	reads = 0
	if _, err := ServiceEndOwner(conn, func() ([]TCPRow, error) { reads++; return nil, nil }); !errors.Is(err, errNoServiceEnd) || reads != peerLookupTries {
		t.Fatalf("err %v after %d reads, want errNoServiceEnd after %d", err, reads, peerLookupTries)
	}
}
