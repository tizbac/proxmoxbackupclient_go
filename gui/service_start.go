//go:build !service

package main

// "Start the service", the button on the window's banner when the background
// service is installed but not running (service_connect.go notices that).
//
// Starting a service needs an administrator. An app run without elevation
// starts a copy of itself elevated (UAC on Windows, pkexec on Linux) with
// --elevated-service-start and a handoff file, as for the token fetch
// (token_elevated.go): the copy starts the service, waits until it answers
// with its new token, writes that token into the handoff file and exits. So
// one prompt both starts the service and connects the app to it. An app
// already elevated (or run as root) starts it itself and reads the token
// file directly.

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// elevatedServiceStartFlag marks the elevated copy that starts the service.
const elevatedServiceStartFlag = "--elevated-service-start"

// serviceAnswerWait bounds the wait for a started service to answer with
// its new token; elevatedServiceStartTimeout the wait for the elevated copy,
// which also waits for the service to start (up to 30 s).
const (
	serviceAnswerWait           = 30 * time.Second
	elevatedServiceStartTimeout = 90 * time.Second
)

// errElevationCancelled is how the window learns that the prompt was
// declined (as for RequestElevation).
var errElevationCancelled = errors.New("ELEVATION_CANCELLED")

// handleElevatedServiceStartChild returns false, or exits the process when
// this is the elevated copy that starts the service. main() calls it first.
func handleElevatedServiceStartChild(args []string) bool {
	for i, a := range args {
		if a != elevatedServiceStartFlag || i+1 >= len(args) {
			continue
		}
		handoff := args[i+1]
		if handoff == "" {
			os.Exit(exitChildBadArgument)
		}
		writeDebugLog("[ElevatedServiceStart] child: starting the background service")
		if err := startInstalledService(); err != nil {
			writeDebugLog(fmt.Sprintf("[ElevatedServiceStart] child: the service did not start: %v", err))
			os.Exit(exitServiceNotStarted)
		}
		token := waitForServiceToken(api.NewClient(getAPITokenPath()), serviceAnswerWait)
		if token == "" {
			writeDebugLog("[ElevatedServiceStart] child: the service started but did not take the token in its file")
			os.Exit(exitTokenFileUnread)
		}
		os.Exit(writeTokenHandoff("[ElevatedServiceStart]", handoff, token))
	}
	return false
}

// waitForServiceToken waits until the service, just started, takes the token
// in its token file (the service writes a new one when it starts, so the old
// file's token is refused until then), and returns it; "" after timeout.
// Only an administrator's process can read the file.
func waitForServiceToken(c *api.Client, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for {
		if b, err := os.ReadFile(getAPITokenPath()); err == nil {
			if token := strings.TrimSpace(string(b)); token != "" {
				api.SetTokenOverride(token)
				if code, _ := c.ProbeStatus(); code == http.StatusOK {
					return token
				}
			}
		}
		if time.Now().After(deadline) {
			return ""
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// waitForServiceAnswer waits until the service answers at all (200 or 401).
func (a *App) waitForServiceAnswer(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if code, _ := a.apiClient.ProbeStatus(); code != 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// StartServiceAndConnect is the banner's "Start the service": it starts the
// installed background service (asking for administrator rights unless the
// app has them) and connects this app to it. Returns nil once the service
// runs (connected, or, if the token could not be handed over, showing
// Connect); "ELEVATION_CANCELLED" when the prompt was declined; otherwise why
// the service did not start.
func (a *App) StartServiceAndConnect() error {
	if a.mode != api.ModeService || a.ownSettings.Load() || a.apiClient == nil {
		return errors.New("the app is not working with the background service")
	}
	if code, _ := a.apiClient.ProbeStatus(); code != 0 {
		// It runs (started elsewhere meanwhile): Connect as usual.
		writeDebugLog("[Service] Start the service: it already answers; connecting")
		return a.connect(true, nil)
	}
	if needAdmin() == nil {
		// Elevated already, or root: no prompt.
		writeDebugLog("[Service] Start the service: starting it (this app is elevated)")
		if err := startInstalledService(); err != nil {
			writeDebugLog(fmt.Sprintf("[Service] the service did not start: %v", err))
			return fmt.Errorf("the background service did not start: %w", err)
		}
		if !a.waitForServiceAnswer(serviceAnswerWait) {
			return fmt.Errorf("the background service started but did not answer within %s", serviceAnswerWait)
		}
		return a.connect(true, nil)
	}

	a.tokenFetchMu.Lock()
	defer a.tokenFetchMu.Unlock()
	handoff, err := newHandoffFile()
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(handoff) }()
	writeDebugLog("[Service] Start the service: asking for administrator rights to start it")
	token, err := runElevatedHandoff(elevatedServiceStartFlag, handoff, elevatedServiceStartTimeout)
	if err != nil {
		if errors.Is(err, ErrElevationDeclined) {
			writeDebugLog("[Service] Start the service: the prompt was declined")
			return errElevationCancelled
		}
		writeDebugLog(fmt.Sprintf("[Service] Start the service: %v", err))
		var ce *elevatedChildError
		if errors.As(err, &ce) && ce.code != exitServiceNotStarted && a.waitForServiceAnswer(5*time.Second) {
			// Started, but the token was not handed over: the window shows
			// Connect.
			return nil
		}
		return err
	}
	writeDebugLog("[Service] the background service started; connecting with its token")
	return a.useServiceToken(token, nil)
}
