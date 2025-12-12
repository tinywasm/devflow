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
    // Check if HEAD exists
    _, err := RunCommandSilent("git", "rev-parse", "HEAD")
    if err != nil {
        // No HEAD (fresh repo). Check if there are any files staged for initial commit.
        // We can use git ls-files to see if anything is staged.
        // Or simpler: git status --porcelain
        out, err := RunCommandSilent("git", "status", "--porcelain")
        if err != nil {
             return false, err
        }
        if len(out) > 0 {
            return true, nil
        }
        return false, nil
    }

	// Use Silent to avoid spamming logs for checks
	_, err = RunCommandSilent("git", "diff-index", "--quiet", "HEAD", "--")

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
        log("GitHasChanges error:", err)
		return err
	}

	if !hasChanges {
		log("No changes to commit")
		return nil
	}

	out, err := RunCommand("git", "commit", "-m", message)
    if err != nil {
        log("RunCommand commit error:", err)
        log("Output:", out)
    }
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
func GitPush() error {
	_, err := RunCommand("git", "push")
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
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

// GitTagExists checks if a tag exists
// Equivalent to: git rev-parse tag
func GitTagExists(tag string) (bool, error) {
	_, err := RunCommandSilent("git", "rev-parse", tag)

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

	_, err = RunCommand("git", "tag", tag)
	if err != nil {
		return fmt.Errorf("git tag failed: %w", err)
	}

	log("new tag", tag)
	return nil
}

// GitGetCurrentBranch gets the current branch
// Equivalent to: git symbolic-ref --short HEAD
func GitGetCurrentBranch() (string, error) {
	output, err := RunCommandSilent("git", "symbolic-ref", "--short", "HEAD")

	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return output, nil
}

// GitHasUpstream checks if the branch has upstream
// Equivalent to: git rev-parse --symbolic-full-name --abbrev-ref @{u}
func GitHasUpstream() (bool, error) {
	_, err := RunCommandSilent("git", "rev-parse", "--symbolic-full-name", "--abbrev-ref", "@{u}")

	if err != nil {
		return false, nil
	}

	return true, nil
}

// GitSetUpstream configures upstream
// Equivalent to: git push --set-upstream origin <branch>
func GitSetUpstream(branch string) error {
	_, err := RunCommand("git", "push", "--set-upstream", "origin", branch)
	if err != nil {
		return fmt.Errorf("failed to set upstream: %w", err)
	}

	return nil
}

// GitPushTag pushes a specific tag
// Equivalent to: git push origin <tag>
func GitPushTag(tag string) error {
	_, err := RunCommand("git", "push", "origin", tag)
	if err != nil {
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
