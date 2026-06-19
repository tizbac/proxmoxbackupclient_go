package pbscommon

import (
	"bytes"
	"crypto/rand"
	"testing"
)

// TestChunkerNew verifies chunker initialization
func TestChunkerNew(t *testing.T) {
	tests := []struct {
		name         string
		avgSize      uint64
		wantMinSize  uint64
		wantMaxSize  uint64
		wantWindowSize uint64
	}{
		{
			name:         "1MB average",
			avgSize:      1024 * 1024,
			wantMinSize:  256 * 1024,  // avg / 4
			wantMaxSize:  4096 * 1024, // avg * 4
			wantWindowSize: 64,
		},
		{
			name:         "4MB average",
			avgSize:      4 * 1024 * 1024,
			wantMinSize:  1024 * 1024,  // avg / 4
			wantMaxSize:  16 * 1024 * 1024, // avg * 4
			wantWindowSize: 64,
		},
		{
			name:         "Small 64KB average",
			avgSize:      64 * 1024,
			wantMinSize:  16 * 1024,
			wantMaxSize:  256 * 1024,
			wantWindowSize: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Chunker{}
			c.New(tt.avgSize)

			if c.chunk_size_min != tt.wantMinSize {
				t.Errorf("chunk_size_min = %d, want %d", c.chunk_size_min, tt.wantMinSize)
			}
			if c.chunk_size_max != tt.wantMaxSize {
				t.Errorf("chunk_size_max = %d, want %d", c.chunk_size_max, tt.wantMaxSize)
			}
			if uint64(len(c.window)) != tt.wantWindowSize {
				t.Errorf("window size = %d, want %d", len(c.window), tt.wantWindowSize)
			}
			if c.h != 0 {
				t.Errorf("initial hash should be 0, got %d", c.h)
			}
			if c.chunk_size != 0 {
				t.Errorf("initial chunk_size should be 0, got %d", c.chunk_size)
			}
		})
	}
}

// TestChunkerDeterministic verifies chunking is deterministic
func TestChunkerDeterministic(t *testing.T) {
	avgSize := uint64(1024 * 1024) // 1MB

	// Create test data
	testData := make([]byte, 10*1024*1024) // 10MB
	rand.Read(testData)

	// Chunk the same data twice
	chunks1 := chunkData(t, testData, avgSize)
	chunks2 := chunkData(t, testData, avgSize)

	// Results should be identical
	if len(chunks1) != len(chunks2) {
		t.Fatalf("chunk counts differ: %d vs %d", len(chunks1), len(chunks2))
	}

	for i := range chunks1 {
		if chunks1[i] != chunks2[i] {
			t.Errorf("chunk %d differs: %d vs %d", i, chunks1[i], chunks2[i])
		}
	}
}

// TestChunkerMinMaxBoundaries verifies chunk size boundaries
func TestChunkerMinMaxBoundaries(t *testing.T) {
	avgSize := uint64(1024 * 1024) // 1MB
	c := &Chunker{}
	c.New(avgSize)

	minSize := c.chunk_size_min
	maxSize := c.chunk_size_max

	// Create data larger than max chunk size
	testData := make([]byte, maxSize+1024*1024)
	rand.Read(testData)

	chunks := chunkData(t, testData, avgSize)

	for i, size := range chunks {
		if size < minSize {
			t.Errorf("chunk %d size %d is below minimum %d", i, size, minSize)
		}
		if size > maxSize {
			t.Errorf("chunk %d size %d exceeds maximum %d", i, size, maxSize)
		}
	}
}

// TestChunkerContentAwareness verifies chunking is content-aware
func TestChunkerContentAwareness(t *testing.T) {
	avgSize := uint64(1024 * 1024)

	// Create data with a recognizable pattern in the middle
	data1 := make([]byte, 5*1024*1024)
	data2 := make([]byte, 5*1024*1024)
	rand.Read(data1)
	rand.Read(data2)

	// Insert same 1MB block in the middle of both
	commonBlock := make([]byte, 1024*1024)
	rand.Read(commonBlock)

	copy(data1[2*1024*1024:3*1024*1024], commonBlock)
	copy(data2[2*1024*1024:3*1024*1024], commonBlock)

	chunks1 := chunkData(t, data1, avgSize)
	chunks2 := chunkData(t, data2, avgSize)

	// The common block should result in similar chunking patterns
	// At least some chunks should align
	t.Logf("data1 produced %d chunks: %v", len(chunks1), chunks1)
	t.Logf("data2 produced %d chunks: %v", len(chunks2), chunks2)

	// This test mainly verifies chunking runs without panics
	// and produces reasonable results
}

// TestChunkerEmptyData verifies behavior with empty input
func TestChunkerEmptyData(t *testing.T) {
	c := &Chunker{}
	c.New(1024 * 1024)

	emptyData := []byte{}
	pos := c.Scan(emptyData)

	if pos != 0 {
		t.Errorf("scanning empty data should return 0, got %d", pos)
	}
}

// TestChunkerSmallData verifies behavior with data smaller than window
func TestChunkerSmallData(t *testing.T) {
	c := &Chunker{}
	c.New(1024 * 1024)

	// Data smaller than window size (64 bytes)
	smallData := []byte("hello world")
	pos := c.Scan(smallData)

	if pos != 0 {
		t.Errorf("scanning small data should return 0, got %d", pos)
	}

	if c.window_size != uint64(len(smallData)) {
		t.Errorf("window_size should be %d, got %d", len(smallData), c.window_size)
	}
}

