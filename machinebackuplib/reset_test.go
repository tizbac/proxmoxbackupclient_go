package machinebackuplib

import "testing"

// A retried job starts its figures from nothing, including the sent bytes of
// the session that was lost.
func TestJobProgressReset(t *testing.T) {
	jp := &JobProgress{}
	jp.countChunk(7, false, true)
	jp.BytesDone.Store(5)
	jp.BytesTotal.Store(9)
	jp.FreeSkipped.Store(3)
	jp.noteUnreadable("d", 0, 512)
	jp.setSentSource(func() uint64 { return 42 })
	jp.startDisk("d", "model", 1, 1, 100)
	jp.setPartition("p")
	if jp.SentBytes() != 42 {
		t.Fatalf("SentBytes = %d, want 42", jp.SentBytes())
	}

	jp.Reset()
	if jp.NewChunks.Load() != 0 || jp.UploadedBytes.Load() != 0 || jp.BytesDone.Load() != 0 ||
		jp.BytesTotal.Load() != 0 || jp.FreeSkipped.Load() != 0 || jp.UnreadableBytes.Load() != 0 {
		t.Fatal("counters not cleared")
	}
	if jp.SentBytes() != 0 {
		t.Fatalf("SentBytes after Reset = %d, want 0", jp.SentBytes())
	}
	if (jp.Where() != Location{}) || len(jp.Unreadable()) != 0 {
		t.Fatalf("location or unreadable list not cleared: %+v %v", jp.Where(), jp.Unreadable())
	}
	var nilJP *JobProgress
	nilJP.Reset() // must not panic
}
