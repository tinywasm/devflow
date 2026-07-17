package devflow

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

const ghTokenKey = "GH_TOKEN"

// GitHubPATAuth manages the GitHub PAT via the system keyring.
// It is used to recover the gh CLI session non-interactively.
type GitHubPATAuth struct {
	kr  *Keyring
	log func(...any)
}

// NewGitHubPATAuth creates a GitHubPATAuth with an initialized keyring.
func NewGitHubPATAuth() (*GitHubPATAuth, error) {
	kr, _ := NewKeyring()
	return &GitHubPATAuth{
		kr:  kr,
		log: func(...any) {},
	}, nil
}

// SetLog sets the logging function.
func (a *GitHubPATAuth) SetLog(fn func(...any)) {
	if fn != nil {
		a.log = fn
	}
}

// HasToken returns true if the GitHub PAT is already stored in the environment or keyring.
func (a *GitHubPATAuth) HasToken() bool {
	if os.Getenv("GH_TOKEN") != "" {
		return true
	}
	if a.kr == nil {
		return false
	}
	tok, err := a.kr.Get(ghTokenKey)
	return err == nil && tok != ""
}

// EnsureToken returns the PAT from the environment or keyring; if absent, prompts once and persists.
func (a *GitHubPATAuth) EnsureToken() (string, error) {
	if envTok := os.Getenv("GH_TOKEN"); envTok != "" {
		return envTok, nil
	}

	if a.kr == nil {
		return "", fmt.Errorf("keyring is unavailable and GH_TOKEN env var is not set")
	}

	tok, err := a.kr.Get(ghTokenKey)
	if err == nil && tok != "" {
		return tok, nil
	}

	fmt.Fprintf(os.Stderr,
		"GitHub token not found. Create a fine-grained PAT (Contents + Pull requests: Read/Write) at %s\nEnter it now: ",
		termLink("https://github.com/settings/tokens", "https://github.com/settings/tokens"))

	tok, err = readSecret()
	if err != nil {
		return "", err
	}

	if tok == "" {
		return "", fmt.Errorf("no GitHub token provided")
	}

	if err := a.kr.Set(ghTokenKey, tok); err != nil {
		a.log(fmt.Sprintf("warning: could not save GitHub token to keyring: %v", err))
	}

	return tok, nil
}

// Reset removes the GitHub PAT from the keyring.
func (a *GitHubPATAuth) Reset() error {
	if a.kr == nil {
		return fmt.Errorf("keyring is unavailable")
	}
	return a.kr.Delete(ghTokenKey)
}

// EnsureGitHubAuth fulfills the GitHubAuthenticator interface.
func (a *GitHubPATAuth) EnsureGitHubAuth() error {
	return EnsureGHSession(RealRunner{})
}

// readSecret reads a secret from stdin without echoing.
func readSecret() (string, error) {
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("could not read secret: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}
