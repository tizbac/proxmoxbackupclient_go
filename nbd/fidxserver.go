package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"pbscommon"
	"retry"
	"slices"
	"sync"
	"time"
)
const LRU_CACHE_LIFE = 16 //will be 16*4MB usage

// chunkFetchTimeout bounds a SINGLE chunk fetch attempt — see the retry loop
// in ReadAt. Matches pbscommon/didx_reader.go's DIDXReaderAt.chunkFetchTimeout
// (the GUI restore path's own equivalent), for the same reason: chunks are at
// most ~16MB, so even a slow link should comfortably finish well inside this.
const chunkFetchTimeout = 60 * time.Second

type CachedChunk struct {
	Data []byte
	Index int64
	Life int
}

type FIDXServer struct {
	header pbscommon.FIDXHeader
	cached map[int64]*CachedChunk
	chunks []string
	lock sync.RWMutex
	client *pbscommon.PBSClient

	// reconnectMu/reconnectGen serialize and coalesce reconnects — see
	// reconnect's doc comment.
	reconnectMu  sync.Mutex
	reconnectGen int
}

// reconnectGeneration returns the current reconnect generation, to be passed
// back into reconnect after a failed fetch — see reconnect's doc comment.
func (f *FIDXServer) reconnectGeneration() int {
	f.reconnectMu.Lock()
	defer f.reconnectMu.Unlock()
	return f.reconnectGen
}

// reconnect tears down and re-establishes the PBS reader session (fresh
// HTTP/2 connection, fresh server-side reader task) on f.client in place,
// then bumps reconnectGen. Added 2026-09-24 after a real bare-metal restore
// test stalled four separate times in one session, at different chunks each
// time, and a SAME-connection retry (chunkFetchTimeout + 5 attempts, see
// ReadAt) demonstrably was not enough — it helped some cases but the exact
// same class of stall kept recurring. Every single manual recovery that
// night was the same fix: kill pbsnbd, reattach fresh, retry — a full
// reconnect, not a retry on the same connection — and it worked every time.
// This automates exactly that, in place, without restarting the whole
// attach/NBD session: the diagnosis is that the degradation is
// connection/session-level, not a single bad chunk, so retrying on the same
// connection just reproduces the same hang.
//
// observedGen is the generation the caller saw before its fetch failed; if
// another goroutine already completed a newer reconnect by the time this one
// acquires the lock, this is a no-op — N concurrent failures on the same bad
// connection trigger one reconnect, not N pointless ones back-to-back.
func (f *FIDXServer) reconnect(observedGen int) {
	f.reconnectMu.Lock()
	defer f.reconnectMu.Unlock()
	if f.reconnectGen != observedGen {
		return
	}
	fmt.Printf("[%s] Reconnecting to PBS after a chunk fetch failure...\n", time.Now().Format(time.RFC3339))
	f.client.Close()
	// ocs-pbs-nbd always authenticates with username/password (ticket) auth,
	// never an API token — refresh the ticket too, in case a long enough
	// run with enough reconnects ever outlives its original ticket. Cheap
	// and correct to do unconditionally when a username was configured;
	// a no-op for token auth (Username is empty in that case).
	if f.client.Username != "" {
		if err := f.client.ObtainTicket(); err != nil {
			fmt.Printf("Reconnect: ObtainTicket failed: %v\n", err)
		}
	}
	f.client.Connect(true, f.client.Manifest.BackupType)
	f.reconnectGen++
	fmt.Printf("[%s] Reconnected.\n", time.Now().Format(time.RFC3339))
}

func NewFIDXServer(data []byte, client *pbscommon.PBSClient) (*FIDXServer, error) {
	var ret FIDXServer
	ret.client = client
	rdr := bytes.NewReader(data)
	err := binary.Read(rdr, binary.LittleEndian, &ret.header)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(ret.header.Magic[:], []byte{47, 127, 65, 237, 145, 253, 15, 205}) {
		return nil, fmt.Errorf("FIDX: Invalid magic %+v", ret.header.Magic)
	}
	fmt.Printf("%+v\n", ret.header)
	for i := uint64(0); i < ret.header.Size/ret.header.ChunkSize + min(1, ret.header.Size%ret.header.ChunkSize); i++ {
		H := make([]byte, 32)
		nbytes, err := rdr.Read(H)
		if err != nil {
			return nil, err
		}
		if nbytes != len(H) {
			return nil, fmt.Errorf("FIDX: Short read")
		}
		ret.chunks = append(ret.chunks, hex.EncodeToString(H))
	}
	ret.cached = make(map[int64]*CachedChunk)
	fmt.Printf("Read ok %d\n", len(ret.chunks))
	return &ret, nil
}

type ChunkIndex struct {
	Index int64 
	SliceStart int64 
	SliceEnd int64
}

func (f * FIDXServer) getChunksIndexes(offset int64, size int64) ([]ChunkIndex) {
	ret := make([]ChunkIndex, 0)
	for i := int64(offset/pbscommon.PBS_FIXED_CHUNK_SIZE); i < (offset+size)/pbscommon.PBS_FIXED_CHUNK_SIZE+1; i++ {
		ss := max(0,offset-i*pbscommon.PBS_FIXED_CHUNK_SIZE)
		se := min(pbscommon.PBS_FIXED_CHUNK_SIZE, (offset+size)-i*pbscommon.PBS_FIXED_CHUNK_SIZE)
		if se-ss == 0 {
			continue
		}
		ret = append(ret, ChunkIndex{
			Index: i,
			SliceStart: ss,
			SliceEnd: se,
		})
	}
	//fmt.Printf("%+v %d %d\n", ret, offset, size)
	return ret
}

