package devflow

import (
	"fmt"
	"strings"
)

// GitHub handler for GitHub operations
type GitHub struct {
	log func(...any)
}

// NewGitHub creates handler and verifies gh CLI availability
// If not authenticated, it initiates OAuth Device Flow automatically
func NewGitHub() (*GitHub, error) {
	gh := &GitHub{
		log: func(...any) {},
	}

	// Verify gh installation
	if _, err := RunCommandSilent("gh", "--version"); err != nil {
		return nil, fmt.Errorf("gh cli is not installed or not in PATH: %w", err)
	}

	// Ensure authentication - this will initiate Device Flow if needed
	auth := NewGitHubAuth()
	auth.SetLog(gh.log)
	if err := auth.EnsureGitHubAuth(); err != nil {
		return nil, fmt.Errorf("github authentication failed: %w", err)
	}

	return gh, nil
}

// SetLog sets the logger function
func (gh *GitHub) SetLog(fn func(...any)) {
	if fn != nil {
		gh.log = fn
	}
}

// GetCurrentUser gets the current authenticated user
func (gh *GitHub) GetCurrentUser() (string, error) {
	output, err := RunCommandSilent("gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// RepoExists checks if a repository exists
func (gh *GitHub) RepoExists(owner, name string) (bool, error) {
	// gh repo view owner/name
	_, err := RunCommandSilent("gh", "repo", "view", fmt.Sprintf("%s/%s", owner, name))
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CreateRepo creates a new empty repository on GitHub
// If owner is provided, creates repo under that organization
func (gh *GitHub) CreateRepo(owner, name, description, visibility string) error {
	repoName := name
	if owner != "" {
		repoName = fmt.Sprintf("%s/%s", owner, name)
	}
	// Create empty repo without --source or --push (will add remote and push manually)
	args := []string{"repo", "create", repoName, "--description", description}

	if visibility == "private" {
		args = append(args, "--private")
	} else {
		args = append(args, "--public")
	}

	_, err := RunCommand("gh", args...)
	return err
}

// IsNetworkError checks if an error is likely a network error
func (gh *GitHub) IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "timeout")
}

// GetHelpfulErrorMessage returns a helpful message for common errors
func (gh *GitHub) GetHelpfulErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if gh.IsNetworkError(err) {
		return "Network error. Check your internet connection."
	}
	if strings.Contains(err.Error(), "authentication") {
		return "Authentication failed. Run 'gh auth login'."
	}
	return err.Error()
}
