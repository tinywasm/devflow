package devflow

import (
	"encoding/json"
	"fmt"
	"github.com/tinywasm/command"
)

// SecretRunner abstracts command execution for testability.
// Exported to be implemented in external tests.
type SecretRunner interface {
	Run(name string, args ...string) (string, error)
	RunSilent(name string, args ...string) (string, error)
	RunWithStdin(input, name string, args ...string) (string, error)
}

// defaultRunner uses the package's global functions.
type defaultRunner struct{}

func (dr defaultRunner) Run(name string, args ...string) (string, error) {
	return command.Run(name, args...)
}
func (dr defaultRunner) RunSilent(name string, args ...string) (string, error) {
	return command.Run(name, args...)
}
func (dr defaultRunner) RunWithStdin(input, name string, args ...string) (string, error) {
	return command.RunWithStdin(input, name, args...)
}

func (gh *GitHub) getSecretRunner() SecretRunner {
	if gh.SecretRunner != nil {
		return gh.SecretRunner
	}
	return defaultRunner{}
}

// SetSecret registers a secret in GitHub Actions via gh CLI.
// The value is passed via stdin — gh CLI encrypts it with the repo's public key
// before transmitting it. It does not appear in system ps/logs.
func (gh *GitHub) SetSecret(repo, name, value string) error {
	runner := gh.getSecretRunner()
	// Use --body-file - to read from stdin
	_, err := runner.RunWithStdin(value, "gh", "secret", "set", name, "--body-file", "-", "--repo", repo)
	if err != nil {
		return fmt.Errorf("failed to set secret %s for repo %s: %w", name, repo, err)
	}
	return nil
}

// ListSecrets returns the names of secrets registered in the repo.
// Values are never accessible via API — GitHub only exposes names.
// gh secret list --repo=OWNER/REPO --json name --jq '[.[].name]'
func (gh *GitHub) ListSecrets(repo string) ([]string, error) {
	runner := gh.getSecretRunner()
	output, err := runner.RunSilent("gh", "secret", "list", "--repo", repo, "--json", "name", "--jq", "[.[].name]")
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets for repo %s: %w", repo, err)
	}
	return parseJSONStringArray(output)
}

// DeleteSecret removes a GitHub Actions secret.
// gh secret delete NAME --repo=OWNER/REPO
func (gh *GitHub) DeleteSecret(repo, name string) error {
	runner := gh.getSecretRunner()
	_, err := runner.Run("gh", "secret", "delete", name, "--repo", repo)
	if err != nil {
		return fmt.Errorf("failed to delete secret %s from repo %s: %w", name, repo, err)
	}
	return nil
}

// parseJSONStringArray parses the output of jq '[.[].name]' → []string.
func parseJSONStringArray(s string) ([]string, error) {
	var names []string
	if err := json.Unmarshal([]byte(s), &names); err != nil {
		return nil, fmt.Errorf("failed to parse secrets list JSON: %w", err)
	}
	if names == nil {
		return []string{}, nil
	}
	return names, nil
}
