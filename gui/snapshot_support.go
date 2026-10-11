package main

import (
	"sync"
	"time"
)

// SnapshotSupport reports, for each folder, whether a backup can read it
// through a snapshot (VSS on Windows, a block snapshot on Linux). Network
// drives and shares cannot be snapshotted and are read live. The Backup Set
// cards use it to show an amber badge when a set asks for a snapshot but some
// of its folders cannot have one.
//
// Each check gets a short time limit: on Linux a hung network mount can block
// statfs indefinitely, and a folder that slow to answer is a network one.
func (a *App) SnapshotSupport(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range paths {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok := make(chan bool, 1)
			go func() { ok <- pathSupportsSnapshot(p) }()
			var v bool
			select {
			case v = <-ok:
			case <-time.After(2 * time.Second):
				v = false
			}
			mu.Lock()
			out[p] = v
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}
