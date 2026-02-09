package devflow

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Git handler for Git operations
type Git struct {
	rootDir     string
	shouldWrite func() bool
	log         func(...any)
}

// NewGit creates a new Git handler and verifies git is available
func NewGit() (*Git, error) {
	// Verify git installation
	if _, err := RunCommandSilent("git", "--version"); err != nil {
		return nil, fmt.Errorf("git is not installed or not in PATH: %w", err)
	}

	return &Git{
		rootDir:     ".",
		shouldWrite: func() bool { return false },
		log:         func(...any) {}, // default no-op
	}, nil
}

// SetRootDir sets the root directory for git operations
func (g *Git) SetRootDir(path string) {
	g.rootDir = path
}

// SetShouldWrite sets a function that determines if Git write operations
// (like updating .gitignore) should be allowed.
func (g *Git) SetShouldWrite(f func() bool) {
	g.shouldWrite = f
}

// SetLog sets the logger function
func (g *Git) SetLog(fn func(...any)) {
	if fn != nil {
		g.log = fn
	}
}

// CheckRemoteAccess verifies connectivity to the remote repository
func (g *Git) CheckRemoteAccess() error {
	// git ls-remote origin checks access without needing upstream configured
	_, err := RunCommandSilent("git", "ls-remote", "origin")
	if err != nil {
		// Try to provide a helpful error message
		if strings.Contains(err.Error(), "Authentication failed") || strings.Contains(err.Error(), "Could not read from remote repository") {
			return fmt.Errorf("❌ Authentication failed. Please check your git credentials or use 'git push' manually to authenticate")
		}
		if strings.Contains(err.Error(), "Could not resolve host") {
			return fmt.Errorf("❌ Network error. Please check your internet connection")
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("❌ Remote 'origin' not found. Please add a remote using 'git remote add origin <url>'")
		}
		return fmt.Errorf("❌ checking remote access failed: %w", err)
	}
	return nil
}

// PushResult contains the results of a Git push operation
type PushResult struct {
	Summary string // Human-readable summary of operations performed
	Tag     string // The tag that was created and pushed
}

// Push executes the complete push workflow (add, commit, tag, push)
// Returns a PushResult and error if any.
func (g *Git) Push(message, tag string) (PushResult, error) {
	// Validate message
	if err := ValidateCommitMessage(message); err != nil {
		return PushResult{}, err
	}
	message = FormatCommitMessage(message)

	summary := []string{}

	// 0. Verify remote access before doing anything destructive
	if err := g.CheckRemoteAccess(); err != nil {
		return PushResult{}, err
	}

	// 1. Git add
	if err := g.Add(); err != nil {
		return PushResult{}, fmt.Errorf("git add failed: %w", err)
	}

	// 2. Determine tag (provided or generated)
	finalTag := tag
	if finalTag == "" {
		generatedTag, err := g.GenerateNextTag()
		if err != nil {
			return PushResult{}, fmt.Errorf("failed to generate tag: %w", err)
		}
		finalTag = generatedTag
	}

	// 3. Validate tag is greater than latest
	latestTag, err := g.GetLatestTag()
	if err == nil && latestTag != "" {
		if CompareVersions(finalTag, latestTag) <= 0 {
			return PushResult{}, fmt.Errorf("tag %s is not greater than latest tag %s", finalTag, latestTag)
		}
	}

	// 4. Commit (only if there are changes)
	committed, err := g.Commit(message)
	if err != nil {
		return PushResult{}, fmt.Errorf("git commit failed: %w", err)
	}

	// If no changes were committed, check if we're ahead of remote
	// If so, we can still push existing commits without creating a tag
	if !committed {
		// Check if there are unpushed commits
		ahead, err := g.IsAheadOfRemote()
		if err != nil {
			// Can't determine status, just return success without doing anything
			return PushResult{
				Summary: "No changes to commit",
				Tag:     "",
			}, nil
		}

		if ahead {
			// There are unpushed commits, push them without creating a new tag
			if err := g.PushWithoutTags(); err != nil {
				return PushResult{}, fmt.Errorf("push failed: %w", err)
			}
			return PushResult{
				Summary: "✅ Pushed existing commits",
				Tag:     "",
			}, nil
		}

		// No changes and no unpushed commits
		return PushResult{
			Summary: "No changes to commit",
			Tag:     "",
		}, nil
	}

	// 5. Create tag - if exists, keep incrementing until we find available one
	maxAttempts := 100 // Prevent infinite loop
	attempt := 0
	for attempt < maxAttempts {
		created, err := g.CreateTag(finalTag)
		if err == nil && created {
			// Success
			summary = append(summary, fmt.Sprintf("✅ Tag: %s", finalTag))
			break
		}

		// Tag exists, increment from current finalTag
		g.log("Tag", finalTag, "already exists, trying next")
		nextTag, err := g.IncrementTag(finalTag)
		if err != nil {
			return PushResult{}, fmt.Errorf("failed to increment tag: %w", err)
		}
		finalTag = nextTag
		attempt++
	}

	if attempt >= maxAttempts {
		return PushResult{}, fmt.Errorf("could not find available tag after %d attempts", maxAttempts)
	}

	// 5. Push commits and tag
	if err := g.PushWithTags(finalTag); err != nil {
		return PushResult{}, fmt.Errorf("push failed: %w", err)
	}
	summary = append(summary, "✅ Pushed ok")

	return PushResult{
		Summary: strings.Join(summary, ", "),
		Tag:     finalTag,
	}, nil
}

