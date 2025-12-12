package gitgo

import (
	"fmt"
	"strconv"
	"strings"
)

// GitHandler handles Git related operations
type GitHandler struct{}

// NewGitHandler creates a new GitHandler
func NewGitHandler() *GitHandler {
	return &GitHandler{}
}

// Add stages all changes
func (h *GitHandler) Add() error {
	_, err := runCommand("git", "add", ".")
	return err
}

// Commit creates a commit if there are changes
func (h *GitHandler) Commit(message string) error {
	hasChanges, err := h.HasChanges()
	if err != nil {
		return err // Caller decides if this is fatal
	}

	if !hasChanges {
		log("No changes to commit")
		return nil
	}

	_, err = runCommand("git", "commit", "-m", message)
	return err
}

// Push executes the push workflow with tags
func (h *GitHandler) Push(tag string) error {
	branch, err := h.currentBranch()
	if err != nil {
		return err
	}

	hasUpstream, err := h.hasUpstream()
	if err != nil {
		return err
	}

	if !hasUpstream {
		if err := h.setUpstream(branch); err != nil {
			return err
		}
	} else {
		if err := h.push(); err != nil {
			return err
		}
	}

	if tag != "" {
		if err := h.pushTag(tag); err != nil {
			return err
		}
	}

	return nil
}

// CreateTag creates a local tag
func (h *GitHandler) CreateTag(tag string) error {
	exists, err := h.tagExists(tag)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tag %s already exists", tag)
	}

	_, err = runCommand("git", "tag", tag)
	if err != nil {
		return err
	}
	log("Created tag:", tag)
	return nil
}

// GenerateNextTag calculates the next patch version
func (h *GitHandler) GenerateNextTag() (string, error) {
	latest, err := h.latestTag()
	if err != nil {
		return "", err
	}
	if latest == "" {
		return "v0.0.1", nil
	}

	parts := strings.Split(latest, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid tag format: %s", latest)
	}

	lastNumStr := parts[len(parts)-1]
	lastNum, err := strconv.Atoi(lastNumStr)
	if err != nil {
		return "", fmt.Errorf("invalid tag number: %s", lastNumStr)
	}

	parts[len(parts)-1] = strconv.Itoa(lastNum + 1)
	return strings.Join(parts, "."), nil
}

// HasChanges checks if there are staged or unstaged changes
func (h *GitHandler) HasChanges() (bool, error) {
	_, err := runCommandSilent("git", "rev-parse", "HEAD")
	if err != nil {
		// No HEAD, check if any files exist
		out, err := runCommandSilent("git", "status", "--porcelain")
		if err != nil {
			return false, err
		}
		return len(out) > 0, nil
	}

	_, err = runCommandSilent("git", "diff-index", "--quiet", "HEAD", "--")
	return err != nil, nil
}

// Internal private helpers

func (h *GitHandler) latestTag() (string, error) {
	tag, err := runCommandSilent("git", "describe", "--abbrev=0", "--tags")
	if err != nil {
		return "", nil // No tags
	}
	return tag, nil
}

func (h *GitHandler) tagExists(tag string) (bool, error) {
	_, err := runCommandSilent("git", "rev-parse", tag)
	return err == nil, nil
}

func (h *GitHandler) currentBranch() (string, error) {
	return runCommandSilent("git", "symbolic-ref", "--short", "HEAD")
}

func (h *GitHandler) hasUpstream() (bool, error) {
	_, err := runCommandSilent("git", "rev-parse", "--symbolic-full-name", "--abbrev-ref", "@{u}")
	return err == nil, nil
}

func (h *GitHandler) setUpstream(branch string) error {
	_, err := runCommand("git", "push", "--set-upstream", "origin", branch)
	return err
}

func (h *GitHandler) push() error {
	_, err := runCommand("git", "push")
	return err
}

func (h *GitHandler) pushTag(tag string) error {
	_, err := runCommand("git", "push", "origin", tag)
	return err
}
