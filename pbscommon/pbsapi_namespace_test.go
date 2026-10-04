package pbscommon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression guard for the namespace parameter.
//
// `ns` must be sent to the server whenever a namespace is configured. A
// version probe used to suppress it on everything newer than PBS 2.x, on the
// (incorrect) assumption that PBS 3.x had dropped namespaces; PBS 4.x does
// support them, they only have to exist on the server beforehand. The probe is
// gone, and these tests pin the behaviour so it cannot silently regress again.
//
// Verified against PBS 4.2.0:
//	ns=""                  -> 101 Switching Protocols
//	ns=<existing>          -> 101 Switching Protocols
//	ns=<missing>           -> 404 "namespace not found"

// newFakeSnapshotsServer answers the snapshot listing endpoint with an empty
// but well-formed PBS envelope and records the query it was called with.
func newFakeSnapshotsServer(t *testing.T) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var captured []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = append(captured, capturedRequest{
			path:  r.URL.Path,
			query: r.URL.Query(),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func TestListSnapshotsSendsNamespace(t *testing.T) {
	srv, captured := newFakeSnapshotsServer(t)
	pbs := newTestClient(srv.URL, nil)
	pbs.Namespace = "myteam"

	if _, err := pbs.ListSnapshots(); err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("expected exactly 1 request, got %d", len(*captured))
	}
	if got := (*captured)[0].query.Get("ns"); got != "myteam" {
		t.Errorf("ns query parameter = %q, want %q", got, "myteam")
	}
	// The datastore is part of the path, not the query string.
	if want := "/api2/json/admin/datastore/teststore/snapshots"; (*captured)[0].path != want {
		t.Errorf("request path = %q, want %q", (*captured)[0].path, want)
	}
}

func TestListSnapshotsOmitsEmptyNamespace(t *testing.T) {
	srv, captured := newFakeSnapshotsServer(t)
	pbs := newTestClient(srv.URL, nil)
	pbs.Namespace = ""

	if _, err := pbs.ListSnapshots(); err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("expected exactly 1 request, got %d", len(*captured))
	}
	// An empty `ns` is legal server-side (it means the root namespace), but
	// sending it is meaningless noise.
	if _, present := (*captured)[0].query["ns"]; present {
		t.Errorf("ns should be omitted when Namespace is empty, got %v", (*captured)[0].query)
	}
}

func TestNamespaceNotFoundDetection(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"namespace not found", true},
		{"namespace not found: myteam", true},
		{"NAMESPACE NOT FOUND", true},
		{"404 Not Found", false},
		{"permission check failed", false},
		{"authentication failed", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := namespaceNotFound(tc.body); got != tc.want {
			t.Errorf("namespaceNotFound(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}

// The server reports a missing namespace as a 404 on the HTTP/2 upgrade
// handshake. Without special handling that surfaces as an *AuthErr*, which
// reads like a credential problem; it must be an actionable error instead.
func TestNamespaceNotFoundErrIsActionable(t *testing.T) {
	err := &NamespaceNotFoundErr{Namespace: "myteam", Datastore: "store1"}
	msg := err.Error()
	for _, want := range []string{
		`"myteam"`, // names the namespace
		`"store1"`, // names the datastore
		"/api2/json/admin/datastore/store1/namespace", // the endpoint
		"name=myteam", // the parameter name
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message is missing %q\nfull message: %s", want, msg)
		}
	}
}

func TestNamespaceNotFoundNotMaskingOtherErrors(t *testing.T) {
	// A 404 for some other reason must still be an AuthErr.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	pbs := newTestClient(srv.URL, nil)
	pbs.Namespace = "myteam"
	if _, err := pbs.ListSnapshots(); err == nil {
		t.Fatal("expected an error")
	} else if _, ok := err.(*NamespaceNotFoundErr); ok {
		t.Errorf("a 401 must not be reported as a namespace problem: %v", err)
	}
}
