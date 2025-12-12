package gitgo

import (
	"os"
	"os/exec"
	"path/filepath"
)

// testCreateGitRepo creates a temporary Git repo for tests
// For internal use in tests only
func testCreateGitRepo() (dir string, cleanup func()) {
	dir, _ = os.MkdirTemp("", "gitgo-test-")

	// Init git
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()

	cleanup = func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// testCreateGoModule creates a temporary Go module
func testCreateGoModule(moduleName string) (dir string, cleanup func()) {
	dir, _ = os.MkdirTemp("", "gitgo-gomod-")

	// Create go.mod
	gomod := "module " + moduleName + "\n\ngo 1.20\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	// Create main.go
	main := "package main\n\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0644)

	cleanup = func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}
