package devflow

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// JulesSessionState polls the Jules API for session status.
// Returns (message, prURL, isDone, error).
func JulesSessionState(sessionID, apiKey string, client HTTPClient) (msg, prURL string, done bool, err error) {
	url := fmt.Sprintf("https://jules.googleapis.com/v1alpha/sessions/%s", sessionID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", false, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("X-Goog-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", false, fmt.Errorf("Jules API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", false, fmt.Errorf("Jules API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var session struct {
		ID      string `json:"id"`
		Outputs []struct {
			PullRequest struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"pullRequest"`
		} `json:"outputs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", "", false, fmt.Errorf("could not decode Jules response: %w", err)
	}

	for _, out := range session.Outputs {
		if out.PullRequest.URL != "" {
			msg := fmt.Sprintf("✅ Jules: PR ready\n   %s\n   %s", out.PullRequest.Title, out.PullRequest.URL)
			return msg, out.PullRequest.URL, true, nil
		}
	}

	return "⏳ Jules: working...", "", false, nil
}

// HandleDone executes cleanup when Jules completes:
// 1. git fetch --all
// 2. os.Rename("docs/PLAN.md", "docs/CHECK_PLAN.md")
// 3. env.Delete("CODEJOB")
// 4. env.Set("CODEJOB_PR", prURL)
// 5. Update .gitignore
func HandleDone(env *DotEnv, git *Git, prURL string) error {
	// 1. git fetch
	if _, err := RunCommandSilent("git", "fetch", "--all"); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	// 2. rename PLAN.md
	planPath := DefaultIssuePromptPath
	if _, err := os.Stat(planPath); err == nil {
		checkPlanPath := "docs/CHECK_PLAN.md"
		if err := os.Rename(planPath, checkPlanPath); err != nil {
			return fmt.Errorf("could not rename %s: %w", planPath, err)
		}
	}

	// 3. delete from env
	if err := env.Delete("CODEJOB"); err != nil {
		return fmt.Errorf("could not update .env: %w", err)
	}

	// 3b. persist PR URL for 'codejob done'
	if prURL != "" {
		if err := env.Set("CODEJOB_PR", prURL); err != nil {
			return fmt.Errorf("could not save CODEJOB_PR: %w", err)
		}
	}

	// 4. .gitignore update
	if git != nil {
		if err := git.GitIgnoreAdd("CHECK_*.md"); err != nil {
			return fmt.Errorf("could not update .gitignore: %w", err)
		}
	}

	return nil
}

// MergePR merges the Jules PR persisted in .env as CODEJOB_PR,
// deletes docs/CHECK_PLAN.md, and cleans up state.
// Called by 'codejob done'.
func MergePR() error {
	env := NewDotEnv(".env")
	prURL, ok := env.Get("CODEJOB_PR")
	if !ok || prURL == "" {
		return fmt.Errorf("no pending PR found. Run 'codejob' first to check status")
	}

	// 1. merge PR and delete Jules branch
	if out, err := RunCommandSilent("gh", "pr", "merge", prURL, "--merge", "--delete-branch"); err != nil {
		return fmt.Errorf("gh pr merge failed: %w\n%s", err, out)
	}

	// 2. delete docs/CHECK_PLAN.md
	if _, err := os.Stat("docs/CHECK_PLAN.md"); err == nil {
		if err := os.Remove("docs/CHECK_PLAN.md"); err != nil {
			return fmt.Errorf("could not delete CHECK_PLAN.md: %w", err)
		}
	}

	// 3. clean up .env
	if err := env.Delete("CODEJOB_PR"); err != nil {
		return fmt.Errorf("could not clean up .env: %w", err)
	}

	return nil
}

// MergeAndPublish merges the Jules PR, pulls the merged commit, commits any
// cleanup files (e.g. .gitignore updated by HandleDone), creates a new semver
// tag, and pushes it. This is the "close the loop" step that leaves the repo
// tagged and published, mirroring what 'push' does for regular commits.
func MergeAndPublish(git *Git) (PushResult, error) {
	env := NewDotEnv(".env")
	prURL, ok := env.Get("CODEJOB_PR")
	if !ok || prURL == "" {
		return PushResult{}, fmt.Errorf("no pending PR found. Run 'codejob' first to check status")
	}

	// 1. merge PR and delete Jules branch on GitHub
	if out, err := RunCommandSilent("gh", "pr", "merge", prURL, "--merge", "--delete-branch"); err != nil {
		return PushResult{}, fmt.Errorf("gh pr merge failed: %w\n%s", err, out)
	}

	// 2. pull the merged commit locally
	if _, err := RunCommandSilent("git", "pull"); err != nil {
		return PushResult{}, fmt.Errorf("git pull failed: %w", err)
	}

	// 3. remove CHECK_PLAN.md (gitignored, local cleanup only)
	if _, err := os.Stat("docs/CHECK_PLAN.md"); err == nil {
		if err := os.Remove("docs/CHECK_PLAN.md"); err != nil {
			return PushResult{}, fmt.Errorf("could not delete CHECK_PLAN.md: %w", err)
		}
	}

	// 4. commit any pending tracked changes (e.g., .gitignore updated by HandleDone)
	if err := git.Add(); err != nil {
		return PushResult{}, fmt.Errorf("git add failed: %w", err)
	}
	if _, err := git.Commit("chore: codejob cleanup"); err != nil {
		return PushResult{}, fmt.Errorf("commit cleanup failed: %w", err)
	}

	// 5. generate, create, and push the new version tag
	tag, err := git.GenerateNextTag()
	if err != nil {
		return PushResult{}, fmt.Errorf("failed to generate tag: %w", err)
	}
	if _, err := git.CreateTag(tag); err != nil {
		return PushResult{}, fmt.Errorf("failed to create tag %s: %w", tag, err)
	}
	if _, err := git.PushWithTags(tag); err != nil {
		return PushResult{}, fmt.Errorf("failed to push tag %s: %w", tag, err)
	}

	// 6. clean up state
	if err := env.Delete("CODEJOB_PR"); err != nil {
		return PushResult{}, fmt.Errorf("could not clean up .env: %w", err)
	}

	summary := fmt.Sprintf("✅ PR merged, ✅ Tag: %s, ✅ Pushed ok", tag)
	return PushResult{Tag: tag, Summary: summary}, nil
}
