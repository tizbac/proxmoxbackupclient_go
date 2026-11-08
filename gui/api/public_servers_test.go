package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// serversHandler is a service App that also gives the computer's servers
// (PublicServersAPI).
type serversHandler struct {
	fakeHandler
	list []PublicServer
}

func (h *serversHandler) PublicServersForAPI() []PublicServer { return h.list }

// GET /servers/public answers without a token, each server with exactly the
// eight fields; a login other than "password" goes out as "token".
func TestPublicServersAnswersWithoutToken(t *testing.T) {
	h := &serversHandler{list: []PublicServer{
		{ID: "pbs1", Name: "Office PBS", BaseURL: "https://pbs.example.com:8007", Datastore: "main",
			Namespace: "office", Fingerprint: "aa:bb", Login: "token", Encrypted: true},
		{ID: "pbs2", Name: "Offsite", BaseURL: "https://offsite.example.com:8007", Datastore: "dr", Login: "password"},
		{ID: "pbs3", Name: "Odd", BaseURL: "https://odd.example.com:8007", Datastore: "x", Login: "something else"},
	}}
	_, url, _ := newPublicService(t, h)
	resp, err := http.Get(url + PublicServersPath)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d without a token, want 200: %s", resp.StatusCode, body)
	}
	var got []map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("not a JSON list: %v: %s", err, body)
	}
	if len(got) != 3 {
		t.Fatalf("%d servers, want 3: %s", len(got), body)
	}
	want := "base_url,datastore,encrypted,fingerprint,id,login,name,namespace"
	for _, s := range got {
		keys := make([]string, 0, len(s))
		for k := range s {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if strings.Join(keys, ",") != want {
			t.Fatalf("keys %v, want exactly %s", keys, want)
		}
	}
	if got[0]["fingerprint"] != "aa:bb" || got[0]["encrypted"] != true || got[1]["login"] != "password" || got[2]["login"] != "token" {
		t.Fatalf("answer %s", body)
	}
}

// Without the handler's list the answer is an empty list, not null.
func TestPublicServersWithoutHandlerList(t *testing.T) {
	_, url, _ := newPublicService(t, &fakeHandler{})
	resp, err := http.Get(url + PublicServersPath)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("answer %q, want []", body)
	}
}

// Only GET is public: another method needs the token, and a browser's
// request (Origin) is refused.
func TestPublicServersOnlyGetIsPublic(t *testing.T) {
	_, url, _ := newPublicService(t, &serversHandler{})
	for _, tc := range []struct {
		method, path, origin string
		want                 int
	}{
		{http.MethodPost, PublicServersPath, "", http.StatusUnauthorized},
		{http.MethodDelete, PublicServersPath, "", http.StatusUnauthorized},
		{http.MethodGet, PublicServersPath + "/", "", http.StatusUnauthorized},
		{http.MethodGet, "/servers", "", http.StatusUnauthorized},
		{http.MethodGet, PublicServersPath, "http://evil.example", http.StatusForbidden},
	} {
		req, err := http.NewRequest(tc.method, url+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s (Origin %q) = %d, want %d", tc.method, tc.path, tc.origin, resp.StatusCode, tc.want)
		}
	}
}

// The client reads the list without sending a token, even while it holds
// one, and the 401 hook never runs.
func TestClientPublicServersSendsNoToken(t *testing.T) {
	h := &serversHandler{list: []PublicServer{{ID: "pbs1", Name: "Office PBS", BaseURL: "https://pbs.example.com:8007", Datastore: "main", Login: "token"}}}
	_, url, seen := newPublicService(t, h)
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	if _, err := EnsureToken(tokenPath); err != nil {
		t.Fatal(err)
	}
	c := NewClientAt(url, tokenPath)
	SetTokenOverride("a-fetched-token")
	hooked := false
	SetUnauthorizedHook(func(string) (string, error) { hooked = true; return "", nil })
	t.Cleanup(func() { SetTokenOverride(""); SetUnauthorizedHook(nil) })

	list, err := c.PublicServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0] != h.list[0] {
		t.Fatalf("list %+v", list)
	}
	for _, hdr := range *seen {
		if hdr.Get(tokenHeader) != "" {
			t.Fatal("the request carried a token")
		}
	}
	if hooked {
		t.Error("the unauthorized hook ran")
	}
}
