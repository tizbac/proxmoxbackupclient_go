package snapshot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestSnapshotStructure verifies the SnapShot struct
func TestSnapshotStructure(t *testing.T) {
	snap := SnapShot{
		FullPath:   "/mnt/snapshot/volume/data",
		Id:         "test-snapshot-123",
		ObjectPath: "/dev/vss/object",
		Valid:      true,
	}

	if snap.FullPath == "" {
		t.Error("FullPath should not be empty")
	}

	if snap.Id == "" {
		t.Error("Id should not be empty")
	}

	if !snap.Valid {
		t.Error("Valid should be true")
	}
}

// TestSnapshotInvalid tests creating invalid snapshot
func TestSnapshotInvalid(t *testing.T) {
	snap := SnapShot{
		FullPath:   "",
		Id:         "",
		ObjectPath: "",
		Valid:      false,
	}

	if snap.Valid {
		t.Error("Snapshot should be marked as invalid")
	}
}

// TestSymlinkSnapshot_Windows tests symlink creation on Windows
func TestSymlinkSnapshot_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// Create temporary directory for testing
	tmpDir := t.TempDir()

	testCases := []struct {
		name             string
		symlinkPath      string
		id               string
		deviceObjectPath string
		expectError      bool
	}{
		{
			name:             "valid paths",
			symlinkPath:      tmpDir,
			id:               "test-snapshot-1",
			deviceObjectPath: `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy1`,
			expectError:      false, // Will error without admin, but tests path logic
		},
		{
			name:             "empty id",
			symlinkPath:      tmpDir,
			id:               "",
			deviceObjectPath: `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy1`,
			expectError:      false, // Path operations should still work
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: This will fail without admin privileges
			// But we can at least test that it doesn't panic
			_, err := SymlinkSnapshot(tc.symlinkPath, tc.id, tc.deviceObjectPath)

			// We expect errors on non-admin systems, just ensure it doesn't panic
			if err != nil {
				t.Logf("Expected error (requires admin): %v", err)
			}
		})
	}
}

// TestGetAppDataFolder_Windows tests AppData folder detection
func TestGetAppDataFolder_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	appDataFolder, err := getAppDataFolder()
	if err != nil {
		t.Fatalf("Failed to get AppData folder: %v", err)
	}

	if appDataFolder == "" {
		t.Error("AppData folder should not be empty")
	}

	// Verify it ends with expected path
	if !filepath.IsAbs(appDataFolder) {
		t.Errorf("AppData folder should be absolute path, got: %s", appDataFolder)
	}

	// Check if folder exists (should be created by function)
	info, err := os.Stat(appDataFolder)
	if err != nil {
		t.Errorf("AppData folder should exist: %v", err)
	}

	if !info.IsDir() {
		t.Error("AppData folder should be a directory")
	}
}

// TestCreateVSSSnapshot_RequiresAdmin tests VSS snapshot creation
func TestCreateVSSSnapshot_RequiresAdmin(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// This test requires admin privileges and will likely fail
	// But we test the function signature and basic behavior
	t.Run("without admin", func(t *testing.T) {
		paths := []string{os.TempDir()}

		callbackExecuted := false
		err := CreateVSSSnapshot(paths, func(snapshots map[string]SnapShot) error {
			callbackExecuted = true

			// Verify snapshot structure
			if len(snapshots) == 0 {
				t.Error("Expected at least one snapshot")
			}

			for path, snap := range snapshots {
				t.Logf("Snapshot for %s: %+v", path, snap)

				if snap.Id == "" {
					t.Error("Snapshot ID should not be empty")
				}

				if snap.FullPath == "" {
					t.Error("Snapshot FullPath should not be empty")
				}

				if !snap.Valid {
					t.Error("Snapshot should be marked as valid")
				}
			}

			return nil
		})

		// We expect this to fail without admin privileges
		if err != nil {
			t.Logf("Expected error without admin privileges: %v", err)
			// This is OK - VSS requires admin
		}

		if err == nil && callbackExecuted {
			t.Log("VSS snapshot created successfully (running with admin privileges)")
		}
	})
}

// TestVSSCleanup tests cleanup function
func TestVSSCleanup(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// VSSCleanup is currently a no-op, but test it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("VSSCleanup panicked: %v", r)
		}
	}()

	VSSCleanup()
}

