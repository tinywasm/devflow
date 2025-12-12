# PROMPT 02: Git Operations

## Context
Implement all Git operations needed for the `push` and `gopu` commands in the `git_operations.go` file. Uses `RunCommand` from `executor.go` to minimize code.

## Reference Bash Scripts

### pu.sh - Git Operations
```bash
# Add
git add .

# Check changes
git diff-index --quiet HEAD --

# Commit
git commit -m "$message"

# Tags
latest_tag=$(git describe --abbrev=0 --tags 2>/dev/null)
git tag $new_tag
git push origin $new_tag

# Upstream
branch=$(git symbolic-ref --short HEAD)
upstream=$(git rev-parse --symbolic-full-name --abbrev-ref @{u} 2>/dev/null)
git push --set-upstream origin $branch
```

## File: git_operations.go

```go
package gitgo

import (
	"fmt"
	"strconv"
	"strings"
)

// GitAdd adds all changes to staging
func GitAdd() error {
	_, err := RunCommand("git", "add", ".")
	return err
}

// GitHasChanges checks if there are staged changes
func GitHasChanges() (bool, error) {
	// Use Silent to avoid spamming logs for checks
	_, err := RunCommandSilent("git", "diff-index", "--quiet", "HEAD", "--")
	
	if err != nil {
		// If command fails (exit code 1), it means there are changes
		// We assume any error means changes or git failure, but for diff-index exit 1 is "changes"
		return true, nil
	}
	
	return false, nil
}

// GitCommit creates a commit with the given message
func GitCommit(message string) error {
	hasChanges, err := GitHasChanges()
	if err != nil {
		return err
	}
	
	if !hasChanges {
		log("No changes to commit")
		return nil
	}
	
	_, err = RunCommand("git", "commit", "-m", message)
	return err
}

// GitGetLatestTag gets the latest tag
func GitGetLatestTag() (string, error) {
	// 2>/dev/null in bash means we ignore stderr, RunCommandSilent captures it but we can ignore error if output is empty
	tag, err := RunCommandSilent("git", "describe", "--abbrev=0", "--tags")
	if err != nil {
		// If no tags exist, git describe fails. We return empty string and no error to handle "v0.0.1" logic
		return "", nil
	}
	return tag, nil
}

// GitTag creates a new tag
func GitTag(tag string) error {
	_, err := RunCommand("git", "tag", tag)
	return err
}

// GitPush pushes changes and tags
func GitPush(tag string) error {
	// Check upstream
	branch, err := RunCommandSilent("git", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if upstream exists
	_, err = RunCommandSilent("git", "rev-parse", "--symbolic-full-name", "--abbrev-ref", "@{u}")
	hasUpstream := err == nil

	if !hasUpstream {
		log("Setting upstream for branch", branch)
		if _, err := RunCommand("git", "push", "--set-upstream", "origin", branch); err != nil {
			return err
		}
	} else {
		if _, err := RunCommand("git", "push"); err != nil {
			return err
		}
	}

	// Push tag
	if tag != "" {
		if _, err := RunCommand("git", "push", "origin", tag); err != nil {
			return err
		}
	}

	return nil
}

// GitGenerateNextTag calculates the next semantic version
func GitGenerateNextTag() (string, error) {
	latestTag, err := GitGetLatestTag()
	if err != nil {
		return "", err
	}

	if latestTag == "" {
		return "v0.0.1", nil
	}

	// Simple semantic versioning bump (patch level)
	// Assumes vX.Y.Z format
	parts := strings.Split(latestTag, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid tag format: %s", latestTag)
	}

	lastNumStr := parts[len(parts)-1]
	lastNum, err := strconv.Atoi(lastNumStr)
	if err != nil {
		return "", fmt.Errorf("invalid tag number: %s", lastNumStr)
	}

	parts[len(parts)-1] = strconv.Itoa(lastNum + 1)
	newTag := strings.Join(parts, ".")

	// Check if exists (simple check, loop logic from bash omitted for simplicity but can be added)
	// In a real scenario, we might want to check if it exists locally
	
	return newTag, nil
}
```
    
    if err != nil {
        // No tags
        return "", nil
    }
    
    return strings.TrimSpace(string(output)), nil
}

// GitGenerateNextTag generates the next tag based on the latest
// Logic from pu.sh for incrementing tags
func GitGenerateNextTag() (string, error) {
    latestTag, err := GitGetLatestTag()
    
    if err != nil || latestTag == "" {
        // No tags, start with v0.0.1
        return "v0.0.1", nil
    }
    
    // Extract number at the end (v0.0.5 -> 5)
    re := regexp.MustCompile(`(\d+)$`)
    matches := re.FindStringSubmatch(latestTag)
    
    if len(matches) < 2 {
        return "", fmt.Errorf("invalid tag format: %s", latestTag)
    }
    
    lastNumber, err := strconv.Atoi(matches[1])
    if err != nil {
        return "", fmt.Errorf("failed to parse tag number: %w", err)
    }
    
    // Increment
    nextNumber := lastNumber + 1
    newTag := re.ReplaceAllString(latestTag, strconv.Itoa(nextNumber))
    
    // Verify it doesn't exist
    for {
        if exists, _ := GitTagExists(newTag); !exists {
            break
        }
        nextNumber++
        newTag = re.ReplaceAllString(latestTag, strconv.Itoa(nextNumber))
        log("tag already exists, trying:", newTag)
    }
    
    return newTag, nil
}

