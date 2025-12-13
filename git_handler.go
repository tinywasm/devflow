package gitgo

import (
	"fmt"
	"strconv"
	"strings"
)

// Git handler for Git operations
type Git struct {
	// We can add configuration fields here if needed
}

// NewGit creates a new Git handler
func NewGit() *Git {
	return &Git{}
}

// Push executes the complete push workflow (add, commit, tag, push)
// Returns a summary of operations and error if any.
func (g *Git) Push(message, tag string) (string, error) {
	// Validate message
	if message == "" {
		message = "auto update package"
	}

	summary := []string{}

	// 1. Git add
	if err := g.add(); err != nil {
		return "", fmt.Errorf("git add failed: %w", err)
	}
	summary = append(summary, "Files staged")

	// 2. Commit (only if there are changes)
	committed, err := g.commit(message)
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}
	if committed {
		summary = append(summary, fmt.Sprintf("Committed: %s", message))
	} else {
		summary = append(summary, "No changes to commit")
	}

	// 3. Determine tag (provided or generated)
	finalTag := tag
	if finalTag == "" {
		generatedTag, err := g.GenerateNextTag()
		if err != nil {
			return "", fmt.Errorf("failed to generate tag: %w", err)
		}
		finalTag = generatedTag
	}

	// 4. Create tag
	created, err := g.createTag(finalTag)
	if err != nil {
		// If it already exists, not fatal error (can be re-run) but we should report it
		// Or maybe we should fail if explicit tag is provided but exists?
		// Existing logic: log warning.
		// We will note it in summary.
		summary = append(summary, fmt.Sprintf("Tag warning: %v", err))
	} else if created {
		summary = append(summary, fmt.Sprintf("Tag created: %s", finalTag))
	} else {
		summary = append(summary, fmt.Sprintf("Tag exists: %s", finalTag))
	}

	// 5. Push commits and tag
	if err := g.pushWithTags(finalTag); err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}
	summary = append(summary, "Pushed to remote")

	return strings.Join(summary, ", "), nil
}

// add adds all changes to staging
func (g *Git) add() error {
	_, err := RunCommand("git", "add", ".")
	return err
}

// hasChanges checks if there are staged changes
func (g *Git) hasChanges() (bool, error) {
	// Check if HEAD exists
	_, err := RunCommandSilent("git", "rev-parse", "HEAD")
	if err != nil {
		// No HEAD (fresh repo). Check if there are any files staged for initial commit.
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
		return true, nil
	}

	return false, nil
}

// commit creates a commit with the given message
// Returns true if a commit was created
func (g *Git) commit(message string) (bool, error) {
	hasChanges, err := g.hasChanges()
	if err != nil {
		return false, err
	}

	if !hasChanges {
		return false, nil
	}

	_, err = RunCommand("git", "commit", "-m", message)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetLatestTag gets the latest tag
func (g *Git) GetLatestTag() (string, error) {
	tag, err := RunCommandSilent("git", "describe", "--abbrev=0", "--tags")
	if err != nil {
		return "", nil
	}
	return tag, nil
}

// createTag creates a new tag
func (g *Git) createTag(tag string) (bool, error) {
	exists, err := g.tagExists(tag)
	if err != nil {
		return false, err
	}

	if exists {
		return false, fmt.Errorf("tag %s already exists", tag)
	}

	_, err = RunCommand("git", "tag", tag)
	return true, err
}

// GenerateNextTag calculates the next semantic version
func (g *Git) GenerateNextTag() (string, error) {
	latestTag, err := g.GetLatestTag()
	if err != nil {
		return "", err
	}

	if latestTag == "" {
		return "v0.0.1", nil
	}

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

	return newTag, nil
}

// tagExists checks if a tag exists
func (g *Git) tagExists(tag string) (bool, error) {
	_, err := RunCommandSilent("git", "rev-parse", tag)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// getCurrentBranch gets the current branch
func (g *Git) getCurrentBranch() (string, error) {
	output, err := RunCommandSilent("git", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return output, nil
}

// hasUpstream checks if the branch has upstream
func (g *Git) hasUpstream() (bool, error) {
	_, err := RunCommandSilent("git", "rev-parse", "--symbolic-full-name", "--abbrev-ref", "@{u}")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// setUpstream configures upstream
func (g *Git) setUpstream(branch string) error {
	_, err := RunCommand("git", "push", "--set-upstream", "origin", branch)
	if err != nil {
		return fmt.Errorf("failed to set upstream: %w", err)
	}
	return nil
}

// pushTag pushes a specific tag
func (g *Git) pushTag(tag string) error {
	_, err := RunCommand("git", "push", "origin", tag)
	if err != nil {
		return fmt.Errorf("failed to push tag %s: %w", tag, err)
	}
	return nil
}

// pushWithTags pushes commits and tag
func (g *Git) pushWithTags(tag string) error {
	branch, err := g.getCurrentBranch()
	if err != nil {
		return err
	}

	hasUpstream, err := g.hasUpstream()
	if err != nil {
		return err
	}

	if !hasUpstream {
		if err := g.setUpstream(branch); err != nil {
			return err
		}
	} else {
		// Normal push
		_, err := RunCommand("git", "push")
		if err != nil {
			return fmt.Errorf("git push failed: %w", err)
		}
	}

	if err := g.pushTag(tag); err != nil {
		return err
	}

	return nil
}
