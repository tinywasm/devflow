package gitgo

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGitHasChanges(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()

	// Create file
	os.WriteFile("test.txt", []byte("test"), 0644)

	// Add
	git.add()

	// Should have changes
	hasChanges, err := git.hasChanges()
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

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()

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
	git.createTag("v0.0.1")

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

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	git := NewGit()

	// Without changes should not fail
	_, err := git.commit("test")
	if err != nil {
		t.Error("Commit without changes should not fail")
	}

	// With changes
	os.WriteFile("test.txt", []byte("test changes"), 0644)
	git.add()

	// Check for changes
	has, _ := git.hasChanges()
	if !has {
		t.Fatal("Should have changes before commit")
	}

	committed, err := git.commit("test commit")
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

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	exec.Command("git", "remote", "add", "origin", remoteDir).Run()

	git := NewGit()
	os.WriteFile("README.md", []byte("# test"), 0644)

	summary, err := git.Push("initial commit", "v0.0.1")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if !strings.Contains(summary, "Pushed to remote") {
		t.Errorf("Expected summary to contain 'Pushed to remote', got: %s", summary)
	}
}