// GitTagExists checks if a tag exists
// Equivalent to: git rev-parse tag
func GitTagExists(tag string) (bool, error) {
    cmd := exec.Command("git", "rev-parse", tag)
    err := cmd.Run()
    
    if err != nil {
        return false, nil
    }
    
    return true, nil
}

// GitCreateTag creates a tag
// Equivalent to: git tag <tag>
func GitCreateTag(tag string) error {
    exists, err := GitTagExists(tag)
    if err != nil {
        return err
    }
    
    if exists {
        return fmt.Errorf("tag %s already exists", tag)
    }
    
    cmd := exec.Command("git", "tag", tag)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("git tag failed: %w", err)
    }
    
    log("new tag", tag)
    return nil
}

// GitGetCurrentBranch gets the current branch
// Equivalent to: git symbolic-ref --short HEAD
func GitGetCurrentBranch() (string, error) {
    cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
    output, err := cmd.Output()
    
    if err != nil {
        return "", fmt.Errorf("failed to get current branch: %w", err)
    }
    
    return strings.TrimSpace(string(output)), nil
}

// GitHasUpstream checks if the branch has upstream
// Equivalent to: git rev-parse --symbolic-full-name --abbrev-ref @{u}
func GitHasUpstream() (bool, error) {
    cmd := exec.Command("git", "rev-parse", "--symbolic-full-name", "--abbrev-ref", "@{u}")
    err := cmd.Run()
    
    if err != nil {
        return false, nil
    }
    
    return true, nil
}

// GitSetUpstream configures upstream
// Equivalent to: git push --set-upstream origin <branch>
func GitSetUpstream(branch string) error {
    cmd := exec.Command("git", "push", "--set-upstream", "origin", branch)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to set upstream: %w", err)
    }
    
    return nil
}

// GitPush pushes to remote
// Equivalent to: git push
func GitPush() error {
    cmd := exec.Command("git", "push")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("git push failed: %w", err)
    }
    
    return nil
}

// GitPushTag pushes a specific tag
// Equivalent to: git push origin <tag>
func GitPushTag(tag string) error {
    cmd := exec.Command("git", "push", "origin", tag)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to push tag %s: %w", tag, err)
    }
    
    return nil
}

// GitPushWithTags pushes commits and tag (pu.sh logic)
func GitPushWithTags(tag string) error {
    branch, err := GitGetCurrentBranch()
    if err != nil {
        return err
    }
    
    hasUpstream, err := GitHasUpstream()
    if err != nil {
        return err
    }
    
    if !hasUpstream {
        // Configure upstream and push
        if err := GitSetUpstream(branch); err != nil {
            return err
        }
    } else {
        // Normal push
        if err := GitPush(); err != nil {
            return err
        }
    }
    
    // Push the tag
    if err := GitPushTag(tag); err != nil {
        return err
    }
    
    log("Commit and Push completed")
    return nil
}
```

## File: git_operations_test.go

```go
package gitgo

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

// Helper: create temporary repo
func createTestRepo(t *testing.T) string {
    t.Helper()
    
    dir := t.TempDir()
    
    // Initialize git
    cmd := exec.Command("git", "init")
    cmd.Dir = dir
    if err := cmd.Run(); err != nil {
        t.Fatal(err)
    }
    
    // Configure user
    exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
    exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
    
    return dir
}

func TestGitHasChanges(t *testing.T) {
    dir := createTestRepo(t)
    os.Chdir(dir)
    
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
    dir := createTestRepo(t)
    os.Chdir(dir)
    
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
    dir := createTestRepo(t)
    os.Chdir(dir)
    
    // Without changes should not fail
    err := GitCommit("test")
    if err != nil {
        t.Error("Commit without changes should not fail")
    }
    
    // With changes
    os.WriteFile("test.txt", []byte("test"), 0644)
    GitAdd()
    
    err = GitCommit("test commit")
    if err != nil {
        t.Fatal(err)
    }
}
```

## Exported Functions

All functions are exported (capital initial) so they can be used from other packages:

- `GitAdd()`
- `GitHasChanges()`
- `GitCommit(message)`
- `GitGetLatestTag()`
- `GitGenerateNextTag()`
- `GitTagExists(tag)`
- `GitCreateTag(tag)`
- `GitGetCurrentBranch()`
- `GitHasUpstream()`
- `GitSetUpstream(branch)`
- `GitPush()`
- `GitPushTag(tag)`
- `GitPushWithTags(tag)`

## Notes

- No colors in output
- Minimal output using `log()`
- Descriptive errors with `fmt.Errorf`
- 100% compatible with `pu.sh` logic
- Basic tests (~40% coverage)
