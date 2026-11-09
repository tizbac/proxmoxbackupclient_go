package retry

import (
	"errors"
	"testing"
)

func TestReconnectable(t *testing.T) {
	for _, tc := range []struct {
		err  string
		want bool
	}{
		// A failure of 2026-10-24 on a test machine, as Backup returned it.
		{`backup device /dev/sdb: failed to upload chunk fb73...: Post "https://192.168.1.10:8007/fixed_chunk?...": http2: client connection lost`, true},
		{"dial tcp 192.168.1.10:8007: connect: no route to host", true},
		{"dial tcp 192.168.1.10:8007: i/o timeout", true},
		{"Error making request: unable to acquire lock on backup group directory - another backup is already running", true},
		{"read /dev/elastio-snap1 at segment 1048576: failed to read block: input/output error", false},
		{"no usable block snapshot module loaded", false},
		{"HTTP 401 - permission check failed", false},
		// A session PBS refused for want of a permission (issue 136): trying
		// again changes nothing, and it must not read as a lost connection.
		{`backup device /dev/sdb: Post "https://pbs:8007/fixed_index": PBS refused the backup session: HTTP 403 Forbidden - missing permissions 'Datastore.Backup' on '/datastore/store'. The account needs the Datastore.Backup permission on the datastore (and on the namespace, if one is used) to back up`, false},
		{`Post "https://pbs:8007/fixed_index": PBS authentication failed: HTTP 401 Unauthorized - authentication failed`, false},
	} {
		if got := Reconnectable(errors.New(tc.err)); got != tc.want {
			t.Errorf("Reconnectable(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestSessionLost(t *testing.T) {
	for _, tc := range []struct {
		err  string
		want bool
	}{
		{"connection lost: PBS session cannot be resumed (reconnect blocked to prevent orphaned sessions)", true},
		{`Post "https://pbs:8007/dynamic_chunk": http2: client connection lost`, true},
		{"failed to upload chunk: unexpected EOF", true},
		{"HTTP 400 - writer '3' not registered", true},
		// As a machine backup returned it on 03/11/2026 when a proxy cut its
		// connection: another upload's error came first.
		{`backup device C:\disk.img: failed to upload chunk 7197: Post "https://127.0.0.1:65371/fixed_chunk?digest=7197": http2: client connection force closed via ClientConn.Close`, true},
		{"HTTP 503 - service unavailable", false},
		{"open C:\\data\\x: Access is denied.", false},
	} {
		if got := SessionLost(errors.New(tc.err)); got != tc.want {
			t.Errorf("SessionLost(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
	if SessionLost(nil) {
		t.Error("SessionLost(nil)")
	}
}
