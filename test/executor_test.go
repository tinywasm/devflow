package devflow_test

import "github.com/tinywasm/devflow"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestConcurrentSafeExecution validates that commands run in isolated directories
// concurrently without interfering with each other (no os.Chdir usage).
func TestConcurrentSafeExecution(t *testing.T) {
	// Create N temporary directories
	tmpRoot := t.TempDir()
	numGoroutines := 10
	dirs := make([]string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		dirName := fmt.Sprintf("dir_%d", i)
		fullPath := filepath.Join(tmpRoot, dirName)
		if err := os.Mkdir(fullPath, 0755); err != nil {
			t.Fatal(err)
		}
		// Resolve symlinks to avoid /var/folders vs /private/var confusion on some OS
		realPath, err := filepath.EvalSymlinks(fullPath)
		if err != nil {
			t.Fatal(err)
		}
		dirs[i] = realPath
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int, targetDir string) {
			defer wg.Done()

			// Execute "pwd" (linux) in the target directory
			// Note: On Mac/Linux 'pwd' outputs the physical path.
			output, err := devflow.RunCommandInDir(targetDir, "pwd")
			if err != nil {
				errors <- fmt.Errorf("worker %d failed: %v", idx, err)
				return
			}

			// Cleaning output just in case
			output = strings.TrimSpace(output)

			// Verify that the output matches the expected directory
			// We check if targetDir is contained or equal, handling potential symlink resolution differences
			// But since we eval'd symlinks above, strict equality usually works best, or suffix check.
			if output != targetDir {
				// Fallback: check if they are the same file
				// Sometimes PWD returns logical path vs physical.
				// Let's rely on string comparison first.
				errors <- fmt.Errorf("worker %d race detected! Expected dir %q, got %q", idx, targetDir, output)
			}
		}(i, dirs[i])
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}