// Add adds all changes to staging
func (g *Git) Add() error {
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

// Commit creates a commit with the given message
// Returns true if a commit was created
func (g *Git) Commit(message string) (bool, error) {
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

// CreateTag creates a new tag
func (g *Git) CreateTag(tag string) (bool, error) {
	exists, err := g.TagExists(tag)
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

// IncrementTag increments a specific tag (e.g., v0.0.12 -> v0.0.13)
func (g *Git) IncrementTag(tag string) (string, error) {
	if tag == "" {
		return "v0.0.1", nil
	}

	parts := strings.Split(tag, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid tag format: %s", tag)
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

// TagExists checks if a tag exists
func (g *Git) TagExists(tag string) (bool, error) {
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

// PushWithTags pushes commits and tag
func (g *Git) PushWithTags(tag string) error {
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

// GetConfigUserName gets the git user.name
func (g *Git) GetConfigUserName() (string, error) {
	name, err := RunCommandSilent("git", "config", "user.name")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

// GetConfigUserEmail gets the git user.email
func (g *Git) GetConfigUserEmail() (string, error) {
	email, err := RunCommandSilent("git", "config", "user.email")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(email), nil
}

// IsAheadOfRemote checks if local branch is ahead of remote
func (g *Git) IsAheadOfRemote() (bool, error) {
	// Get current branch
	branch, err := RunCommandSilent("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return false, err
	}

	// Check if ahead of origin
	out, err := RunCommandSilent("git", "rev-list", "--count", fmt.Sprintf("origin/%s..HEAD", branch))
	if err != nil {
		// If origin/<branch> doesn't exist, we're not ahead
		return false, nil
	}

	count := strings.TrimSpace(out)
	if count == "0" || count == "" {
		return false, nil
	}

	return true, nil
}

// PushWithoutTags pushes commits without pushing tags
func (g *Git) PushWithoutTags() error {
	_, err := RunCommand("git", "push")
	return err
}

// SetUserConfig sets git user name and email
func (g *Git) SetUserConfig(name, email string) error {
	if _, err := RunCommand("git", "config", "user.name", name); err != nil {
		return err
	}
	if _, err := RunCommand("git", "config", "user.email", email); err != nil {
		return err
	}
	return nil
}

// InitRepo initializes a new git repository
func (g *Git) InitRepo(dir string) error {
	if _, err := RunCommand("git", "init", dir); err != nil {
		return err
	}

	// Set main branch
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(dir); err != nil {
		return err
	}

	if _, err := RunCommand("git", "branch", "-M", "main"); err != nil {
		// On fresh init with no commits, this might fail, but git init usually sets up a default branch.
		// Newer git versions use init.defaultBranch.
		// If it fails, it might mean there are no commits yet so HEAD doesn't point anywhere meaningful.
		// We can ignore or handle.
		// Actually "git branch -M main" works even with no commits in recent git.
		// Let's assume it works or is not critical if we are on a version that defaults to master.
		return err
	}
	return nil
}