func (f * FIDXServer) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(f.header.Size) {
		return 0, io.EOF
	}
	f.lock.RLock()
	indexes := f.getChunksIndexes(off, int64(len(p)))
	var pos int64 = 0
	for _, idx := range indexes {
		ch, ok := f.cached[idx.Index]
		if ok {
			
		} else {
			// A per-attempt timeout, not a deadline on the whole session
			// (pbsnbd serves reads for however long the NBD device stays
			// attached — a Clonezilla restore can run for hours): each
			// individual chunk fetch gets chunkFetchTimeout, retried up to
			// 5 times, mirroring pbscommon/didx_reader.go's DIDXReaderAt
			// (the GUI restore path's own equivalent protection). Added
			// 2026-09-24 after a real bare-metal restore test hung
			// indefinitely mid-partition-clone: GetChunkData(context.
			// Background()) has no deadline at all, so one stuck HTTP/2
			// stream (the exact class of hang seen earlier the same night
			// in the GUI, root cause still unconfirmed server-side) blocked
			// this ReadAt — and therefore Clonezilla's partclone — forever,
			// with the only recovery being to kill pbsnbd and restart the
			// whole attach+restore from scratch. A retried, bounded fetch
			// gives a stuck stream a real chance to recover on its own
			// before that drastic step is needed.
			//
			// EXTENDED same night, after that same-connection retry alone
			// proved insufficient — a real restore stalled four separate
			// times in one session, at different chunks, and every manual
			// recovery was a full reconnect (kill pbsnbd, reattach fresh),
			// never just a retry. So a failed attempt now reconnects (see
			// FIDXServer.reconnect) BEFORE the next retry, instead of
			// retrying on what's likely the same degraded connection.
			var data []byte
			attempt := 0
			retryCfg := retry.DefaultConfig()
			retryCfg.MaxAttempts = 5
			fetchErr := retry.DoWithJitter(context.Background(), retryCfg, retry.DefaultRetryable, func() error {
				attempt++
				// Diagnostic logging added 2026-09-24 after a live stall where
				// NEITHER a new "Got" line NOR a "Reconnecting..." line
				// appeared for far longer than chunkFetchTimeout x 5 should
				// allow, with pbsnbd still alive throughout - i.e. no visible
				// evidence the timeout/retry/reconnect logic was firing AT
				// ALL, not just failing to recover. Without this, that is
				// unfalsifiable from the log alone. With it, the next
				// occurrence will show exactly: whether an attempt started,
				// how long it actually ran before returning (vs the intended
				// chunkFetchTimeout bound), and the exact error returned.
				attemptStart := time.Now()
				fmt.Printf("[%s] Fetching chunk %s (attempt %d/%d)...\n", attemptStart.Format(time.RFC3339), f.chunks[idx.Index], attempt, retryCfg.MaxAttempts)
				fetchCtx, cancel := context.WithTimeout(context.Background(), chunkFetchTimeout)
				defer cancel()
				d, ferr := f.client.GetChunkData(fetchCtx, f.chunks[idx.Index])
				elapsed := time.Since(attemptStart)
				if ferr != nil {
					fmt.Printf("[%s] Attempt %d/%d for chunk %s FAILED after %s: %v\n", time.Now().Format(time.RFC3339), attempt, retryCfg.MaxAttempts, f.chunks[idx.Index], elapsed, ferr)
					gen := f.reconnectGeneration()
					f.reconnect(gen)
					return ferr
				}
				if elapsed > 5*time.Second {
					fmt.Printf("[%s] Attempt %d for chunk %s succeeded but took %s\n", time.Now().Format(time.RFC3339), attempt, f.chunks[idx.Index], elapsed)
				}
				data = d
				return nil
			})
			if fetchErr != nil {
				// All retries (each preceded by a reconnect) exhausted —
				// this used to be the ONLY outcome (immediately, on the
				// very first error, timeout or not). Still fatal: FIDXServer
				// has no way to report a read failure through io.ReaderAt's
				// contract back to the NBD protocol layer gracefully at this
				// call depth, so this crashes pbsnbd same as before. The
				// real improvement is upstream — a transient stall, and even
				// a degraded connection, now gets up to 5 fresh-connection
				// chances to clear on their own first.
				panic(fetchErr)
			}

			for _, idx2 := range f.cached {
				idx2.Life--
			}
			f.cached[idx.Index] = &CachedChunk{
				Data: data,
				Index: idx.Index,
				Life: LRU_CACHE_LIFE,
			}

			fmt.Printf("Got %s\n", f.chunks[idx.Index])

			ch , _ = f.cached[idx.Index]
		}

		copy(p[pos:pos+(idx.SliceEnd-idx.SliceStart)], ch.Data[idx.SliceStart:idx.SliceEnd])
			
		ch.Life = LRU_CACHE_LIFE
		pos += (idx.SliceEnd-idx.SliceStart)
	}



	// Clean up expired cache entries (Go 1.22 compatible)
	for key := range f.cached {
		if f.cached[key].Life <= 0 {
			delete(f.cached, key)
			fmt.Printf("Remove from cache %s\n", f.chunks[key])
		}
	}

	f.lock.RUnlock()

	if pos != int64(len(p)) {
		panic(fmt.Errorf("Short read"))
	}

	return int(pos), nil
}

func (f *FIDXServer) WriteAt(p []byte, off int64) (n int, err error) {
	

	return 0, fmt.Errorf("Read only")
}


func (f *FIDXServer) Size() (int64, error) {

	return int64(f.header.Size), nil
}

func (f *FIDXServer) Sync() error {
	return fmt.Errorf("Read only")
}

