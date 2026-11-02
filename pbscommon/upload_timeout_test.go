package pbscommon

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestUploadChunkTimesOut: a server that takes the chunk but never answers
// (a stuck stream, issue #59) fails the upload after chunkUploadTimeout
// instead of holding the backup for ever.
func TestUploadChunkTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	saved := chunkUploadTimeout
	chunkUploadTimeout = 200 * time.Millisecond
	defer func() { chunkUploadTimeout = saved }()

	client := newTestClient(srv.URL, nil)
	pt := bytes.Repeat([]byte("x"), 4096)
	start := time.Now()
	err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, true)
	if err == nil {
		t.Fatal("UploadChunk to a server that never answers: want an error")
	}
	if !strings.Contains(err.Error(), "no answer within") {
		t.Errorf("error = %v; want it to say the upload got no answer", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("UploadChunk took %v; want about %v", d, chunkUploadTimeout)
	}
}
