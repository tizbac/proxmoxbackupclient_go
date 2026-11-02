package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// gone reports whether nothing answers at url any more (the listener is
// closed), waiting up to a second for the shutdown.
func gone(url string) bool {
	for i := 0; i < 20; i++ {
		r, err := http.Get(url)
		if err != nil {
			return true
		}
		r.Body.Close()
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// The paper key page is served once, at its random path only, with
// no-store and a policy that loads nothing from elsewhere, and the
// listener closes afterwards.
func TestServePageOnce(t *testing.T) {
	page := "<!doctype html><html><body>paper key</body></html>"
	url, _, err := servePageOnce(page, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") || len(url[strings.LastIndex(url, "/")+1:]) != 32 {
		t.Fatalf("url %q: want 127.0.0.1 and a 128-bit random path", url)
	}
	base := url[:strings.LastIndex(url, "/")]

	r, err := http.Get(base + "/nothing")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Fatalf("other path: %d, want 404", r.StatusCode)
	}

	r, err = http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK || string(body) != page {
		t.Fatalf("first fetch: %d %q", r.StatusCode, body)
	}
	if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control %q, want no-store", cc)
	}
	if csp := r.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("Content-Security-Policy %q", csp)
	}
	if !gone(url) {
		t.Fatal("still listening after the page was fetched")
	}
}

// Nothing fetches the page: the listener closes when the window ends.
func TestServePageOnceExpires(t *testing.T) {
	url, _, err := servePageOnce("<html></html>", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if !gone(url) {
		t.Fatal("still listening after the window")
	}
}

// stop (the browser could not be opened) closes the listener at once.
func TestServePageOnceStop(t *testing.T) {
	url, stop, err := servePageOnce("<html></html>", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	if !gone(url) {
		t.Fatal("still listening after stop")
	}
}

// Only an HTML page is accepted.
func TestPrintPaperKeyInBrowserRefusesNonPage(t *testing.T) {
	if err := (&App{}).PrintPaperKeyInBrowser("just text"); err == nil {
		t.Fatal("accepted something that is not a page")
	}
}
