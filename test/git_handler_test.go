package devflow_test

import "github.com/tinywasm/devflow"

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHasChanges(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Create file
	os.WriteFile("test.txt", []byte("test"), 0644)

	// Add
	git.Add()

	// Should have changes
	hasChanges, err := git.HasChanges()
	if err != nil {
		t.Fatal(err)
	}

	if !hasChanges {
		t.Error("Expected changes but got none")
	}
}

func TestGitHasPendingChanges_IgnoresEnvAndGitignore(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()
	// commit initial state
	os.WriteFile("README.md", []byte("# test"), 0644)
	git.Add()
	git.Commit("initial")

	// Case 1: Modify only .env -> Should return FALSE
	os.WriteFile(".env", []byte("foo=bar"), 0644)
	has, err := git.HasPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("Expected HasPendingChanges to be false when only .env is modified")
	}

	// Case 2: Modify only .gitignore -> Should return FALSE
	os.WriteFile(".gitignore", []byte("bin/"), 0644)
	has, err = git.HasPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("Expected HasPendingChanges to be false when only .gitignore is modified")
	}

	// Case 3: Modify .env AND a real file -> Should return TRUE
	os.WriteFile("main.go", []byte("package main"), 0644)
	has, err = git.HasPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("Expected HasPendingChanges to be true when a real file is modified")
	}
}