// TestChunkerIncrementalScanning verifies incremental data feeding
func TestChunkerIncrementalScanning(t *testing.T) {
	avgSize := uint64(1024 * 1024)

	// Full scan
	testData := make([]byte, 5*1024*1024)
	rand.Read(testData)
	chunksFullScan := chunkData(t, testData, avgSize)

	// Incremental scan (feed data in 100KB chunks)
	c := &Chunker{}
	c.New(avgSize)
	chunksIncremental := []uint64{}

	chunkSize := 100 * 1024
	for offset := 0; offset < len(testData); {
		end := offset + chunkSize
		if end > len(testData) {
			end = len(testData)
		}

		pos := c.Scan(testData[offset:end])
		if pos > 0 {
			chunksIncremental = append(chunksIncremental, c.chunk_size)
		}

		offset = end
	}

	// Results should be similar (chunking is independent of how data is fed)
	t.Logf("Full scan: %d chunks", len(chunksFullScan))
	t.Logf("Incremental: %d chunks", len(chunksIncremental))

	// Allow some tolerance due to boundary effects
	if abs(len(chunksFullScan)-len(chunksIncremental)) > 2 {
		t.Errorf("chunk counts differ significantly: full=%d, incremental=%d",
			len(chunksFullScan), len(chunksIncremental))
	}
}

// TestChunkerAverageSize verifies average chunk size is near target
func TestChunkerAverageSize(t *testing.T) {
	avgSize := uint64(1024 * 1024) // 1MB target

	// Create 100MB of test data
	testData := make([]byte, 100*1024*1024)
	rand.Read(testData)

	chunks := chunkData(t, testData, avgSize)

	// Calculate actual average
	var totalSize uint64
	for _, size := range chunks {
		totalSize += size
	}
	actualAvg := totalSize / uint64(len(chunks))

	t.Logf("Target average: %d bytes", avgSize)
	t.Logf("Actual average: %d bytes", actualAvg)
	t.Logf("Chunk count: %d", len(chunks))

	// Allow 30% variance (chunking is probabilistic)
	tolerance := float64(avgSize) * 0.3
	diff := float64(abs64(int64(actualAvg) - int64(avgSize)))

	if diff > tolerance {
		t.Errorf("average chunk size %d differs too much from target %d (tolerance: %.0f)",
			actualAvg, avgSize, tolerance)
	}
}

// TestBuzhashTable verifies the buzhash table is properly initialized
func TestBuzhashTable(t *testing.T) {
	if len(buzhash_table) != 256 {
		t.Fatalf("buzhash_table should have 256 entries, got %d", len(buzhash_table))
	}

	// Check for unique values (no duplicates)
	seen := make(map[uint32]bool)
	for i, val := range buzhash_table {
		if seen[val] {
			t.Errorf("duplicate value %d at index %d", val, i)
		}
		seen[val] = true
	}

	// Check that values are non-zero
	zeroCount := 0
	for _, val := range buzhash_table {
		if val == 0 {
			zeroCount++
		}
	}

	if zeroCount > 5 {
		t.Errorf("too many zero values in buzhash_table: %d", zeroCount)
	}
}

// Helper function to chunk data and return slice of chunk sizes
func chunkData(t *testing.T, data []byte, avgSize uint64) []uint64 {
	t.Helper()

	c := &Chunker{}
	c.New(avgSize)

	chunks := []uint64{}
	offset := 0

	for offset < len(data) {
		remaining := data[offset:]
		pos := c.Scan(remaining)

		if pos == 0 {
			// No chunk boundary found, feed more data
			offset = len(data)
			break
		}

		// Chunk boundary found. The chunk size is `pos` (the number of bytes
		// consumed from `remaining` up to the break) — NOT c.chunk_size, which
		// Scan resets to 0 the moment it breaks, so reading it here always gave 0.
		chunks = append(chunks, pos)
		offset += int(pos)

		// Reset state for next chunk (this happens in Scan internally)
	}

	return chunks
}

// Helper functions
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// Benchmark chunking performance
func BenchmarkChunker1MB(b *testing.B) {
	benchmarkChunker(b, 1024*1024)
}

func BenchmarkChunker4MB(b *testing.B) {
	benchmarkChunker(b, 4*1024*1024)
}

func benchmarkChunker(b *testing.B, avgSize uint64) {
	// Create 10MB test data
	testData := make([]byte, 10*1024*1024)
	rand.Read(testData)

	b.ResetTimer()
	b.SetBytes(int64(len(testData)))

	for i := 0; i < b.N; i++ {
		c := &Chunker{}
		c.New(avgSize)

		offset := 0
		for offset < len(testData) {
			pos := c.Scan(testData[offset:])
			if pos == 0 {
				break
			}
			offset += int(pos)
		}
	}
}

// Test for regression: ensure chunker doesn't panic on edge cases
func TestChunkerNoPanic(t *testing.T) {
	avgSize := uint64(1024 * 1024)

	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"single byte", []byte{0xFF}},
		{"all zeros", bytes.Repeat([]byte{0}, 1024*1024)},
		{"all ones", bytes.Repeat([]byte{0xFF}, 1024*1024)},
		{"alternating", bytes.Repeat([]byte{0xAA, 0x55}, 512*1024)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("chunker panicked on %s: %v", tc.name, r)
				}
			}()

			c := &Chunker{}
			c.New(avgSize)
			_ = c.Scan(tc.data)
		})
	}
}
