package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// The computer's PBS servers, without their logins: the second route that
// needs no token. A user who works on their own settings (Use my own
// settings) holds no token: they could not, or chose not to, answer the
// administrator prompt. They may still back up to a server the computer
// already has, with their own PBS token or login, instead of typing its
// address, datastore and fingerprint again. So the answer holds where each
// server is and how it is reached, and nothing that lets anyone log in to
// it: no token ID, no username, no secret, no password, no key file.
//
// The peer check (peer.go) applies to it as to every route: the app reads it
// only from the real service.

// PublicServersPath is the route of the computer's servers (GET only).
const PublicServersPath = "/servers/public"

// PublicServer is one of the computer's PBS servers as GET /servers/public
// gives it. Every field is always present.
type PublicServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	Datastore   string `json:"datastore"`
	Namespace   string `json:"namespace"`
	Fingerprint string `json:"fingerprint"`
	// Login is how the computer logs in to it: "token" (an API token) or
	// "password" (a username and password). The user enters their own.
	Login string `json:"login"`
	// Encrypted: the computer's backups to it are encrypted. The key is
	// not shared; the user chooses their own.
	Encrypted bool `json:"encrypted"`
}

// PublicServersAPI is optionally implemented by the BackupHandler: the
// computer's PBS servers for GET /servers/public. Without it the list is
// empty.
type PublicServersAPI interface {
	PublicServersForAPI() []PublicServer
}

// handlePublicServers serves GET /servers/public.
func (s *Server) handlePublicServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list := []PublicServer{}
	if p, ok := s.app.(PublicServersAPI); ok {
		// Copied field by field: whatever the handler gives, only these go out.
		for _, sv := range p.PublicServersForAPI() {
			login := "token"
			if sv.Login == "password" {
				login = "password"
			}
			list = append(list, PublicServer{
				ID: sv.ID, Name: sv.Name, BaseURL: sv.BaseURL, Datastore: sv.Datastore,
				Namespace: sv.Namespace, Fingerprint: sv.Fingerprint, Login: login, Encrypted: sv.Encrypted,
			})
		}
	}
	s.writeJSON(w, list, http.StatusOK)
}

// PublicServers reads GET /servers/public. Like PublicStatus it sends no
// token and never runs the unauthorized hook; the peer check applies. A
// service older than the route answers 401: an error.
func (c *Client) PublicServers() ([]PublicServer, error) {
	client := &http.Client{Timeout: ConnectionTimeout, Transport: c.public}
	resp, err := client.Get(c.baseURL + PublicServersPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}
	var list []PublicServer
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return list, nil
}
