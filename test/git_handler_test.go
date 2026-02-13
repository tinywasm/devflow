package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"os/exec"
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
