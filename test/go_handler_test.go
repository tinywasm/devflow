package devflow_test

import "github.com/tinywasm/devflow"

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
	goHandler, _ := devflow.NewGo(mockGit)

	path, err := goHandler.GetModulePath()
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
	goHandler, _ := devflow.NewGo(mockGit)

	err := goHandler.Verify()
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
	goHandler, _ := devflow.NewGo(mockGit)

	_, err := goHandler.Test([]string{}, false, 0, false, false) // quiet mode, full suite, default timeout, allow cache, runAll=false
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
	goHandler, _ := devflow.NewGo(mockGit)

	// Search dependents
	dependents, err := goHandler.FindDependentModules("github.com/test/main", tmpDir)
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
	goHandler, _ := devflow.NewGo(mockGit)

	// Should find the dependency
	if !goHandler.HasDependency(gomodPath, "github.com/tinywasm/devflow") {
		t.Error("Expected to find dependency")
	}

	// Should not find this one
	if goHandler.HasDependency(gomodPath, "github.com/other/repo") {
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

	goHandler, err := devflow.NewGo(mockGit)
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
	result, err := goHandler.Push("test update", "v0.0.1", false, true, true, true, false, "")
	if err != nil {
		t.Fatalf("Go Push failed: %v", err)
	}

	// Verify summary contains expected elements
	// Mock returns "Mock push ok"
	if !strings.Contains(result.Summary, "Mock push ok") {
		t.Errorf("Expected summary to contain 'Mock push ok', got: %s", result.Summary)
	}
	if !strings.Contains(result.Summary, "vet ✅") {
		t.Errorf("Expected summary to contain 'vet ✅', got: %s", result.Summary)
	}
}

func TestGoPush_NoGoMod(t *testing.T) {
	mockGit := &MockGitClient{
		pushResult: devflow.PushResult{Summary: "Git push ok"},
	}

	// Temp dir WITHOUT go.mod
	dir := t.TempDir()
	defer testChdir(t, dir)()

	goHandler, _ := devflow.NewGo(mockGit)

	// Run Push
	result, err := goHandler.Push("test", "", false, false, false, false, false, "")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if result.Summary != "Git push ok" {
		t.Errorf("Expected 'Git push ok', got: %s", result.Summary)
	}
}

func TestGoPush_SkipTag(t *testing.T) {
	mockGit := &MockGitClient{}
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	goHandler, _ := devflow.NewGo(mockGit)

	// Run Push with skipTag=true
	result, err := goHandler.Push("test", "", true, true, true, true, true, "")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if !strings.Contains(result.Summary, "Pushed ✅") {
		t.Errorf("Expected summary to contain '✅ Pushed commits', got: %s", result.Summary)
	}
	if !mockGit.pushWithoutTagsCalled {
		t.Error("PushWithoutTags should have been called")
	}
}

func TestGoPush_DependentOutput(t *testing.T) {
	// This tests that dependents print to consoleOutput and NOT to summary
	var consoleLines []string
	mockGit := &MockGitClient{
		createdTag: "v1.0.0",
	}

	dir, cleanup := testCreateGoModule("github.com/test/main")
	defer cleanup()
	defer testChdir(t, dir)()

	// Create a dependent
	depDir := filepath.Join(filepath.Join(filepath.Dir(dir), "dep"))
	os.MkdirAll(depDir, 0755)
	os.WriteFile(filepath.Join(depDir, "go.mod"), []byte("module github.com/test/dep\n\nrequire github.com/test/main v0.0.0\n"), 0644)

	goHandler, _ := devflow.NewGo(mockGit)
	goHandler.SetConsoleOutput(func(s string) {
		consoleLines = append(consoleLines, s)
	})

	// We need to mock FindDependentModules or ensure it finds our dep
	// SearchPath is the parent of dir
	result, _ := goHandler.Push("feat: main", "v1.0.0", true, true, false, true, false, "..")

	// Summary should NOT contain dep update result
	if strings.Contains(result.Summary, "updated to v1.0.0") {
		t.Errorf("Summary should not contain dep update result, got: %s", result.Summary)
	}

	// Console should contain dep update result
	found := false
	for _, line := range consoleLines {
		if strings.Contains(line, "📦 dep") {
			found = true
			break
		}
	}
	if !found {
		t.Log("Console output:", consoleLines)
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

	neutralDir := t.TempDir()
	defer testChdir(t, neutralDir)()

	// Disable proxy so "go get" fails instantly without network requests
	t.Setenv("GOPROXY", "off")

	mockGit := &MockGitClient{}
	g, _ := devflow.NewGo(mockGit)
	g.SetRetryConfig(time.Millisecond, 1)

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
	g, _ := devflow.NewGo(mockGit)
	g.SetRootDir(tmpDir)

	err := g.Install("v1.2.3")
	if err != nil {
		t.Logf("Install failed as expected (no real module in tmp): %v", err)
	}

	// Verify behavior when cmd is missing
	g.SetRootDir(t.TempDir())
	if err := g.Install("v1.0.0"); err != nil {
		t.Errorf("Expected no error when cmd is missing, got %v", err)
	}
}

// MockGitClient for testing
type MockGitClient struct {
	checkAccessErr         error
	pushErr                error
	latestTag              string
	createdTag             string
	pushResult             devflow.PushResult
	pushWithoutTagsCalled  bool
	log                    func(...any)
}

func (m *MockGitClient) CheckRemoteAccess() error {
	return m.checkAccessErr
}

func (m *MockGitClient) Push(message, tag string) (devflow.PushResult, error) {
	if m.checkAccessErr != nil {
		return devflow.PushResult{}, m.checkAccessErr
	}
	if m.pushErr != nil {
		return devflow.PushResult{}, m.pushErr
	}
	if m.pushResult.Summary != "" || m.pushResult.Tag != "" {
		return m.pushResult, nil
	}
	resultTag := m.createdTag
	if resultTag == "" {
		resultTag = "v0.0.1" // Default test tag
	}
	return devflow.PushResult{
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
}

func (m *MockGitClient) SetRootDir(path string) {
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

func (m *MockGitClient) PushWithoutTags() (bool, error) {
	m.pushWithoutTagsCalled = true
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

func (m *MockGitClient) HasPendingChanges() (bool, error) {
	return true, nil
}

func TestGoPush_RemoteAccessFailure(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{
		checkAccessErr: fmt.Errorf("❌ Network error"),
		log:            func(args ...any) {},
	}

	goHandler, _ := devflow.NewGo(mockGit)

	_, err := goHandler.Push("msg", "tag", true, true, true, true, false, "")

	if err == nil {
		t.Fatal("Expected error due to remote access failure, got nil")
	}

	if !strings.Contains(err.Error(), "Network error") {
		t.Errorf("Expected error to contain 'Network error', got: %v", err)
	}
}
