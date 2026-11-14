package api

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// The service's logs need no token. They are in the service's settings
// folder, which only the service and an administrator may read, and a person
// who is not an administrator still has to see why a backup failed and has to
// attach the logs to a bug report. So the service hands them out itself: the
// list, a part of one, the log of one run and a zip of them. Only log files
// are served (the names classifyLog in the app admits), never the settings,
// the token or a key, and the settings folder stays closed.
//
// What that opens: every account of the computer can read what the service
// logs, which names the folders and files it backs up, the servers and
// datastores, and the login names (never a secret: the log calls mask those,
// and a Backup Set's before and after commands are not logged at all).
//
// Two things keep it to the computer's own programs. The service listens on
// 127.0.0.1 only, and a request without the token must be addressed to it by
// that name (loopbackHost): a web page that points its own host name at
// 127.0.0.1 (DNS rebinding) sends that name as Host, and is refused. And the
// log requests are served one at a time with a limit on how long an answer
// may take to send, so asking for them in a loop cannot fill the service's
// memory.

// The log routes (server_state.go).
const (
	logsPath       = "/logs"
	logsReadPath   = "/logs/read"
	logsRunPath    = "/logs/run"
	logsBundlePath = "/logs/bundle"
)

// logAnswerTimeout is how long one log answer may take to build and send.
const logAnswerTimeout = 2 * time.Minute

// isPublicLogRequest reports whether r is one of the four log requests.
func isPublicLogRequest(r *http.Request) bool {
	switch r.URL.Path {
	case logsPath, logsRunPath:
		return r.Method == http.MethodGet
	case logsReadPath, logsBundlePath:
		return r.Method == http.MethodPost
	}
	return false
}

// loopbackHost reports whether a request's Host header names this computer
// itself: 127.0.0.1 (any 127.x), ::1 or localhost, with or without a port.
func loopbackHost(hostport string) bool {
	host := hostport
	if h, port, err := net.SplitHostPort(hostport); err == nil {
		// SplitHostPort does not check the port: "127.0.0.1:1@x" is refused here.
		if strings.Trim(port, "0123456789") != "" {
			return false
		}
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// oneLogRequest runs serve, which answers a log request, while no other log
// request is being answered, and gives the answer logAnswerTimeout to reach
// the caller: a caller that asks and then does not read cannot hold the next
// one up for longer, and at most one bundle is in memory at a time.
func (s *Server) oneLogRequest(w http.ResponseWriter, serve func()) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(logAnswerTimeout))
	serve()
	// Send what is left of the answer under the limit, then lift the limit:
	// it is the connection's, and the next request on it is not a log's.
	_ = rc.Flush()
	_ = rc.SetWriteDeadline(time.Time{})
}
