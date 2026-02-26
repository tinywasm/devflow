package devflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient defines the interface for HTTP operations (injectable for tests).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// JulesConfig holds the configuration for the Jules driver.
// All fields are optional: APIKey is loaded from keyring if empty,
// SourceID and StartBranch are auto-detected via gh/git if empty.
type JulesConfig struct {
	APIKey              string        // optional: loaded from keyring if empty
	SourceID            string        // optional: auto-detected via gh CLI if empty
	StartBranch         string        // optional: auto-detected via git if empty
	SessionTitle        string        // optional: defaults to prompt filename
	SourceIndexTimeout  time.Duration // optional: max wait for source to appear (default 2m)
	SourceIndexInterval time.Duration // optional: polling interval for source check (default 10s)
}

// JulesDriver implements CodeJobDriver for the Jules AI agent.
type JulesDriver struct {
	config    JulesConfig
	http      HTTPClient
	log       func(...any)
	sessionID string
}

// NewJulesDriver creates a JulesDriver. All JulesConfig fields are optional.
func NewJulesDriver(config JulesConfig) *JulesDriver {
	return &JulesDriver{
		config: config,
		http:   &http.Client{},
		log:    func(...any) {},
	}
}

// Name returns the driver name.
func (d *JulesDriver) Name() string { return "Jules" }

// SessionID returns the last session ID created.
func (d *JulesDriver) SessionID() string { return d.sessionID }

// SetLog sets the logging function.
func (d *JulesDriver) SetLog(fn func(...any)) {
	if fn != nil {
		d.log = fn
	}
}

// SetHTTPClient replaces the HTTP client (for testing).
func (d *JulesDriver) SetHTTPClient(client HTTPClient) {
	d.http = client
}

// julesSessionRequest is the Jules API POST body.
type julesSessionRequest struct {
	Title          string      `json:"title"`
	Prompt         string      `json:"prompt"`
	SourceContext  julesSource `json:"sourceContext"`
	AutomationMode string      `json:"automationMode"`
}

type julesSource struct {
	Source            string         `json:"source"`
	GithubRepoContext julesGithubCtx `json:"githubRepoContext"`
}

type julesGithubCtx struct {
	StartingBranch string `json:"startingBranch"`
}

// Send creates a Jules session using the prompt and title resolved by CodeJob.
// Jules accesses the referenced file directly from the repository via its GitHub App access.
// If the source is not yet indexed in Jules (404 on new repos), it polls GET /sources
// until the source appears or the timeout is exceeded.
func (d *JulesDriver) Send(prompt, title string) (string, error) {
	apiKey, err := d.resolveAPIKey()
	if err != nil {
		return "", err
	}

	sourceID, err := d.resolveSourceID()
	if err != nil {
		return "", err
	}

	branch, err := d.resolveBranch()
	if err != nil {
		return "", err
	}

	if d.config.SessionTitle != "" {
		title = d.config.SessionTitle // config override takes precedence
	}
	if title == "" {
		title = "CodeJob Task" // ultimate fallback
	}

	body := julesSessionRequest{
		Title:  title,
		Prompt: prompt,
		SourceContext: julesSource{
			Source: sourceID,
			GithubRepoContext: julesGithubCtx{
				StartingBranch: branch,
			},
		},
		AutomationMode: "AUTO_CREATE_PR",
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("could not encode request: %w", err)
	}

	// doPost sends the session creation request. Uses bytes.NewReader so
	// the same payload slice can be re-read on each retry attempt.
	doPost := func() (int, []byte, error) {
		req, err := http.NewRequest(http.MethodPost,
			"https://jules.googleapis.com/v1alpha/sessions", bytes.NewReader(payload))
		if err != nil {
			return 0, nil, fmt.Errorf("could not create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Goog-Api-Key", apiKey)
		resp, err := d.http.Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("Jules API request failed: %w", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, nil
	}

	code, respBody, err := doPost()
	if err != nil {
		return "", err
	}

	if code == http.StatusOK {
		return d.parseSessionID(respBody)
	}

	if code != http.StatusNotFound {
		return "", fmt.Errorf("Jules API returned %d: %s", code, strings.TrimSpace(string(respBody)))
	}

	// 404: check whether the source is simply not indexed yet.
	indexed, err := d.isSourceIndexed(sourceID, apiKey)
	if err != nil {
		return "", fmt.Errorf("Jules source check failed: %w", err)
	}
	if indexed {
		// Source exists in Jules but the API still returned 404 — real error.
		return "", fmt.Errorf("Jules API returned 404: %s", strings.TrimSpace(string(respBody)))
	}

	// Source not indexed yet — poll until it appears or timeout is exceeded.
	timeout := d.config.SourceIndexTimeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	interval := d.config.SourceIndexInterval
	if interval == 0 {
		interval = 10 * time.Second
	}
	d.log("Jules: source not indexed yet, waiting for", sourceID)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		time.Sleep(interval)
		found, err := d.isSourceIndexed(sourceID, apiKey)
		if err != nil {
			return "", fmt.Errorf("Jules source check failed: %w", err)
		}
		if !found {
			d.log("Jules: source still not indexed, retrying...")
			continue
		}
		d.log("Jules: source now indexed, retrying session...")
		code, respBody, err = doPost()
		if err != nil {
			return "", err
		}
		if code == http.StatusOK {
			return d.parseSessionID(respBody)
		}
		return "", fmt.Errorf("Jules API returned %d after source appeared: %s",
			code, strings.TrimSpace(string(respBody)))
	}

	return "", fmt.Errorf("Jules source %q not indexed after %s — add the repo to Jules and retry",
		sourceID, timeout)
}

// parseSessionID extracts the session ID from a Jules 200 response body.
func (d *JulesDriver) parseSessionID(respBody []byte) (string, error) {
	var julesResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &julesResp)
	d.sessionID = julesResp.ID
	return fmt.Sprintf("jules: %s", d.sessionID), nil
}

