package gitgo

import (
	"os"
	"os/exec"
	"testing"
)

func TestGitGenerateNextTagErrors(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()

    // Test with invalid tag format
    // Force a tag with invalid format
    exec.Command("git", "commit", "--allow-empty", "-m", "init").Run()
    exec.Command("git", "tag", "invalid-tag").Run()

    tag, err := git.GenerateNextTag()
    // It might return error or default?
    // Code says: if parts < 3 return error "invalid tag format"
    if err == nil {
        t.Errorf("Expected error for invalid tag format, got %s", tag)
    }

    // Test with non-integer patch version
    exec.Command("git", "tag", "-d", "invalid-tag").Run()
    exec.Command("git", "tag", "v1.0.abc").Run()

    _, err = git.GenerateNextTag()
    if err == nil {
        t.Error("Expected error for non-integer patch version")
    }
}

func TestGitPushWithUpstreamLogic(t *testing.T) {
    // This requires a remote
    remoteDir, _ := os.MkdirTemp("", "gitgo-remote-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()

	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()

    // Add remote
	exec.Command("git", "remote", "add", "origin", remoteDir).Run()

    // Create commit
    os.WriteFile("test.txt", []byte("content"), 0644)
    git.add()
    git.commit("initial")

    // Create tag locally first!
    git.createTag("v0.0.1")

    // Test hasUpstream (should be false)
    has, err := git.hasUpstream()
    if err != nil {
        t.Fatal(err)
    }
    if has {
        t.Error("Should not have upstream yet")
    }

    // Test pushWithTags (should set upstream)
    err = git.pushWithTags("v0.0.1")
    if err != nil {
        t.Fatal(err)
    }

    // Now should have upstream
    has, err = git.hasUpstream()
    if err != nil {
        t.Fatal(err)
    }
    if !has {
        t.Error("Should have upstream now")
    }
}

func TestGitCreateTagExists(t *testing.T) {
    dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()

    // Initial commit needed for tagging
    exec.Command("git", "commit", "--allow-empty", "-m", "init").Run()

    created, err := git.createTag("v0.0.1")
    if err != nil {
        t.Fatal(err)
    }
    if !created {
        t.Error("Expected tag to be created")
    }

    // Try to create again
    created, err = git.createTag("v0.0.1")
    if err == nil {
        t.Error("Expected error when creating existing tag")
    }
    if created {
        t.Error("Expected tag not to be created")
    }
}

func TestGoPushFlags(t *testing.T) {
    dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

    oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    // Init git
    exec.Command("git", "init").Run()
    exec.Command("git", "config", "user.name", "Test").Run()
    exec.Command("git", "config", "user.email", "test@test.com").Run()

    // Mock remote to avoid push errors
    remoteDir, _ := os.MkdirTemp("", "gitgo-remote-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()
    exec.Command("git", "remote", "add", "origin", remoteDir).Run()

    git := NewGit()
    goHandler := NewGo(git)

    // 1. Skip Tests and Skip Race
    summary, err := goHandler.Push("msg", "v0.0.1", true, true, "")
    if err != nil {
        t.Fatal(err)
    }
    if summary == "" {
        t.Error("Empty summary")
    }

    // 2. Run Tests, Skip Race
    // Create dummy test
     testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
	os.WriteFile("main_test.go", []byte(testContent), 0644)

    summary, err = goHandler.Push("msg", "v0.0.2", false, true, "")
    if err != nil {
        t.Fatal(err)
    }

    // 3. Run Tests, Run Race
    summary, err = goHandler.Push("msg", "v0.0.3", false, false, "")
    if err != nil {
        t.Fatal(err)
    }
}

func TestGoUpdateDependentsNoSearchPath(t *testing.T) {
    // Test that default search path ".." works (or at least is used)
    // We can just call it in a dir structure where .. has nothing relevant
     dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

    oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()
    goHandler := NewGo(git)

    // It should not fail, just find nothing
    updated, err := goHandler.updateDependents("github.com/test/repo", "v0.0.1", "")
    if err != nil {
        t.Fatal(err)
    }
    if updated != 0 {
        t.Errorf("Expected 0 updated, got %d", updated)
    }
}

func TestGoFailures(t *testing.T) {
    dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

    oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()
    goHandler := NewGo(git)

    // Test Verify Failure (delete go.mod)
    os.Remove("go.mod")
    err := goHandler.verify()
    if err == nil {
        t.Error("Expected verify to fail when go.mod is missing")
    }

    // Restore go.mod for next steps
    os.WriteFile("go.mod", []byte("module github.com/test/repo\n\ngo 1.20\n"), 0644)

    // Test GetModulePath Failure (corrupt go.mod)
    os.WriteFile("go.mod", []byte("invalid content"), 0644)
    _, err = goHandler.getModulePath()
    if err == nil {
        t.Error("Expected getModulePath to fail with invalid content")
    }
}

func TestGoUpdateModuleFail(t *testing.T) {
     dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

    oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()
    goHandler := NewGo(git)

    // Try to update a module in current dir (which is not a valid dependent in this context, or just fails `go get`)
    // We try to run updateModule on the current directory for a non-existent dependency

    err := goHandler.updateModule(".", "github.com/nonexistent/dep", "v1.0.0")
    if err == nil {
        t.Error("Expected updateModule to fail")
    }
}

func TestGoPushFailTest(t *testing.T) {
    dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

    oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()
    goHandler := NewGo(git)

    // Create failing test
    testContent := `package main
import "testing"
func TestFail(t *testing.T) { t.Fatal("fail") }
`
	os.WriteFile("main_test.go", []byte(testContent), 0644)

    _, err := goHandler.Push("msg", "", false, false, "")
    if err == nil {
        t.Error("Expected Push to fail due to failed tests")
    }
}

// Add one more test case for edge cases in executor
func TestExecutorErrors(t *testing.T) {
    // Run invalid command
    _, err := RunCommand("invalid_command_xyz")
    if err == nil {
        t.Error("Expected error for invalid command")
    }
}

func TestGitAddError(t *testing.T) {
    // We need to make git add fail.
    // One way is to lock the index file?
    dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()

    // Corrupt .git/index
    os.WriteFile(".git/index", []byte("garbage"), 0000)

    err := git.add()
    if err == nil {
        t.Error("Expected git add to fail with corrupt index")
    }
}

func TestGitPushCommitFailure(t *testing.T) {
    dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

    git := NewGit()

    // Stage file
    os.WriteFile("test.txt", []byte("content"), 0644)
    git.add()

    // Create failing pre-commit hook
    os.MkdirAll(".git/hooks", 0755)
    hook := `#!/bin/sh
exit 1
`
    os.WriteFile(".git/hooks/pre-commit", []byte(hook), 0755)

    // Push should fail at commit step
    _, err := git.Push("msg", "")
    if err == nil {
        t.Error("Expected Push to fail at commit step")
    }
}
