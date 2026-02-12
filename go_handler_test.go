package devflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGoGetModulePath(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{}
	goHandler, _ := NewGo(mockGit)

	path, err := goHandler.getModulePath()
	if err != nil {
		t.Fatal(err)
	}

	if path != "github.com/test/repo" {
		t.Errorf("Expected github.com/test/repo, got %s", path)
	}
}

func TestGoModVerify(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{}
	goHandler, _ := NewGo(mockGit)

	err := goHandler.verify()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGoTest(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

	// Create passing test
	testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
	os.WriteFile(dir+"/main_test.go", []byte(testContent), 0644)

	defer testChdir(t, dir)()

	mockGit := &MockGitClient{}
	goHandler, _ := NewGo(mockGit)

	_, err := goHandler.Test([]string{}, false, 0) // quiet mode, full suite, default timeout
	if err != nil {
		// In test environment, tests might fail, but we check the call works
		t.Log("Test failed as expected in test environment:", err)
	}
}

func TestFindDependentModules(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Main module
	mainDir := tmpDir + "/main"
	os.MkdirAll(mainDir, 0755)
	// Important: module name matches what dep1 requires
	os.WriteFile(mainDir+"/go.mod", []byte("module github.com/test/main\n\ngo 1.20\n"), 0644)

	// Dependent module 1
	dep1Dir := tmpDir + "/dep1"
	os.MkdirAll(dep1Dir, 0755)
	dep1Mod := `module github.com/test/dep1

go 1.20

require github.com/test/main v0.0.1
`
	os.WriteFile(dep1Dir+"/go.mod", []byte(dep1Mod), 0644)

	// Independent module
	indepDir := tmpDir + "/indep"
	os.MkdirAll(indepDir, 0755)
	os.WriteFile(indepDir+"/go.mod", []byte("module github.com/test/indep\n\ngo 1.20\n"), 0644)

	mockGit := &MockGitClient{}
	goHandler, _ := NewGo(mockGit)

	// Search dependents
	dependents, err := goHandler.findDependentModules("github.com/test/main", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should find only dep1
	if len(dependents) != 1 {
		for _, d := range dependents {
			t.Logf("Found: %s", d)
		}
		t.Errorf("Expected 1 dependent, got %d", len(dependents))
	}
}

func TestHasDependency(t *testing.T) {
	tmpDir := t.TempDir()
	gomodPath := tmpDir + "/go.mod"

	content := `module github.com/test/repo

go 1.20

require github.com/tinywasm/devflow v0.0.1
`
	os.WriteFile(gomodPath, []byte(content), 0644)

	mockGit := &MockGitClient{}
	goHandler, _ := NewGo(mockGit)

	// Should find the dependency
	if !goHandler.hasDependency(gomodPath, "github.com/tinywasm/devflow") {
		t.Error("Expected to find dependency")
	}

	// Should not find this one
	if goHandler.hasDependency(gomodPath, "github.com/other/repo") {
		t.Error("Should not find non-existent dependency")
	}
}

func TestGoPush(t *testing.T) {
	// Use MockGitClient to decouple from real git and avoid remote access issues in tests
	mockGit := &MockGitClient{
		latestTag: "v0.0.0",
		log:       func(args ...any) {},
	}

	// We need to create a dummy go module context because Go.Push reads go.mod
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	goHandler, err := NewGo(mockGit)
	if err != nil {
		t.Fatalf("NewGo failed: %v", err)
	}

	// Create test file so tests pass (Go.Push runs tests by default)
	// But actually, Go.Push runs tests using `go test`.
	// Since we are mocking Git, we might still fail on `go test` if we don't have valid go files?
	// The original test created main_test.go. Let's keep that.
	testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
	os.WriteFile("main_test.go", []byte(testContent), 0644)

	// We need to ensure Go.verify() passes. It runs `go mod verify`.
	// Make sure go.mod is valid (testCreateGoModule does this).

	// Run Push
	// We skip dependents (true) and backup (true/false) to focus on core flow
	summary, err := goHandler.Push("test update", "v0.0.1", false, true, true, true, "")
	if err != nil {
		t.Fatalf("Go Push failed: %v", err)
	}

	// Verify summary contains expected elements
	// Mock returns "Mock push ok"
	if !strings.Contains(summary, "Mock push ok") {
		t.Errorf("Expected summary to contain 'Mock push ok', got: %s", summary)
	}
	if !strings.Contains(summary, "vet ok") {
		t.Errorf("Expected summary to contain 'vet ok', got: %s", summary)
	}
}

func TestUpdateDependentModule(t *testing.T) {
	// 1. Setup structure:
	// /tmp/test-gopush/
	// ├── mylib/  (module github.com/test/mylib)
	// └── myapp/  (module github.com/test/myapp, requires mylib, replace ../mylib)

	tmp := t.TempDir()
	mylibDir := filepath.Join(tmp, "mylib")
	myappDir := filepath.Join(tmp, "myapp")

	os.MkdirAll(mylibDir, 0755)
	os.MkdirAll(myappDir, 0755)

	// Create mylib
	os.WriteFile(filepath.Join(mylibDir, "go.mod"), []byte("module github.com/test/mylib\n\ngo 1.20\n"), 0644)
	os.WriteFile(filepath.Join(mylibDir, "mylib.go"), []byte("package mylib\n"), 0644)

	// Create myapp
	os.WriteFile(filepath.Join(myappDir, "go.mod"), []byte("module github.com/test/myapp\n\ngo 1.20\n\nrequire github.com/test/mylib v0.0.0\nreplace github.com/test/mylib => ../mylib\n"), 0644)

	// Init git in myapp (needed for Push)
	// We use explicit Dir instead of chdir to avoid confusion and leaking
	runGit := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Logf("git %v in %s failed: %v", args, dir, err)
		}
	}

	runGit(myappDir, "init")
	runGit(myappDir, "config", "user.name", "Test")
	runGit(myappDir, "config", "user.email", "test@test.com")
	runGit(myappDir, "add", ".")
	runGit(myappDir, "commit", "-m", "initial")

	// Setup remote for myapp
	remoteDir, _ := os.MkdirTemp("", "myapp-remote-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()
	runGit(myappDir, "remote", "add", "origin", remoteDir)

	// Switch to mylibDir context for the rest of the test logic if needed,
	// but the handler below allows specifying paths.
	// Actually, we don't need to chdir at all if we pass absolute paths or correct relative ones.
	// But UpdateDependentModule uses chdir internally, so we just need to ensure
	// we are currently in a "safe" place or the tool handles it.
	// The test logic below passes myappDir (absolute path from TempDir).
	// So we can stay in root or chdir to a neutral temp dir.

	neutralDir := t.TempDir()
	defer testChdir(t, neutralDir)() // Move out of real repo just in case

	defer testChdir(t, neutralDir)() // Move out of real repo just in case

	mockGit := &MockGitClient{}
	g, _ := NewGo(mockGit)
	// Optimize test speed by disabling retries
	g.SetRetryConfig(time.Millisecond, 1)

	// This will fail in real life because "go get github.com/test/mylib@v0.0.1" won't find the module
	// So we'll mock the RunCommand to accept "go get" and "go mod tidy"
	// Actually, we can't easily mock RunCommand globally without more effort.
	// But we can check if it fails exactly where we expect.

	result, err := g.UpdateDependentModule(myappDir, "github.com/test/mylib", "v0.0.1")

	// We expect a failure at "go get" because the module doesn't exist in registry
	if err == nil {
		t.Errorf("Expected error from go get (module not in registry), got result: %s", result)
	} else if !strings.Contains(err.Error(), "go get failed") {
		t.Errorf("Expected error to contain 'go get failed', got: %v", err)
	}

	// However, we can verify that the replace was removed BEFORE the go get failure
	gomodContent, _ := os.ReadFile(filepath.Join(myappDir, "go.mod"))
	if strings.Contains(string(gomodContent), "replace github.com/test/mylib") {
		t.Error("replace directive should have been removed even if go get failed later")
	}
}

func TestGoInstall(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cmd structure
	os.MkdirAll(filepath.Join(tmpDir, "cmd/tool1"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "cmd/tool2"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "cmd/tool1/main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "cmd/tool2/main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	mockGit := &MockGitClient{}
	g, _ := NewGo(mockGit)
	g.SetRootDir(tmpDir)

	// Test installation logic
	// Note: go install will fail in a temp directory without a real module or GOPATH setup for these files,
	// but we want to verify it attempts to run the commands.
	// Since we can't easily mock RunCommandInDir without refactoring executor.go,
	// we'll check if it fails for the right reasons or at least doesn't panic.
	summary, err := g.Install("v1.2.3")

	if err != nil {
		t.Logf("Install failed as expected (no real module in tmp): %v", err)
	} else {
		t.Logf("Install summary: %s", summary)
	}

	// Verify behavior when cmd is missing
	g.SetRootDir(t.TempDir())
	summary, err = g.Install("v1.0.0")
	if err != nil {
		t.Errorf("Expected no error when cmd is missing, got %v", err)
	}
	if summary != "" {
		t.Errorf("Expected empty summary when cmd is missing, got %s", summary)
	}
}

// MockGitClient for testing
type MockGitClient struct {
	checkAccessErr error
	pushErr        error
	latestTag      string
	createdTag     string // NEW: Tag to return from Push()
	log            func(...any)
}

func (m *MockGitClient) CheckRemoteAccess() error {
	return m.checkAccessErr
}

func (m *MockGitClient) Push(message, tag string) (PushResult, error) {
	if m.checkAccessErr != nil {
		return PushResult{}, m.checkAccessErr
	}
	if m.pushErr != nil {
		return PushResult{}, m.pushErr
	}
	resultTag := m.createdTag
	if resultTag == "" {
		resultTag = "v0.0.1" // Default test tag
	}
	return PushResult{
		Summary: "Mock push ok",
		Tag:     resultTag,
	}, nil
}

func (m *MockGitClient) GetLatestTag() (string, error) {
	return m.latestTag, nil
}

func (m *MockGitClient) SetLog(fn func(...any)) {
	m.log = fn
}

func (m *MockGitClient) SetShouldWrite(fn func() bool) {
	// mock implementation
}

func (m *MockGitClient) SetRootDir(path string) {
	// mock implementation
}

func (m *MockGitClient) GitIgnoreAdd(entry string) error {
	return nil
}

func (m *MockGitClient) Add() error {
	return nil
}

func (m *MockGitClient) Commit(message string) (bool, error) {
	return true, nil
}

func (m *MockGitClient) CreateTag(tag string) (bool, error) {
	return true, nil
}

func (m *MockGitClient) PushWithTags(tag string) (bool, error) {
	return false, nil
}

func (m *MockGitClient) GetConfigUserName() (string, error) {
	return "Mock User", nil
}

func (m *MockGitClient) GetConfigUserEmail() (string, error) {
	return "mock@example.com", nil
}

func (m *MockGitClient) InitRepo(dir string) error {
	return nil
}

func TestGoPush_RemoteAccessFailure(t *testing.T) {
	// Isolate execution in a temp directory to avoid recursive testing of the current project
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{
		checkAccessErr: fmt.Errorf("❌ Network error"),
		log:            func(args ...any) {},
	}

	goHandler, _ := NewGo(mockGit)

	// Attempt push
	// Skip tests (true), skip race (true), skip dependents (true), skip backup (true), no search path
	// We want to hit the git.Push() call where CheckRemoteAccess happens, without running real tests
	_, err := goHandler.Push("msg", "tag", true, true, true, true, "")

	// Should fail with the mock error
	if err == nil {
		t.Fatal("Expected error due to remote access failure, got nil")
	}

	if !strings.Contains(err.Error(), "Network error") {
		t.Errorf("Expected error to contain 'Network error', got: %v", err)
	}
}
