package api

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serveWithLimits serves s on a free port of 127.0.0.1 with the API's own
// HTTP server (Start's), the time limits shortened, and returns its address.
func serveWithLimits(t *testing.T, header, idle time.Duration) string {
	t.Helper()
	oldH, oldR, oldI := apiReadHeaderTimeout, apiReadTimeout, apiIdleTimeout
	apiReadHeaderTimeout, apiReadTimeout, apiIdleTimeout = header, 2*header, idle
	t.Cleanup(func() { apiReadHeaderTimeout, apiReadTimeout, apiIdleTimeout = oldH, oldR, oldI })
	tok, err := EnsureToken(filepath.Join(t.TempDir(), "api-token"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer("127.0.0.1:0", &stateHandler{}, tok, "test")
	srv := s.httpServer()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// closedWithin reports whether the server closes conn within d, reading
// whatever it sends first (a 408 answer, say).
func closedWithin(t *testing.T, conn net.Conn, d time.Duration) bool {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(d))
	_, err := io.Copy(io.Discard, conn)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return false
	}
	return true
}

func TestAPIServerHasTimeLimits(t *testing.T) {
	srv := (&Server{addr: "127.0.0.1:18765", mux: http.NewServeMux()}).httpServer()
	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.IdleTimeout <= 0 || srv.MaxHeaderBytes <= 0 {
		t.Fatalf("limits %v %v %v %d, want all set", srv.ReadHeaderTimeout, srv.ReadTimeout, srv.IdleTimeout, srv.MaxHeaderBytes)
	}
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout %v: a connection test waits on a PBS, and the log answers set their own", srv.WriteTimeout)
	}
}

// A program that starts a request and never finishes its headers is cut off.
func TestAPIServerCutsOffAnUnfinishedRequest(t *testing.T) {
	addr := serveWithLimits(t, 300*time.Millisecond, time.Minute)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("GET /status/public HTTP/1.1\r\nHost: 127.0.0.1\r\n")); err != nil {
		t.Fatal(err)
	}
	if !closedWithin(t, conn, 5*time.Second) {
		t.Fatal("a request whose headers never ended kept its connection")
	}
}

// A connection left open after an answer is closed once it has been idle.
func TestAPIServerClosesAnIdleConnection(t *testing.T) {
	addr := serveWithLimits(t, 5*time.Second, 300*time.Millisecond)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("GET /status/public HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !closedWithin(t, conn, 5*time.Second) {
		t.Fatal("an idle connection was kept open")
	}
}

// Headers far beyond any the API uses are refused.
func TestAPIServerRefusesHugeHeaders(t *testing.T) {
	addr := serveWithLimits(t, 5*time.Second, time.Minute)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	req := "GET /status/public HTTP/1.1\r\nHost: 127.0.0.1\r\nX-Pad: " + strings.Repeat("a", 2*apiMaxHeaderBytes) + "\r\n\r\n"
	_, _ = conn.Write([]byte(req))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return // closed without an answer: refused too
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status %d, want 431", resp.StatusCode)
	}
}
