package devflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tinywasm/command"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DevflowOAuthClientID is the OAuth App Client ID for devflow.
//
// IMPORTANT: This Client ID is intentionally hardcoded and is NOT a secret.
// OAuth Client IDs are public identifiers (like a username, not a password).
// The Client Secret is NEVER included in the code - Device Flow doesn't need it.
// This is the standard approach used by CLI tools like gh, goreleaser, hub, etc.
//
// The OAuth App is registered under a personal GitHub account (not organization).
// Manage the app at: https://github.com/settings/developers -> OAuth Apps -> devflow
const DevflowOAuthClientID = "Ov23lijHU2vxBCpShn1Q"

// GitHub token key for keyring storage
// Deprecated: use SecretGitHubToken with SecretStore
const githubTokenKey = "github_token"

// GitHubOAuth handles GitHub authentication and token management via Device Flow
type GitHubOAuth struct {
	log   func(...any)
	store *SecretStore
}

// NewGitHubOAuth creates a new GitHub authentication handler
func NewGitHubOAuth() *GitHubOAuth {
	return &GitHubOAuth{
		log:   func(...any) {},
		store: NewSecretStore(),
	}
}

// Name returns the handler name for TUI display.
func (a *GitHubOAuth) Name() string {
	return "GitHub Auth"
}

// SetLog sets the logger function
func (a *GitHubOAuth) SetLog(fn func(...any)) {
	if fn != nil {
		a.log = fn
		a.store.SetLog(fn)
	}
}

// deviceCodeResponse represents the response from GitHub's device code endpoint
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// tokenResponse represents the response from GitHub's token endpoint
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// EnsureGitHubAuth checks if GitHub is authenticated via keyring, and if not, initiates Device Flow
func (a *GitHubOAuth) EnsureGitHubAuth() error {
	token, src, err := a.store.Get(SecretGitHubToken)
	if err == nil && token != "" {
		// Verify the token works by configuring gh
		if a.configureGhWithToken(token) == nil {
			if _, err := command.Run("gh", "auth", "status"); err == nil {
				return nil
			}
		}

		// Token is invalid
		if src == SourceEnv {
			return fmt.Errorf("invalid GitHub token in environment variable (GH_TOKEN or GITHUB_TOKEN)")
		}

		// If it was from keyring, delete it and try to re-authenticate
		a.store.Delete(SecretGitHubToken)
	}

	// Not authenticated - check if interactive
	if !IsInteractive() {
		return fmt.Errorf("no GitHub token found; set GH_TOKEN or GITHUB_TOKEN, or run locally to authenticate")
	}

	// Initiate Device Flow
	token, err = a.DeviceFlowAuth()
	if err != nil {
		return err
	}

	// Configure gh CLI with the new token
	return a.configureGhWithToken(token)
}

// DeviceFlowAuth initiates GitHub OAuth Device Flow and returns an access token
func (a *GitHubOAuth) DeviceFlowAuth() (string, error) {
	// Step 1: Request device and user codes
	codeResp, err := a.requestDeviceCode()
	if err != nil {
		return "", fmt.Errorf("failed to request device code: %w", err)
	}

	// Step 2: Open browser for user authorization
	// Use LogOpen prefix for animated progress in TUI
	a.log("[...", fmt.Sprintf("Paste this code in browser: %s", codeResp.UserCode))

	if err := a.openBrowser(codeResp.VerificationURI); err != nil {
		a.log(fmt.Sprintf("Could not open browser. Please go to: %s", codeResp.VerificationURI))
	}

	// Step 3: Poll for the access token
	interval := codeResp.Interval
	if interval < 5 {
		interval = 5
	}

	token, err := a.pollForToken(codeResp.DeviceCode, interval, codeResp.ExpiresIn)
	if err != nil {
		return "", err
	}

	// Step 4: Save token to keyring
	if err := a.store.Set(SecretGitHubToken, token); err != nil {
		a.log(fmt.Sprintf("Warning: could not save token: %v", err))
	}

	// Use LogClose prefix to stop animation and show success
	a.log("...]", "GitHub authentication successful!")
	return token, nil
}

// requestDeviceCode requests a device code from GitHub
func (a *GitHubOAuth) requestDeviceCode() (*deviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", DevflowOAuthClientID)
	data.Set("scope", "repo read:org delete_repo")

	req, err := http.NewRequest("POST", "https://github.com/login/device/code", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var codeResp deviceCodeResponse
	if err := json.Unmarshal(body, &codeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	if codeResp.DeviceCode == "" {
		return nil, fmt.Errorf("no device code in response: %s", string(body))
	}

	return &codeResp, nil
}

// pollForToken polls GitHub for the access token
func (a *GitHubOAuth) pollForToken(deviceCode string, interval, expiresIn int) (string, error) {
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)

		data := url.Values{}
		data.Set("client_id", DevflowOAuthClientID)
		data.Set("device_code", deviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var tokenResp tokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			continue
		}

		switch tokenResp.Error {
		case "":
			if tokenResp.AccessToken != "" {
				return tokenResp.AccessToken, nil
			}
		case "authorization_pending":
			// TUI handles animation, no need to log dots
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			return "", fmt.Errorf("authorization expired, please try again")
		case "access_denied":
			return "", fmt.Errorf("access denied by user")
		default:
			return "", fmt.Errorf("authorization failed: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
	}

	return "", fmt.Errorf("authorization timed out")
}

// openBrowser opens a URL in the default browser (cross-platform)
func (a *GitHubOAuth) openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// configureGhWithToken configures gh CLI to use the token
func (a *GitHubOAuth) configureGhWithToken(token string) error {
	cmd := exec.Command("gh", "auth", "login", "--with-token")
	cmd.Stdin = bytes.NewReader([]byte(token))
	return cmd.Run()
}
