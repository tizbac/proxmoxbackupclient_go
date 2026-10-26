package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/alphadose/haxmap"
)

// After Stop, chunks still queued for upload are not sent and not counted as
// failed uploads: the walk sees the cancel instead (upstream #117). The client
// is nil, so any attempt to upload would panic.
func TestStoppedUploadsAreDroppedNotFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var newchunk, reusechunk, failedchunk atomic.Uint64
	c := &ChunkState{cancelCtx: ctx}
	c.Init(nil, &newchunk, &reusechunk, &failedchunk, haxmap.New[string, bool](), nil, nil, nil, "")
	for i := 0; i < 3; i++ {
		c.uploadJobs <- uploadJob{digest: "d", data: []byte("x")}
	}
	c.closeUploads()
	c.closeUploads() // a second close (EOF after an error) must not panic
	c.uploadWG.Wait()

	if failedchunk.Load() != 0 || newchunk.Load() != 0 {
		t.Errorf("failed %d, new %d; want both 0 after Stop", failedchunk.Load(), newchunk.Load())
	}
	if err := c.checkFatal(); !errors.Is(err, context.Canceled) {
		t.Errorf("checkFatal = %v, want context.Canceled", err)
	}
	if len(c.uploadErrors) != 0 {
		t.Errorf("upload errors recorded for a Stop: %v", c.uploadErrors)
	}
}

// Without a Stop context the retries derive from Background, as before.
func TestRetryParentWithoutStop(t *testing.T) {
	c := &ChunkState{}
	if c.retryParent() != context.Background() || c.stopped() {
		t.Error("a ChunkState without cancelCtx must use Background and never report stopped")
	}
	c.closeUploads() // never initialised: nothing to close, must not panic
}
