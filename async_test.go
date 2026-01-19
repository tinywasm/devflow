package devflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAsyncUpdateFlow(t *testing.T) {
	// Setup a complex directory structure:
	// /tmp/test-async/
	// ├── main/  (upstream module)
	// ├── dep1/  (depends on main)
	// ├── dep2/  (depends on main)

	tmp := t.TempDir()
	mainDir := filepath.Join(tmp, "main")
	dep1Dir := filepath.Join(tmp, "dep1")
	dep2Dir := filepath.Join(tmp, "dep2")

	os.MkdirAll(mainDir, 0755)
	os.MkdirAll(dep1Dir, 0755)
	os.MkdirAll(dep2Dir, 0755)

	// 1. Setup Main
	os.WriteFile(filepath.Join(mainDir, "go.mod"), []byte("module github.com/test/main\n\ngo 1.20\n"), 0644)
	os.WriteFile(filepath.Join(mainDir, "main.go"), []byte("package main\n"), 0644)

	// Init git for main
	runGit(t, mainDir, "init")
	runGit(t, mainDir, "config", "user.name", "Test")
	runGit(t, mainDir, "config", "user.email", "test@test.com")
	runGit(t, mainDir, "add", ".")
	runGit(t, mainDir, "commit", "-m", "initial")

	// Remote for main
	mainRemote, _ := os.MkdirTemp("", "main-remote-")
	defer os.RemoveAll(mainRemote)
	exec.Command("git", "init", "--bare", mainRemote).Run()
	runGit(t, mainDir, "remote", "add", "origin", mainRemote)

	// 2. Setup Dep1
	depContent := "module github.com/test/dep1\n\ngo 1.20\n\nrequire github.com/test/main v0.0.0\nreplace github.com/test/main => ../main\n"
	os.WriteFile(filepath.Join(dep1Dir, "go.mod"), []byte(depContent), 0644)
	// Git for dep1 (needed for internal push check)
	runGit(t, dep1Dir, "init")
	runGit(t, dep1Dir, "config", "user.name", "Test")
	runGit(t, dep1Dir, "config", "user.email", "test@test.com")
	runGit(t, dep1Dir, "add", ".")
	runGit(t, dep1Dir, "commit", "-m", "initial")

	// 3. Setup Dep2
	dep2Content := "module github.com/test/dep2\n\ngo 1.20\n\nrequire github.com/test/main v0.0.0\nreplace github.com/test/main => ../main\n"
	os.WriteFile(filepath.Join(dep2Dir, "go.mod"), []byte(dep2Content), 0644)
	runGit(t, dep2Dir, "init")
	runGit(t, dep2Dir, "config", "user.name", "Test")
	runGit(t, dep2Dir, "config", "user.email", "test@test.com")
	runGit(t, dep2Dir, "add", ".")
	runGit(t, dep2Dir, "commit", "-m", "initial")

	// 4. Initialize Handler on Main

	// Mock ExecCommand to prevent actual go get network calls that fail with "repository not found"
	// We restore it at the end of the test
	originalExec := ExecCommand
	defer func() { ExecCommand = originalExec }()

	ExecCommand = func(name string, args ...string) *exec.Cmd {
		// Mock go get, go mod tidy, and go list for our fake modules
		if name == "go" {
			// Join args to inspect
			cmdStr := strings.Join(args, " ")

			// Mock 'go list -m -json' for GetCurrentVersion logic
			if strings.Contains(cmdStr, "list -m -json") {
				// Return a fake JSON version
				return exec.Command("echo", `{"Version": "v0.0.0"}`)
			}

			// Mock 'go list -m' (module path detection)
			// This is CRITICAL because we are running in devflow root (not mocked dir),
			// so real 'go list -m' returns 'github.com/tinywasm/devflow', breaking dependency lookup.
			if cmdStr == "list -m" || (strings.Contains(cmdStr, "list") && strings.Contains(cmdStr, "-m") && !strings.Contains(cmdStr, "-json")) {
				return exec.Command("echo", "github.com/test/main")
			}

			// If it's attempting to get/tidy our fake test modules, succeed immediately
			if strings.Contains(cmdStr, "get") || strings.Contains(cmdStr, "tidy") {
				if strings.Contains(cmdStr, "github.com/test/main") || strings.Contains(cmdStr, "tidy") {
					// Return a dummy successful command (e.g. echo)
					return exec.Command("echo", "mock success")
				}
			}
		}
		// Pass through normal commands (git init, etc)
		return originalExec(name, args...)
	}

	// Switch context to main for the Git handler interaction
	// CRITICAL: Do NOT use os.Chdir as it affects other parallel tests and global state
	// Instead, ensure handlers use explicit root dirs.

	git, _ := NewGit()
	g, err := NewGo(git)
	if err != nil {
		t.Fatalf("NewGo failed: %v", err)
	}
	g.SetRootDir(mainDir)

	// 5. Execute Push
	// We skip tests/race/backup for speed.
	// Important: searchPath is ".." (the tmp root) so it finds dep1 and dep2
	// But since we are NOT in mainDir, ".." relative to CWD is wrong.
	// We must pass the absolute path to the TMP dir where dep1/dep2 live.
	summary, err := g.Push("feat: update main", "v0.0.2", true, true, false, true, tmp)

	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	t.Logf("Summary: %s", summary)

	// 6. Verify Async Results presence
	// Since 'go get' will fail (modules don't exist remotely), we expect failure messages
	// BUT we expect failures for BOTH dep1 and dep2, proving the async loop visited both.
	// (Or success if mocked)

	hasDep1 := strings.Contains(summary, "dep1")
	hasDep2 := strings.Contains(summary, "dep2")

	if !hasDep1 || !hasDep2 {
		t.Errorf("Summary should contain traces of both dependents. Got: %s", summary)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	// Use ExecCommand for consistency with mocking if needed,
	// though runGit is for setup where we might prefer real git.
	// Using generic exec.Command for setup is safer if our mock is too aggressive.
	// But our mock passes through unknown commands.
	cmd := ExecCommand("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Logf("git %v in %s failed: %v", args, dir, err)
	}
}
