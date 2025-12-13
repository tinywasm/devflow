package gitgo

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGoGetModulePath(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()
	goHandler := NewGo(git)

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
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()
	goHandler := NewGo(git)

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

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()
	goHandler := NewGo(git)

	err := goHandler.test()
	if err != nil {
		t.Fatal(err)
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

	git := NewGit()
	goHandler := NewGo(git)

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

require github.com/cdvelop/gitgo v0.0.1
`
	os.WriteFile(gomodPath, []byte(content), 0644)

	git := NewGit()
	goHandler := NewGo(git)

	// Should find the dependency
	if !goHandler.hasDependency(gomodPath, "github.com/cdvelop/gitgo") {
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

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Init git in the module dir
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.name", "Test").Run()
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	exec.Command("git", "remote", "add", "origin", remoteDir).Run()

	git := NewGit()
	goHandler := NewGo(git)

	// Create test file so tests pass (although we skip them here for speed/reliability in this test)
	// But let's run them to verify full flow
	testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
	os.WriteFile("main_test.go", []byte(testContent), 0644)

	summary, err := goHandler.Push("test update", "v0.0.1", false, true, "")
	if err != nil {
		t.Fatalf("Go Push failed: %v", err)
	}

	if !strings.Contains(summary, "Git Push") {
		t.Errorf("Expected summary to contain 'Git Push', got: %s", summary)
	}
	if !strings.Contains(summary, "Tests passed") {
		t.Errorf("Expected summary to contain 'Tests passed', got: %s", summary)
	}
}
