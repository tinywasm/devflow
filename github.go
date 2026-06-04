package devflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GitHub handler for GitHub operations
type GitHub struct {
	log          func(...any)
	SecretRunner SecretRunner
}

// NewGitHub creates handler and verifies gh CLI availability.
// logFn is used to display authentication messages during Device Flow.
// If not authenticated, it initiates OAuth Device Flow automatically.
func NewGitHub(logFn func(...any), auth ...GitHubAuthenticator) (*GitHub, error) {
	if logFn == nil {
		logFn = func(...any) {}
	}
	gh := &GitHub{
		log: logFn,
	}

	// Verify gh installation
	if _, err := RunCommandSilent("gh", "--version"); err != nil {
		return nil, fmt.Errorf("gh cli is not installed or not in PATH: %w", err)
	}

	// Ensure authentication - this will initiate Device Flow if needed
	var authenticator GitHubAuthenticator
	if len(auth) > 0 && auth[0] != nil {
		// Use injected authenticator (already has TUI logger set)
		authenticator = auth[0]
	} else {
		// Create default authenticator and set logger
		authenticator = NewGitHubAuth()
		authenticator.SetLog(gh.log)
	}
	if err := authenticator.EnsureGitHubAuth(); err != nil {
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

// CreateRelease creates a GitHub Release and uploads assets.
// If targetRepo is not empty, it uses the --repo flag to publish to that repository.
func (gh *GitHub) CreateRelease(tag string, assets []string, targetRepo string) (string, error) {
	runner := gh.getSecretRunner()
	args := []string{"release", "create", tag, "--title", tag, "--notes", ""}
	if targetRepo != "" {
		args = append(args, "--repo", targetRepo)
	}
	args = append(args, assets...)

	output, err := runner.Run("gh", args...)
	if err != nil {
		return "", fmt.Errorf("failed to create release %s: %w", tag, err)
	}

	// Output is usually the URL of the created release
	return strings.TrimSpace(output), nil
}

// GetCurrentUser gets the current authenticated user
func (gh *GitHub) GetCurrentUser() (string, error) {
	output, err := RunCommandSilent("gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// repoInfo returns basic information about a repository.
// If repoRef is empty, it queries the repository in the current directory.
func (gh *GitHub) repoInfo(repoRef string) (owner, name, visibility string, err error) {
	runner := gh.getSecretRunner()
	args := []string{"repo", "view", "--json", "owner,name,visibility"}
	if repoRef != "" {
		args = []string{"repo", "view", repoRef, "--json", "owner,name,visibility"}
	}

	output, err := runner.RunSilent("gh", args...)
	if err != nil {
		return "", "", "", err
	}

	var data struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}

	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return "", "", "", fmt.Errorf("failed to parse repo info JSON: %w", err)
	}

	return data.Owner.Login, data.Name, data.Visibility, nil
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

// DeleteRepo deletes a repository on GitHub.
// WARNING: This permanently deletes the repository and cannot be undone.
// Use with caution, primarily for test cleanup.
func (gh *GitHub) DeleteRepo(owner, name string) error {
	repoName := fmt.Sprintf("%s/%s", owner, name)
	// --yes confirms deletion without prompting
	_, err := RunCommand("gh", "repo", "delete", repoName, "--yes")
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
