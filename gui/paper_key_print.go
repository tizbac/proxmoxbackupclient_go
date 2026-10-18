package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"
)

// Printing from the app's own window does not work everywhere: WebView2
// cannot print while the app runs as administrator (its preview opens, but
// the job fails with "Print failed": the step that hands the page to the
// printer runs in a helper process that refuses an elevated parent). There
// the paper key is printed from the user's web browser instead, started
// without administrator rights. The page is served once from this process
// on 127.0.0.1 at a random address, so the key is never written to a file.

// paperKeyServeWindow is how long the page waits to be fetched.
const paperKeyServeWindow = 3 * time.Minute

// paperKeyPrintsInBrowser reports whether a paper key is printed through the
// web browser rather than the app's window.
func paperKeyPrintsInBrowser() bool {
	return stdruntime.GOOS == "windows" && isAdmin()
}

// PrintPaperKeyInBrowser opens page (the paper key laid out for printing,
// from the frontend) in the user's web browser, which prints it. The page is
// served once: the address stops working after the first fetch, or after
// paperKeyServeWindow if nothing fetches it.
func (a *App) PrintPaperKeyInBrowser(page string) error {
	if !strings.Contains(page, "<html") {
		return fmt.Errorf("not a page")
	}
	url, err := servePageOnce(page, paperKeyServeWindow)
	if err != nil {
		return err
	}
	if err := openInUserBrowser(a.ctx, url); err != nil {
		return fmt.Errorf("opening the web browser: %w", err)
	}
	writeDebugLog("PrintPaperKeyInBrowser: paper key opened in the web browser to print")
	return nil
}

// servePageOnce serves page at a random path on 127.0.0.1 and returns its
// URL. The listener closes after the page has been fetched once, or after
// window. Nothing is cached: the response says no-store.
func servePageOnce(page string, window time.Duration) (string, error) {
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		return "", err
	}
	path := "/" + hex.EncodeToString(tok)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
			}()
		})
	}
	var served sync.Once
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		first := false
		served.Do(func() { first = true })
		if !first {
			http.Error(w, "This page has already been opened. Print the paper key again from the app.", http.StatusGone)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		_, _ = w.Write([]byte(page))
		stop()
	})
	go func() { _ = srv.Serve(ln) }()
	time.AfterFunc(window, stop)
	return "http://" + ln.Addr().String() + path, nil
}