func TestGitGenerateNextTag(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Initial commit
	os.WriteFile("test.txt", []byte("test"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()

	// Without tags should return v0.0.1
	tag, err := git.GenerateNextTag()
	if err != nil {
		t.Fatal(err)
	}

	if tag != "v0.0.1" {
		t.Errorf("Expected v0.0.1, got %s", tag)
	}

	// Create tag
	git.CreateTag("v0.0.1")

	// Next should be v0.0.2
	tag, err = git.GenerateNextTag()
	if err != nil {
		t.Fatal(err)
	}

	if tag != "v0.0.2" {
		t.Errorf("Expected v0.0.2, got %s", tag)
	}
}

func TestGitCommit(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Without changes should not fail
	_, err := git.Commit("test")
	if err != nil {
		t.Error("Commit without changes should not fail")
	}

	// With changes
	os.WriteFile("test.txt", []byte("test changes"), 0644)
	git.Add()

	// Check for changes
	has, _ := git.HasChanges()
	if !has {
		t.Fatal("Should have changes before commit")
	}

	committed, err := git.Commit("test commit")
	if err != nil {
		t.Logf("Error content: %v", err)
		t.Fatalf("GitCommit failed: %v", err)
	}
	if !committed {
		t.Fatal("Should have committed")
	}

	// Verify commit happened
	out, err := exec.Command("git", "log", "-1", "--pretty=%B").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "test commit" {
		t.Errorf("Expected 'test commit', got '%s'", strings.TrimSpace(string(out)))
	}
}

func TestGitPush(t *testing.T) {
	// This test is tricky because it requires a remote.
	// We can mock the remote or just check if it fails gracefully or use a local remote.

	// Create bare repo as remote
	remoteDir, _ := os.MkdirTemp("", "gitgo-remote-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()

	// Create local repo
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	exec.Command("git", "remote", "add", "origin", "file://"+remoteDir).Run()

	git, _ := devflow.NewGit()
	os.WriteFile("README.md", []byte("# test"), 0644)

	result, err := git.Push("initial commit", "v0.0.1")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if !strings.Contains(result.Summary, "Pushed ok") {
		t.Errorf("Expected summary to contain 'Pushed ok', got: %s", result.Summary)
	}
}

func TestGitPushRejectsLowerTag(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Setup dummy remote for CheckRemoteAccess
	remoteDir, _ := os.MkdirTemp("", "gitgo-remote-reject-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "remote", "add", "origin", "file://"+remoteDir).Run()

	os.WriteFile("test.txt", []byte("initial"), 0644)
	git.Add()
	git.Commit("initial")
	git.CreateTag("v0.4.6")

	// Attempt push with lower tag
	_, err := git.Push("fix: something", "v0.0.51")
	if err == nil {
		t.Fatal("Expected error when pushing lower tag v0.0.51 after v0.4.6, but got nil")
	}

	expectedErr := "tag v0.0.51 is not greater than latest tag v0.4.6"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error containing %q, got %q", expectedErr, err.Error())
	}
}

func TestGitPushAcceptsHigherTag(t *testing.T) {
	// Create bare repo as remote
	remoteDir, _ := os.MkdirTemp("", "gitgo-remote-accept-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()

	dir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, dir)()

	exec.Command("git", "remote", "add", "origin", "file://"+remoteDir).Run()

	git, _ := devflow.NewGit()
	os.WriteFile("test.txt", []byte("initial"), 0644)
	git.Add()
	git.Commit("initial")
	git.CreateTag("v0.4.6")

	os.WriteFile("test.txt", []byte("update"), 0644)
	git.Add()
	result, err := git.Push("fix: something", "v0.4.7")
	if err != nil {
		t.Fatalf("Push failed for higher tag v0.4.7: %v", err)
	}

	if !strings.Contains(result.Summary, "v0.4.7") {
		t.Errorf("Expected summary to contain tag v0.4.7, got: %s", result.Summary)
	}
}

func TestGitGenerateNextTagErrors(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

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
	remoteDir, _ := os.MkdirTemp("", "gitgo-remote-upstream-")
	defer os.RemoveAll(remoteDir)
	exec.Command("git", "init", "--bare", remoteDir).Run()

	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Add remote
	exec.Command("git", "remote", "add", "origin", "file://"+remoteDir).Run()

	// Create commit
	os.WriteFile("test.txt", []byte("content"), 0644)
	git.Add()
	git.Commit("initial")

	// Create tag locally first!
	git.CreateTag("v0.0.1")

	// Test hasUpstream (should be false)
	has, err := git.HasUpstream()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("Should not have upstream yet")
	}

	// Test pushWithTags (should set upstream)
	_, err = git.PushWithTags("v0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	// Now should have upstream
	has, err = git.HasUpstream()
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

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Initial commit needed for tagging
	exec.Command("git", "commit", "--allow-empty", "-m", "init").Run()

	created, err := git.CreateTag("v0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("Expected tag to be created")
	}

	// Try to create again
	created, err = git.CreateTag("v0.0.1")
	if err == nil {
		t.Error("Expected error when creating existing tag")
	}
	if created {
		t.Error("Expected tag not to be created")
	}
}

func TestGitAddError(t *testing.T) {
	// We need to make git add fail.
	// One way is to lock the index file?
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Corrupt .git/index
	os.WriteFile(".git/index", []byte("garbage"), 0000)

	err := git.Add()
	if err == nil {
		t.Error("Expected git add to fail with corrupt index")
	}
}

func TestGitPushCommitFailure(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Stage file
	os.WriteFile("test.txt", []byte("content"), 0644)
	git.Add()

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

func TestGetLatestTagSemverOrder(t *testing.T) {
	// This test reproduces the production bug where tags exist
	// out of commit order and GetLatestTag must return the HIGHEST
	// semver tag, not just the closest reachable from HEAD.
	dir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Create commits and tags in sequence
	exec.Command("git", "commit", "--allow-empty", "-m", "c1").Run()
	exec.Command("git", "tag", "v0.0.88").Run()

	exec.Command("git", "commit", "--allow-empty", "-m", "c2").Run()
	exec.Command("git", "tag", "v0.1.0").Run()

	exec.Command("git", "commit", "--allow-empty", "-m", "c3").Run()
	exec.Command("git", "tag", "v0.0.89").Run()

	// GetLatestTag MUST return v0.1.0 (highest semver),
	// NOT v0.0.89 (closest to HEAD)
	tag, err := git.GetLatestTag()
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.1.0" {
		t.Errorf("Expected v0.1.0 (highest semver), got %s", tag)
	}
}

func TestGenerateNextTagWithOutOfOrderTags(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, dir)()

	git, _ := devflow.NewGit()

	// Create tags out of order (simulates the production bug)
	exec.Command("git", "commit", "--allow-empty", "-m", "c1").Run()
	exec.Command("git", "tag", "v0.0.88").Run()

	exec.Command("git", "commit", "--allow-empty", "-m", "c2").Run()
	exec.Command("git", "tag", "v0.1.0").Run()

	exec.Command("git", "commit", "--allow-empty", "-m", "c3").Run()
	exec.Command("git", "tag", "v0.0.89").Run()

	// Must generate v0.1.1 (increment from highest: v0.1.0)
	tag, err := git.GenerateNextTag()
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.1.1" {
		t.Errorf("Expected v0.1.1, got %s", tag)
	}
}

func TestGitClone_IdempotentAndRootDir(t *testing.T) {
	// 1. Create a bare source repository with a commit
	srcDir := t.TempDir()
	exec.Command("git", "init", "--bare", srcDir).Run()

	// Seed the bare repo with a commit via a temporary clone
	seedDir := t.TempDir()
	exec.Command("git", "clone", srcDir, seedDir).Run()
	os.WriteFile(filepath.Join(seedDir, "file.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", seedDir, "add", ".").Run()
	exec.Command("git", "-C", seedDir, "commit", "-m", "initial").Run()
	exec.Command("git", "-C", seedDir, "push", "origin", "HEAD").Run()

	// 2. Clone into a target directory using SetRootDir
	targetDir := filepath.Join(t.TempDir(), "target_repo")
	git, err := devflow.NewGit()
	if err != nil {
		t.Fatal(err)
	}
	git.SetRootDir(targetDir)

	// First clone -> should clone and return alreadyPresent == false
	alreadyPresent, err := git.Clone(srcDir)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if alreadyPresent {
		t.Error("Expected alreadyPresent to be false on first clone")
	}

	// Verify file exists
	content, err := os.ReadFile(filepath.Join(targetDir, "file.txt"))
	if err != nil || string(content) != "hello" {
		t.Fatalf("Expected file.txt content 'hello', got %q, err: %v", string(content), err)
	}

	// Second clone -> should be idempotent, return alreadyPresent == true, err == nil, without modifying working tree
	alreadyPresent, err = git.Clone(srcDir)
	if err != nil {
		t.Fatalf("Second clone failed: %v", err)
	}
	if !alreadyPresent {
		t.Error("Expected alreadyPresent to be true on second clone")
	}
}

// TestGitClone_EmptyDirNestedInUnrelatedRepo guards against detecting
// "already present" via the parent directory's repo instead of rootDir's own
// .git — an empty rootDir must always be cloned into, regardless of what
// git repo (if any) contains it.
func TestGitClone_EmptyDirNestedInUnrelatedRepo(t *testing.T) {
	srcDir := t.TempDir()
	exec.Command("git", "init", "--bare", srcDir).Run()

	seedDir := t.TempDir()
	exec.Command("git", "clone", srcDir, seedDir).Run()
	os.WriteFile(filepath.Join(seedDir, "file.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", seedDir, "add", ".").Run()
	exec.Command("git", "-C", seedDir, "commit", "-m", "initial").Run()
	exec.Command("git", "-C", seedDir, "push", "origin", "HEAD").Run()

	// unrelated parent repo, with an empty subdirectory that has no .git of its own
	parentDir := t.TempDir()
	exec.Command("git", "init", parentDir).Run()
	targetDir := filepath.Join(parentDir, "sub", "target_repo")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	git, err := devflow.NewGit()
	if err != nil {
		t.Fatal(err)
	}
	git.SetRootDir(targetDir)

	alreadyPresent, err := git.Clone(srcDir)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if alreadyPresent {
		t.Fatal("Expected alreadyPresent to be false: targetDir has no .git of its own")
	}

	content, err := os.ReadFile(filepath.Join(targetDir, "file.txt"))
	if err != nil || string(content) != "hello" {
		t.Fatalf("Expected file.txt content 'hello', got %q, err: %v", string(content), err)
	}
}

func TestGitPull_CleanAndDirty(t *testing.T) {
	// Setup remote bare repo
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()

	// Setup local repo
	localDir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, localDir)()

	exec.Command("git", "remote", "add", "origin", remoteDir).Run()
	os.WriteFile("README.md", []byte("# initial"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()
	exec.Command("git", "push", "-u", "origin", "HEAD").Run()

	// Push a new commit to remote from a secondary clone
	secDir := t.TempDir()
	exec.Command("git", "clone", remoteDir, secDir).Run()
	os.WriteFile(filepath.Join(secDir, "remote_file.txt"), []byte("from remote"), 0644)
	exec.Command("git", "-C", secDir, "add", ".").Run()
	exec.Command("git", "-C", secDir, "commit", "-m", "remote commit").Run()
	exec.Command("git", "-C", secDir, "push", "origin", "HEAD").Run()

	git, _ := devflow.NewGit()

	// Case A: Dirty worktree -> Pull must fail with ErrDirtyWorkTree and leave worktree intact
	os.WriteFile("local_dirty.txt", []byte("uncommitted"), 0644)
	err := git.Pull()
	if err == nil {
		t.Fatal("Expected Pull to fail on dirty worktree, got nil")
	}
	if !errors.Is(err, devflow.ErrDirtyWorkTree) {
		t.Errorf("Expected ErrDirtyWorkTree, got: %v", err)
	}
	// Verify remote_file.txt was not pulled during dirty pull
	if _, err := os.Stat("remote_file.txt"); !os.IsNotExist(err) {
		t.Error("Expected remote_file.txt to not exist after dirty pull")
	}

	// Case B: Clean worktree -> Pull succeeds and updates worktree
	os.Remove("local_dirty.txt")
	err = git.Pull()
	if err != nil {
		t.Fatalf("Pull failed on clean worktree: %v", err)
	}
	content, err := os.ReadFile("remote_file.txt")
	if err != nil || string(content) != "from remote" {
		t.Errorf("Expected remote_file.txt 'from remote', got %q, err: %v", string(content), err)
	}
}

func TestGitFetch(t *testing.T) {
	// Setup remote bare repo
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()

	// Setup local repo
	localDir, cleanup := testCreateGitRepo()
	defer cleanup()
	defer testChdir(t, localDir)()

	exec.Command("git", "remote", "add", "origin", remoteDir).Run()
	os.WriteFile("README.md", []byte("# initial"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()
	exec.Command("git", "push", "-u", "origin", "HEAD").Run()

	// Push a new commit to remote from a secondary clone
	secDir := t.TempDir()
	exec.Command("git", "clone", remoteDir, secDir).Run()
	os.WriteFile(filepath.Join(secDir, "remote_file.txt"), []byte("from remote"), 0644)
	exec.Command("git", "-C", secDir, "add", ".").Run()
	exec.Command("git", "-C", secDir, "commit", "-m", "remote commit").Run()
	exec.Command("git", "-C", secDir, "push", "origin", "HEAD").Run()

	git, _ := devflow.NewGit()

	// Call Fetch
	err := git.Fetch()
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Working tree must NOT be updated (remote_file.txt should not exist)
	if _, err := os.Stat("remote_file.txt"); !os.IsNotExist(err) {
		t.Error("Working tree was modified by Fetch; remote_file.txt should not exist yet")
	}

	// Remote tracking ref should have the new commit
	out, err := exec.Command("git", "log", "-1", "--pretty=%B", "@{u}").Output()
	if err != nil || !strings.Contains(string(out), "remote commit") {
		t.Errorf("Expected fetch to update remote ref, got commit msg %q, err: %v", string(out), err)
	}
}
