package gitgo

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGitHandler_HasChanges(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	h := NewGitHandler()

	// Create file
	os.WriteFile("test.txt", []byte("test"), 0644)

	// Add
	h.Add()

	// Should have changes
	hasChanges, err := h.HasChanges()
	if err != nil {
		t.Fatal(err)
	}

	if !hasChanges {
		t.Error("Expected changes but got none")
	}
}

func TestGitHandler_GenerateNextTag(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	h := NewGitHandler()

	// Initial commit
	os.WriteFile("test.txt", []byte("test"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()

	// Without tags should return v0.0.1
	tag, err := h.GenerateNextTag()
	if err != nil {
		t.Fatal(err)
	}

	if tag != "v0.0.1" {
		t.Errorf("Expected v0.0.1, got %s", tag)
	}

	// Create tag
	h.CreateTag("v0.0.1")

	// Next should be v0.0.2
	tag, err = h.GenerateNextTag()
	if err != nil {
		t.Fatal(err)
	}

	if tag != "v0.0.2" {
		t.Errorf("Expected v0.0.2, got %s", tag)
	}
}

func TestGitHandler_Commit(t *testing.T) {
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	h := NewGitHandler()

	// Without changes should not fail
	err := h.Commit("test")
	if err != nil {
		t.Error("Commit without changes should not fail")
	}

	// With changes
	os.WriteFile("test.txt", []byte("test changes"), 0644)
	h.Add()

	err = h.Commit("test commit")
	if err != nil {
		t.Fatalf("GitCommit failed: %v", err)
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
