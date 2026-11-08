//go:build (linux || windows) && !service

package main

// Opt-in live test of the check of who answers on the service's port
// (gui/api/peer.go), on a test machine with the service installed:
//
//   PBS_PEER_LIVE=service   the service runs: the app's client connects
//                           (200, or 401 without the token).
//   PBS_PEER_LIVE=squatter  the service is stopped and another program
//                           listens on 127.0.0.1:18765: the client refuses
//                           it before sending anything.

import (
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

func TestLiveServicePeer(t *testing.T) {
	step := os.Getenv("PBS_PEER_LIVE")
	if step == "" {
		t.Skip("set PBS_PEER_LIVE=service or squatter")
	}
	c := api.NewClient(getAPITokenPath())
	code, err := c.ProbeStatus()
	t.Logf("ProbeStatus: %d, %v", code, err)
	var pe *api.ServicePeerError
	switch step {
	case "service":
		if err != nil || (code != http.StatusOK && code != http.StatusUnauthorized) {
			t.Fatalf("the service: (%d, %v), want 200 or 401", code, err)
		}
	case "squatter":
		if code != 0 || !errors.As(err, &pe) || pe.Owner == "" {
			t.Fatalf("a squatter: (%d, %v), want a refusal naming it", code, err)
		}
	default:
		t.Fatalf("PBS_PEER_LIVE=%q", step)
	}
}
