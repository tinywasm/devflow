package gitgo

import (
	"os"
	"os/exec"
	"testing"
)

func TestWorkflowPush(t *testing.T) {
	// Create temporary repo
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	// Init git
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()

	// Change to directory
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	h := NewGitHandler()

	// Create file
	os.WriteFile("test.txt", []byte("test"), 0644)

	// Execute workflow (without real push to avoid remote error)
	// Only test up to tag
	h.Add()
	h.Commit("test commit")
	tag, _ := h.GenerateNextTag()

	if tag != "v0.0.1" {
		t.Errorf("Expected v0.0.1, got %s", tag)
	}

	h.CreateTag(tag)

	exists, _ := h.tagExists(tag)
	if !exists {
		t.Error("Tag should exist")
	}
}
