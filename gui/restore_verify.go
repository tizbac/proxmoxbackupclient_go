package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"pbscommon"
	"sync"
	"sync/atomic"
)

// verifyRestoredFiles re-reads every file the restore wrote and compares its
// size and SHA-256 with the hash taken from the archive stream while the file
// was being written (pbscommon.PXARReader.SetHashFiles). A mismatch means what
// is on disk is not what PBS delivered (disk/filesystem fault, something else
// touched the file). Nothing is ever deleted or repaired here, only reported.
//
// onProgress is called (from several goroutines) with the number of files
// checked so far, the total, and the path just started.
func verifyRestoredFiles(ctx context.Context, files []pbscommon.PXARExtractedFile, onProgress func(done, total int, path string)) (verified int, failures []string, err error) {
	var todo []pbscommon.PXARExtractedFile
	for _, f := range files {
		if f.Skipped || f.IsDir || f.SHA256 == nil {
			continue
		}
		todo = append(todo, f)
	}

	const workers = 4
	var (
		next     atomic.Int64
		done     atomic.Int64
		okCount  atomic.Int64
		mu       sync.Mutex
		wg       sync.WaitGroup
		failList []string
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx != nil && ctx.Err() != nil {
					return
				}
				i := int(next.Add(1) - 1)
				if i >= len(todo) {
					return
				}
				f := todo[i]
				if onProgress != nil {
					onProgress(int(done.Load()), len(todo), f.Path)
				}
				if reason := verifyOneFile(f); reason != "" {
					mu.Lock()
					failList = append(failList, fmt.Sprintf("%s: %s", f.Path, reason))
					mu.Unlock()
				} else {
					okCount.Add(1)
				}
				done.Add(1)
			}
		}()
	}
	wg.Wait()
	if ctx != nil && ctx.Err() != nil {
		return int(okCount.Load()), failList, fmt.Errorf("restore cancelled while verifying")
	}
	return int(okCount.Load()), failList, nil
}

// verifyOneFile returns "" when the file on disk matches, otherwise why not.
func verifyOneFile(f pbscommon.PXARExtractedFile) string {
	fh, err := os.Open(f.Path)
	if err != nil {
		return fmt.Sprintf("cannot be read back (%v)", err)
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil {
		return fmt.Sprintf("cannot be read back (%v)", err)
	}
	if uint64(st.Size()) != f.Size {
		return fmt.Sprintf("size on disk is %d bytes, snapshot has %d", st.Size(), f.Size)
	}
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return fmt.Sprintf("cannot be read back (%v)", err)
	}
	if !bytes.Equal(h.Sum(nil), f.SHA256) {
		return "content on disk differs from what was received from the snapshot"
	}
	return ""
}
