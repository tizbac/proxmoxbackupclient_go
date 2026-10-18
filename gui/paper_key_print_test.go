package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The paper key page is served once, at its random path only, with
// no-store, and the address is gone afterwards.
func TestServePageOnce(t *testing.T) {
	page := "<!doctype html><html><body>paper key</body></html>"
	url, err := servePageOnce(page, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") || len(url[strings.LastIndex(url, "/")+1:]) != 32 {
		t.Fatalf("url %q: want 127.0.0.1 and a 128-bit random path", url)
	}
	base := url[:strings.LastIndex(url, "/")]

	if r, err := http.Get(base + "/nothing"); err != nil || r.StatusCode != http.StatusNotFound {
		t.Fatalf("other path: %v %v, want 404", r, err)
	}

	r, err := http.Get(url)
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

	time.Sleep(200 * time.Millisecond)
	if r, err := http.Get(url); err == nil {
		r.Body.Close()
		if r.StatusCode == http.StatusOK {
			t.Fatal("served a second time")
		}
	}
}

// Nothing fetches the page: the address closes when the window ends.
func TestServePageOnceExpires(t *testing.T) {
	url, err := servePageOnce("<html></html>", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if r, err := http.Get(url); err == nil {
		r.Body.Close()
		t.Fatalf("still served after the window: %d", r.StatusCode)
	}
}

// Only an HTML page is accepted.
func TestPrintPaperKeyInBrowserRefusesNonPage(t *testing.T) {
	if err := (&App{}).PrintPaperKeyInBrowser("just text"); err == nil {
		t.Fatal("accepted something that is not a page")
	}
}