// isSourceIndexed paginates GET /v1alpha/sources and returns true if sourceID is found.
func (d *JulesDriver) isSourceIndexed(sourceID, apiKey string) (bool, error) {
	pageToken := ""
	for {
		url := "https://jules.googleapis.com/v1alpha/sources"
		if pageToken != "" {
			url += "?pageToken=" + pageToken
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return false, fmt.Errorf("could not create sources request: %w", err)
		}
		req.Header.Set("X-Goog-Api-Key", apiKey)

		resp, err := d.http.Do(req)
		if err != nil {
			return false, fmt.Errorf("Jules sources request failed: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var sr struct {
			Sources []struct {
				Name string `json:"name"`
			} `json:"sources"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &sr); err != nil {
			return false, fmt.Errorf("Jules sources parse error: %w", err)
		}
		for _, s := range sr.Sources {
			if s.Name == sourceID {
				return true, nil
			}
		}
		if sr.NextPageToken == "" {
			return false, nil
		}
		pageToken = sr.NextPageToken
	}
}

// resolveAPIKey returns config.APIKey or fetches it from keyring/prompt.
func (d *JulesDriver) resolveAPIKey() (string, error) {
	if d.config.APIKey != "" {
		return d.config.APIKey, nil
	}
	auth, err := NewJulesAuth()
	if err != nil {
		return "", fmt.Errorf("could not initialize keyring: %w", err)
	}
	auth.SetLog(d.log)
	return auth.EnsureAPIKey()
}

// resolveSourceID returns config.SourceID or auto-detects via gh CLI.
func (d *JulesDriver) resolveSourceID() (string, error) {
	if d.config.SourceID != "" {
		return d.config.SourceID, nil
	}
	return autoDetectSourceID()
}

// resolveBranch returns config.StartBranch or auto-detects via git.
func (d *JulesDriver) resolveBranch() (string, error) {
	if d.config.StartBranch != "" {
		return d.config.StartBranch, nil
	}
	return autoDetectBranch()
}

// autoDetectOwnerRepo uses gh CLI to return the GitHub owner and repo name.
func autoDetectOwnerRepo() (owner, repo string, err error) {
	out, err := RunCommandSilent("gh", "repo", "view", "--json", "owner,name")
	if err != nil {
		return "", "", fmt.Errorf("could not detect GitHub repo (is gh CLI installed?): %w", err)
	}
	var r struct {
		Owner struct{ Login string } `json:"owner"`
		Name  string                 `json:"name"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return "", "", fmt.Errorf("could not parse repo info: %w", err)
	}
	if r.Owner.Login == "" || r.Name == "" {
		return "", "", fmt.Errorf("incomplete repo info from gh: %s", out)
	}
	return r.Owner.Login, r.Name, nil
}

// autoDetectSourceID uses gh CLI to build the Jules source path.
func autoDetectSourceID() (string, error) {
	owner, repo, err := autoDetectOwnerRepo()
	if err != nil {
		return "", err
	}
	return "sources/github/" + owner + "/" + repo, nil
}

// autoDetectBranch uses git to get the current branch name.
func autoDetectBranch() (string, error) {
	branch, err := RunCommandSilent("git", "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("could not detect git branch: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main", nil
	}
	return branch, nil
}
