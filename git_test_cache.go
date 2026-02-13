package devflow

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TestCache provides git-based test caching to avoid re-running tests
// when the code hasn't changed since the last successful test run.
type TestCache struct {
	CacheDir string
}

// NewTestCache creates a new TestCache instance
func NewTestCache() *TestCache {
	return &TestCache{
		CacheDir: filepath.Join(os.TempDir(), "gotest-cache"),
	}
}

// GetCacheKey returns a unique key for the current module based on its path
func (tc *TestCache) GetCacheKey() (string, error) {
	moduleName, err := getModuleName(".")
	if err != nil {
		return "", err
	}
	// Hash the module name to create a safe filename
	hash := fmt.Sprintf("%x", md5.Sum([]byte(moduleName)))
	return hash[:16], nil
}

// GetCachePath returns the full path to the cache file
func (tc *TestCache) GetCachePath() (string, error) {
	key, err := tc.GetCacheKey()
	if err != nil {
		return "", err
	}
	return filepath.Join(tc.CacheDir, key), nil
}

// GetGitState returns current git state: commit hash + diff hash
// This uniquely identifies the exact state of the code
func (tc *TestCache) GetGitState() (string, error) {
	// Get current commit hash
	commitHash, err := RunCommandSilent("git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	// Get hash of uncommitted changes (if any)
	diff, err := RunCommandSilent("git", "diff", "HEAD")
	if err != nil {
		// No diff or error, use empty
		diff = ""
	}

	// Combine commit + diff hash for unique state
	diffHash := fmt.Sprintf("%x", md5.Sum([]byte(diff)))

	return commitHash + ":" + diffHash[:8], nil
}

// SaveCache saves the current git state and test message
func (tc *TestCache) SaveCache(message string) error {
	state, err := tc.GetGitState()
	if err != nil {
		return err
	}

	cachePath, err := tc.GetCachePath()
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(tc.CacheDir, 0755); err != nil {
		return err
	}

	// Store state and message separated by newline
	content := state + "\n" + message
	return os.WriteFile(cachePath, []byte(content), 0644)
}

// IsCacheValid checks if tests were already run successfully with the current code
func (tc *TestCache) IsCacheValid() bool {
	currentState, err := tc.GetGitState()
	if err != nil {
		return false
	}

	cachePath, err := tc.GetCachePath()
	if err != nil {
		return false
	}

	cachedData, err := os.ReadFile(cachePath)
	if err != nil {
		return false // No cache exists
	}

	// First line is the state
	lines := strings.SplitN(string(cachedData), "\n", 2)
	if len(lines) < 1 {
		return false
	}

	return strings.TrimSpace(lines[0]) == currentState
}

// GetCachedMessage returns the cached test output message
func (tc *TestCache) GetCachedMessage() string {
	cachePath, err := tc.GetCachePath()
	if err != nil {
		return ""
	}

	cachedData, err := os.ReadFile(cachePath)
	if err != nil {
		return ""
	}

	// Second line is the message
	lines := strings.SplitN(string(cachedData), "\n", 2)
	if len(lines) < 2 {
		return ""
	}

	return lines[1]
}

// InvalidateCache removes the cache file
func (tc *TestCache) InvalidateCache() error {
	cachePath, err := tc.GetCachePath()
	if err != nil {
		return err
	}
	return os.Remove(cachePath)
}
