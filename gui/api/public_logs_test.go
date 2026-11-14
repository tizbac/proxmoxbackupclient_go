package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// askLogs sends one request to a test service with no token, as host when
// that is given, and returns the status and the body.
func askLogs(t *testing.T, method, url, body, host string) (int, string) {
	t.Helper()
	var in io.Reader
	if body != "" {
		in = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, in)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode, string(b)
}

// The four log requests answer without a token.
func TestLogsAnswerWithoutToken(t *testing.T) {
	_, url, seen := newPublicService(t, &stateHandler{})
	for _, tc := range []struct {
		method, path, body, want string
	}{
		{http.MethodGet, logsPath, "", `"name":"service.log"`},
		{http.MethodPost, logsReadPath, `{"name":"service.log"}`, `"query":{"name":"service.log"}`},
		{http.MethodGet, logsRunPath + "?backup_id=pc-docs&at=42", "", `"name":"pc-docs"`},
		{http.MethodPost, logsBundlePath, `{"days":7}`, `"data":`},
	} {
		code, body := askLogs(t, tc.method, url+tc.path, tc.body, "")
		if code != http.StatusOK || !strings.Contains(body, tc.want) {
			t.Errorf("%s %s without a token = %d %s, want 200 with %s", tc.method, tc.path, code, body, tc.want)
		}
	}
	for _, hdr := range *seen {
		if hdr.Get(tokenHeader) != "" {
			t.Fatal("test setup: a request carried a token")
		}
	}
	var bundle LogBundleResponse
	_, body := askLogs(t, http.MethodPost, url+logsBundlePath, `{"days":7}`, "")
	if err := json.Unmarshal([]byte(body), &bundle); err != nil || string(bundle.Data) != "zip-of-7-days" {
		t.Fatalf("the bundle without a token: %q, %v", bundle.Data, err)
	}
}

// Only the four log requests are open: another method on them, another path
// under /logs and everything else the service does still need the token.
func TestOnlyTheLogRequestsNeedNoToken(t *testing.T) {
	_, url, _ := newPublicService(t, &stateHandler{doc: map[string]interface{}{"default_pbs_id": "pbs1"}})
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, logsPath},
		{http.MethodDelete, logsPath},
		{http.MethodGet, logsPath + "/"},
		{http.MethodGet, logsReadPath},
		{http.MethodPost, logsRunPath},
		{http.MethodGet, logsBundlePath},
		{http.MethodGet, logsPath + "/../config"},
		{http.MethodGet, "/config"},
		{http.MethodGet, "/status"},
		{http.MethodGet, "/jobs"},
		{http.MethodGet, "/settings/export"},
		{http.MethodPost, "/settings/export"},
		{http.MethodPost, "/keys/save"},
		{http.MethodPost, "/pbs/access"},
		{http.MethodPost, "/call"},
	} {
		code, body := askLogs(t, tc.method, url+tc.path, "", "")
		if code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token = %d %s, want 401", tc.method, tc.path, code, body)
		}
	}
}

