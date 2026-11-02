package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
)

// Whether the background service answers at all. With the service installed
// the app works on its settings, and a stopped service looks, to a request,
// much like a refused token: the window must tell the two apart (the service
// is not running, so this computer's Backup Sets do not run; or it runs and
// the app has to Connect). Every request a Client sends records what it
// found:
//
//   - an HTTP answer, whatever its status: the service answers (a connection
//     to the service's address is only used once the peer check has found
//     the service at its other end, peer.go);
//   - no answer: the connection was refused or could not be made (nothing
//     listens: the service is stopped), or the peer check refused who holds
//     the port (something other than the service answers there, which is
//     treated as the service not running);
//   - anything else (a slow answer, a request cancelled): no change.
//
// The app's window learns of a change through the hook (SetAnswerHook). The
// app lists the service's runs every 3 seconds while it works with the
// service, so a stop is noticed within a few seconds.

const (
	answerUnknown int32 = iota // nothing sent yet
	answerYes
	answerNo
)

type answerState struct {
	state  atomic.Int32
	hookMu sync.RWMutex
	hook   func(answers bool, err error)
}

// IsNoAnswer reports whether err, the error of a request to the service,
// means that nothing that may be the service answered: the connection was
// refused or could not be made, or the peer check refused it.
func IsNoAnswer(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var pe *ServicePeerError
	if errors.As(err, &pe) {
		return true
	}
	var op *net.OpError
	return errors.As(err, &op) && op.Op == "dial"
}

// Answering reports whether the service answered the last request this
// client sent (known is false until one has been sent).
func (c *Client) Answering() (answers, known bool) {
	switch c.answer.state.Load() {
	case answerYes:
		return true, true
	case answerNo:
		return false, true
	}
	return false, false
}

// SetAnswerHook installs (nil removes) the function called when the service
// starts or stops answering: answers is the new state, err why it does not
// answer (nil when it does). Not called for the first request's finding,
// only for a change after it. It runs on the request's goroutine and must
// not block.
func (c *Client) SetAnswerHook(hook func(answers bool, err error)) {
	c.answer.hookMu.Lock()
	c.answer.hook = hook
	c.answer.hookMu.Unlock()
}

// noteAnswer records what one request found (err: its error).
func (c *Client) noteAnswer(err error) {
	next := answerYes
	if err != nil {
		if !IsNoAnswer(err) {
			return
		}
		next = answerNo
	}
	prev := c.answer.state.Swap(next)
	if prev == next || prev == answerUnknown {
		return
	}
	c.answer.hookMu.RLock()
	hook := c.answer.hook
	c.answer.hookMu.RUnlock()
	if hook != nil {
		hook(next == answerYes, err)
	}
}

// answerTransport sends a request and records whether anything answered.
type answerTransport struct {
	c    *Client
	base http.RoundTripper
}

func (t *answerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	t.c.noteAnswer(err)
	return resp, err
}
