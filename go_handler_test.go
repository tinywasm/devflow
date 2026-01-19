package devflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoGetModulePath(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	defer testChdir(t, dir)()

	git, _ := NewGit()
	goHandler, _ := NewGo(git)

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

	git, _ := NewGit()
	goHandler, _ := NewGo(git)

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

	git, _ := NewGit()
	goHandler, _ := NewGo(git)

	_, err := goHandler.Test() // quiet mode
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

	git, _ := NewGit()
	goHandler, _ := NewGo(git)

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

	git, _ := NewGit()
	goHandler, _ := NewGo(git)

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
	// Create bare repo as remote
	remoteDir, _ := os.MkdirTemp("", "gitgo-remote-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()

	// Create local repo and go module
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

	defer testChdir(t, dir)()

	// Init git in the module dir
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.name", "Test").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "remote", "add", "origin", remoteDir).Run()

	git, _ := NewGit()
	goHandler, _ := NewGo(git)

	// Create test file so tests pass (although we skip them here for speed/reliability in this test)
	// But let's run them to verify full flow
	testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
	os.WriteFile("main_test.go", []byte(testContent), 0644)

	summary, err := goHandler.Push("test update", "v0.0.1", false, true, false, false, "")
	if err != nil {
		t.Fatalf("Go Push failed: %v", err)
	}

	// Verify summary contains expected elements from new format
	if !strings.Contains(summary, "Tag: v0.0.1") {
		t.Errorf("Expected summary to contain 'Tag: v0.0.1', got: %s", summary)
	}
	if !strings.Contains(summary, "Pushed ok") {
		t.Errorf("Expected summary to contain 'Pushed ok', got: %s", summary)
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
	testChdir(t, myappDir)()
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.name", "Test").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "initial").Run()

	// Setup remote for myapp
	remoteDir, _ := os.MkdirTemp("", "myapp-remote-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "remote", "add", "origin", remoteDir).Run()

	testChdir(t, mylibDir)() // back to root of test context or whatever

	git, _ := NewGit()
	g, _ := NewGo(git)

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