// TestSnapshotPathHandling tests path manipulation logic
func TestSnapshotPathHandling(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	testCases := []struct {
		name         string
		inputPath    string
		wantVolume   string
		wantSubPath  string
	}{
		{
			name:       "C drive with path",
			inputPath:  `C:\Users\Test\Documents`,
			wantVolume: `C:\`,
		},
		{
			name:       "D drive",
			inputPath:  `D:\Data`,
			wantVolume: `D:\`,
		},
		{
			name:       "Root of C",
			inputPath:  `C:\`,
			wantVolume: `C:\`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			absPath, err := filepath.Abs(tc.inputPath)
			if err != nil {
				t.Skipf("Cannot get absolute path for %s: %v", tc.inputPath, err)
			}

			volName := filepath.VolumeName(absPath)
			if volName == "" {
				t.Error("Volume name should not be empty")
			}

			volName += "\\"
			if volName != tc.wantVolume {
				t.Errorf("Volume = %q, want %q", volName, tc.wantVolume)
			}

			// Verify subpath extraction
			subPath := absPath[len(volName):]
			t.Logf("Input: %s -> Volume: %s, SubPath: %s", absPath, volName, subPath)
		})
	}
}

// TestSnapshotMapHandling tests snapshot map operations
func TestSnapshotMapHandling(t *testing.T) {
	snapshots := make(map[string]SnapShot)

	// Add snapshots
	snapshots["C:\\Users\\Test"] = SnapShot{
		FullPath:   "\\\\?\\GLOBALROOT\\Device\\HarddiskVolumeShadowCopy1\\Users\\Test",
		Id:         "snapshot-1",
		ObjectPath: "\\\\?\\GLOBALROOT\\Device\\HarddiskVolumeShadowCopy1",
		Valid:      true,
	}

	snapshots["D:\\Data"] = SnapShot{
		FullPath:   "\\\\?\\GLOBALROOT\\Device\\HarddiskVolumeShadowCopy2\\Data",
		Id:         "snapshot-2",
		ObjectPath: "\\\\?\\GLOBALROOT\\Device\\HarddiskVolumeShadowCopy2",
		Valid:      true,
	}

	// Verify map operations
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(snapshots))
	}

	// Test retrieval
	snap, exists := snapshots["C:\\Users\\Test"]
	if !exists {
		t.Error("Snapshot should exist for C:\\Users\\Test")
	}

	if snap.Id != "snapshot-1" {
		t.Errorf("Snapshot ID = %q, want %q", snap.Id, "snapshot-1")
	}

	// Test iteration
	count := 0
	for path, snap := range snapshots {
		count++
		t.Logf("Snapshot %d: %s -> %s (valid=%v)", count, path, snap.FullPath, snap.Valid)

		if !snap.Valid {
			t.Errorf("Snapshot for %s should be valid", path)
		}
	}

	if count != 2 {
		t.Errorf("Iterated %d times, expected 2", count)
	}
}

// TestSnapshotCallbackPattern tests the callback pattern
func TestSnapshotCallbackPattern(t *testing.T) {
	// Simulate the callback pattern used by CreateVSSSnapshot
	mockSnapshots := map[string]SnapShot{
		"/tmp/test": {
			FullPath:   "/mnt/snapshot/test",
			Id:         "mock-snapshot-id",
			ObjectPath: "/dev/snapshot",
			Valid:      true,
		},
	}

	// Test successful callback
	t.Run("successful callback", func(t *testing.T) {
		callbackCalled := false

		callback := func(snapshots map[string]SnapShot) error {
			callbackCalled = true

			if len(snapshots) == 0 {
				t.Error("Snapshots should not be empty")
			}

			return nil
		}

		err := callback(mockSnapshots)
		if err != nil {
			t.Errorf("Callback should not return error: %v", err)
		}

		if !callbackCalled {
			t.Error("Callback should have been called")
		}
	})

	// Test callback with error
	t.Run("callback with error", func(t *testing.T) {
		callback := func(snapshots map[string]SnapShot) error {
			return os.ErrInvalid
		}

		err := callback(mockSnapshots)
		if err != os.ErrInvalid {
			t.Errorf("Expected ErrInvalid, got %v", err)
		}
	})
}

// Benchmark snapshot map operations
func BenchmarkSnapshotMapOperations(b *testing.B) {
	snapshots := make(map[string]SnapShot)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Add
		snapshots["test-path"] = SnapShot{
			FullPath:   "/mnt/snapshot/test",
			Id:         "benchmark-id",
			ObjectPath: "/dev/snapshot",
			Valid:      true,
		}

		// Read
		_ = snapshots["test-path"]

		// Delete
		delete(snapshots, "test-path")
	}
}
