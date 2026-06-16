package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTestCache_SaveAndValidate(t *testing.T) {
	dir, cleanup := testCreateGoModule("example.com/test")
	defer cleanup()
	defer testChdir(t, dir)()

	// Init git
	devflow.RunCommand("git", "init")
	devflow.RunCommand("git", "config", "user.name", "Test")
	devflow.RunCommand("git", "config", "user.email", "test@test.com")
	devflow.RunCommand("git", "add", ".")
	devflow.RunCommand("git", "commit", "-m", "init")

	cache := devflow.NewTestCache(".")
	testMsg := "vet ✅, tests ✅"

	// Clean up any existing cache
	cache.InvalidateCache()

	// Initially cache should be invalid
	if cache.IsCacheValid() {
		t.Error("Cache should be invalid before saving")
	}

	// Save cache with message
	if err := cache.SaveCache(testMsg); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Now cache should be valid
	if !cache.IsCacheValid() {
		t.Error("Cache should be valid after saving")
	}

	// Cached message should match
	if got := cache.GetCachedMessage(); got != testMsg {
		t.Errorf("GetCachedMessage() = %q, want %q", got, testMsg)
	}

	// Cleanup
	cache.InvalidateCache()
}

func TestTestCache_InvalidateCache(t *testing.T) {
	dir, cleanup := testCreateGoModule("example.com/test")
	defer cleanup()
	defer testChdir(t, dir)()

	// Init git
	devflow.RunCommand("git", "init")
	devflow.RunCommand("git", "config", "user.name", "Test")
	devflow.RunCommand("git", "config", "user.email", "test@test.com")
	devflow.RunCommand("git", "add", ".")
	devflow.RunCommand("git", "commit", "-m", "init")

	cache := devflow.NewTestCache(".")

	// Save cache
	if err := cache.SaveCache("test message"); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Verify it's valid
	if !cache.IsCacheValid() {
		t.Error("Cache should be valid after saving")
	}

	// Invalidate
	cache.InvalidateCache()

	// Should be invalid now
	if cache.IsCacheValid() {
		t.Error("Cache should be invalid after invalidation")
	}
}

func TestTestCache_CacheKey(t *testing.T) {
	dir, cleanup := testCreateGoModule("example.com/test")
	defer cleanup()
	defer testChdir(t, dir)()

	cache := devflow.NewTestCache(".")

	key, err := cache.GetCacheKey()
	if err != nil {
		t.Fatalf("Failed to get cache key: %v", err)
	}

	if len(key) != 16 {
		t.Errorf("Cache key should be 16 characters, got %d: %s", len(key), key)
	}

	// Key should be consistent
	key2, _ := cache.GetCacheKey()
	if key != key2 {
		t.Error("Cache key should be consistent across calls")
	}
}

func TestTestCache_GitState(t *testing.T) {
	if _, err := devflow.RunCommandSilent("git", "rev-parse", "HEAD"); err != nil {
		t.Skip("Not in a git repository")
	}

	cache := devflow.NewTestCache(".")

	state, err := cache.GetGitState()
	if err != nil {
		t.Fatalf("Failed to get git state: %v", err)
	}

	// State should be in format "commitHash:diffHash"
	if len(state) < 10 {
		t.Errorf("Git state seems too short: %s", state)
	}

	// State should contain a colon separator
	if !containsColon(state) {
		t.Errorf("Git state should contain colon separator: %s", state)
	}

	// State should be consistent when code hasn't changed
	state2, _ := cache.GetGitState()
	if state != state2 {
		t.Error("Git state should be consistent when code hasn't changed")
	}
}

func TestTestCache_CacheDirectory(t *testing.T) {
	cache := devflow.NewTestCache(".")

	expectedDir := filepath.Join(os.TempDir(), "gotest-cache")
	if cache.CacheDir != expectedDir {
		t.Errorf("Cache dir should be %s, got %s", expectedDir, cache.CacheDir)
	}
}

// TestTestCache_UntrackedFileInvalidatesCache verifies that adding an untracked file
// (new test file not yet staged) changes the git state and invalidates the cache.
// Reproduces the bug where gotest returns a cached success even when a new failing
// test file is added but not yet committed (status "??").
func TestTestCache_UntrackedFileInvalidatesCache(t *testing.T) {
	dir, cleanup := testCreateGoModule("example.com/test")
	defer cleanup()
	defer testChdir(t, dir)()

	devflow.RunCommand("git", "init")
	devflow.RunCommand("git", "config", "user.name", "Test")
	devflow.RunCommand("git", "config", "user.email", "test@test.com")
	devflow.RunCommand("git", "add", ".")
	devflow.RunCommand("git", "commit", "-m", "init")

	cache := devflow.NewTestCache(".")
	cache.InvalidateCache()

	// Save cache simulating a successful full run
	if err := cache.SaveCache("vet ✅, race ✅, tests ✅, wasm ✅, coverage: 90.0%"); err != nil {
		t.Fatalf("SaveCache failed: %v", err)
	}
	if !cache.IsCacheValid() {
		t.Fatal("Cache should be valid right after saving")
	}

	// Add a new untracked file (not staged, not committed — simulates adding a new test)
	newFile := filepath.Join(dir, "new_failing_test.go")
	if err := os.WriteFile(newFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to write untracked file: %v", err)
	}

	// BUG: IsCacheValid() returns true even though a new .go file was added.
	// It should return false because the module state changed.
	if cache.IsCacheValid() {
		t.Error("BUG: cache is still valid after adding an untracked .go file — gotest will skip the new failing test")
	}
}

func containsColon(s string) bool {
	for _, c := range s {
		if c == ':' {
			return true
		}
	}
	return false
}