// A request without the token must name this computer as its Host: a web page
// whose host name was pointed at 127.0.0.1 sends that name, and is refused.
// The same holds for the public status and the public server list. With the
// token the Host does not matter (a script behind a forwarder).
func TestRequestsWithoutTokenMustBeAddressedToThisComputer(t *testing.T) {
	s, url, _ := newPublicService(t, &stateHandler{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, logsPath, ""},
		{http.MethodPost, logsReadPath, `{"name":"service.log"}`},
		{http.MethodGet, logsRunPath + "?backup_id=pc-docs&at=42", ""},
		{http.MethodPost, logsBundlePath, `{}`},
		{http.MethodGet, PublicStatusPath, ""},
		{http.MethodGet, PublicServersPath, ""},
	} {
		for _, host := range []string{"evil.example", "evil.example:18765", "127.0.0.1.evil.example:18765", "192.168.0.10:18765", "localhost.evil.example"} {
			code, body := askLogs(t, tc.method, url+tc.path, tc.body, host)
			if code != http.StatusForbidden {
				t.Errorf("%s %s as Host %q = %d %s, want 403", tc.method, tc.path, host, code, body)
			}
		}
		for _, host := range []string{"127.0.0.1:18765", "localhost:18765", "LOCALHOST", "[::1]:18765"} {
			if code, body := askLogs(t, tc.method, url+tc.path, tc.body, host); code != http.StatusOK {
				t.Errorf("%s %s as Host %q = %d %s, want 200", tc.method, tc.path, host, code, body)
			}
		}
	}

	// A browser's request (Origin) is refused whatever its Host.
	req, err := http.NewRequest(http.MethodGet, url+logsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://127.0.0.1:18765")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a request with an Origin = %d, want 403", resp.StatusCode)
	}

	// With the token, under another host name.
	req, err = http.NewRequest(http.MethodGet, url+logsPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "backup-pc.example:18765"
	req.Header.Set(tokenHeader, s.token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("with the token under another host name = %d, want 200", resp.StatusCode)
	}
}

func TestLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1":                             true,
		"127.0.0.1:18765":                       true,
		"127.8.9.10:80":                         true,
		"localhost":                             true,
		"Localhost:18765":                       true,
		"[::1]":                                 true,
		"[::1]:18765":                           true,
		"::1":                                   true,
		"":                                      false,
		"example.com":                           false,
		"example.com:18765":                     false,
		"127.0.0.1.example.com":                 false,
		"localhost.example.com:18765":           false,
		"192.168.0.10:18765":                    false,
		"0.0.0.0:18765":                         false,
		"[2001:db8::1]:18765":                   false,
		"127.0.0.1:18765@example.com":           false,
		"example.com#127.0.0.1":                 false,
		"localhost:18765:example.com":           false,
		"notlocalhost":                          false,
		"127.0.0.1 example.com":                 false,
		"[::ffff:192.168.0.10]:18765":           false,
		"[::ffff:127.0.0.1]:18765":              true,
		"0x7f.0.0.1":                            false,
		"2130706433":                            false,
		"localhost.":                            false,
		"127.1":                                 false,
		"[::1]extra":                            false,
		"::1:18765":                             false,
		"[fe80::1%25eth0]:18765":                false,
		"local" + string(rune(0x200b)) + "host": false,
		"ｌocalhost":                             false,
		"LOCALHOST:1":                           true,
		"127.0.0.1:":                            true,
		"[::1]:":                                true,
		"127.0.0.1:18765/":                      false,
		"127.0.0.1:18765,example.com":           false,
		"example.com, 127.0.0.1:18765":          false,
	} {
		if got := loopbackHost(host); got != want {
			t.Errorf("loopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// An app that holds no token reads the logs through the client, and reading
// them never runs the 401 hook: no elevation prompt for a log.
func TestClientReadsLogsWithoutTokenAndNeverAsksForOne(t *testing.T) {
	_, url, seen := newPublicService(t, &stateHandler{})
	c := NewClientAt(url, filepath.Join(t.TempDir(), "no-token"))
	hooked := false
	SetUnauthorizedHook(func(string) (string, error) { hooked = true; return "", nil })
	t.Cleanup(func() { SetUnauthorizedHook(nil) })

	var list []map[string]interface{}
	if err := c.ListLogs(&list); err != nil || len(list) != 1 {
		t.Fatalf("ListLogs %v, %v", list, err)
	}
	var view map[string]interface{}
	if err := c.ReadLog(map[string]interface{}{"name": "service.log"}, &view); err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	var run map[string]interface{}
	if err := c.FindRunLog("pc-docs", 42, &run); err != nil || run["name"] != "pc-docs" {
		t.Fatalf("FindRunLog %v, %v", run, err)
	}
	if data, err := c.LogBundle(7); err != nil || string(data) != "zip-of-7-days" {
		t.Fatalf("LogBundle %q, %v", data, err)
	}
	for _, hdr := range *seen {
		if hdr.Get(tokenHeader) != "" {
			t.Fatal("test setup: a request carried a token")
		}
	}
	// The rest of the service stays shut to it, and that is where the hook is for.
	if _, err := c.GetFullConfig(); err == nil {
		t.Fatal("the settings were given without a token")
	}
	if !hooked {
		t.Fatal("test setup: the 401 hook did not run for the settings")
	}
	hooked = false
	if err := c.ListLogs(&list); err != nil {
		t.Fatal(err)
	}
	if hooked {
		t.Error("reading the logs ran the 401 hook")
	}
}

// slowLogs is a service App whose log list takes a while, and counts how many
// log answers are being built at once.
type slowLogs struct {
	stateHandler
	now, most atomic.Int32
}

func (h *slowLogs) busy() func() {
	n := h.now.Add(1)
	for {
		m := h.most.Load()
		if n <= m || h.most.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(30 * time.Millisecond)
	return func() { h.now.Add(-1) }
}

func (h *slowLogs) ListLogsForAPI() (json.RawMessage, error) {
	defer h.busy()()
	return json.RawMessage(`[]`), nil
}

func (h *slowLogs) LogBundleForAPI(days int) ([]byte, error) {
	defer h.busy()()
	return []byte("zip"), nil
}

// Log requests are answered one at a time: asking in a loop builds one answer
// at a time, so at most one bundle is in the service's memory.
func TestLogRequestsAreAnsweredOneAtATime(t *testing.T) {
	h := &slowLogs{}
	_, url, _ := newPublicService(t, h)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			method, path, body := http.MethodGet, logsPath, ""
			if i%2 == 1 {
				method, path, body = http.MethodPost, logsBundlePath, `{}`
			}
			var in io.Reader
			if body != "" {
				in = strings.NewReader(body)
			}
			req, err := http.NewRequest(method, url+path, in)
			if err != nil {
				t.Error(err)
				return
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s %s = %d", method, path, resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	if most := h.most.Load(); most != 1 {
		t.Fatalf("%d log answers were built at once, want 1", most)
	}
}
