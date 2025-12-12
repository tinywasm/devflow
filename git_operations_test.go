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

	// Create file
	os.WriteFile("test.txt", []byte("test"), 0644)

	// Add
	GitAdd()

	// Should have changes
	hasChanges, err := GitHasChanges()
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

	// Initial commit
	os.WriteFile("test.txt", []byte("test"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()

	// Without tags should return v0.0.1
	tag, err := GitGenerateNextTag()
	if err != nil {
		t.Fatal(err)
	}

	if tag != "v0.0.1" {
		t.Errorf("Expected v0.0.1, got %s", tag)
	}

	// Create tag
	GitCreateTag("v0.0.1")

	// Next should be v0.0.2
	tag, err = GitGenerateNextTag()
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

	// Without changes should not fail
	err := GitCommit("test")
	if err != nil {
		t.Error("Commit without changes should not fail")
	}

	// With changes
	os.WriteFile("test.txt", []byte("test changes"), 0644)
	GitAdd()

    // Check for changes
    has, _ := GitHasChanges()
    if !has {
        t.Fatal("Should have changes before commit")
    }

	// Wait a bit to ensure git timestamp update? No, that's usually for racy tests.
	// But let's check why it fails.

	err = GitCommit("test commit")
	if err != nil {
        t.Logf("Error content: %v", err)
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
