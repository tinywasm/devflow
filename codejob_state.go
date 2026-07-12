package devflow

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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

// CheckoutPRBranch fetches and hard-positions the working tree on the PR's
// head branch. A dirty working tree is handled, not feared: local drift is
// stashed with a labeled stash (CodejobStashMessage) and re-applied after the
// switch; if re-applying conflicts, the stash is KEPT, the conflict files are
// listed, and an error is returned. Returns the branch name on success.
func CheckoutPRBranch(prURL string) (string, error) {
	if prURL == "" {
		return "", fmt.Errorf("empty PR URL")
	}

	// 1. git fetch --all
	if _, err := RunCommandSilent("git", "fetch", "--all"); err != nil {
		return "", fmt.Errorf("git fetch --all failed: %w", err)
	}

	// 2. Resolve branch via gh pr view
	branchOut, err := RunCommandSilent("gh", "pr", "view", prURL, "--json", "headRefName", "--jq", ".headRefName")
	if err != nil {
		return "", fmt.Errorf("%s %s", HintManualPRCheckout, prURL)
	}
	branch := strings.TrimSpace(branchOut)
	if branch == "" {
		return "", fmt.Errorf("could not resolve branch name from PR %s", prURL)
	}

	// 3. Check for dirty working tree
	statusOut, _ := RunCommandSilent("git", "status", "--porcelain")
	isDirty := strings.TrimSpace(statusOut) != ""

	// 4. Stash if dirty
	if isDirty {
		if _, err := RunCommandSilent("git", "stash", "push", "-u", "-m", CodejobStashMessage); err != nil {
			return "", fmt.Errorf("git stash failed: %w", err)
		}
	}

	// 5. Checkout
	if _, err := RunCommandSilent("git", "checkout", branch); err != nil {
		// If checkout fails, try to restore stash before returning
		if isDirty {
			_, _ = RunCommandSilent("git", "stash", "pop")
		}
		return "", fmt.Errorf("%s\n    git checkout %s", HintManualCheckout, branch)
	}

	// 6. Verify checkout
	currentBranch, err := RunCommandSilent("git", "branch", "--show-current")
	if err != nil || strings.TrimSpace(currentBranch) != branch {
		return "", fmt.Errorf("checkout verification failed: expected %s, got %s", branch, strings.TrimSpace(currentBranch))
	}

	// 7. Pop stash if we stashed
	if isDirty {
		if out, err := RunCommandSilent("git", "stash", "pop"); err != nil {
			return branch, fmt.Errorf("conflict while re-applying local drift.\nStash kept: %s\nConflicts:\n%s", CodejobStashMessage, out)
		}
	}

	return branch, nil
}

// HandleDone executes cleanup when Jules completes:
// 1. Checkout PR branch (transactional, aborts on failure)
// 2. os.Rename("docs/PLAN.md", "docs/CHECK_PLAN.md")
// 3. env.Delete(EnvKeyCodejob)
// 4. env.Set(EnvKeyCodejobPR, prURL)
// 5. Update .gitignore
func HandleDone(env *DotEnv, git *Git, prURL string) error {
	// 1. Checkout PR branch (transactional)
	branch, err := CheckoutPRBranch(prURL)
	if err != nil {
		return err
	}
	fmt.Printf("🔀 On PR branch %s — review %s against this tree.\n", branch, DefaultCheckPlanPath)

	// 2. rename PLAN.md
	planPath := DefaultIssuePromptPath
	if _, err := os.Stat(planPath); err == nil {
		if err := os.Rename(planPath, DefaultCheckPlanPath); err != nil {
			return fmt.Errorf("could not rename %s: %w", planPath, err)
		}
	}

	// 3. delete from env
	if err := env.Delete(EnvKeyCodejob); err != nil {
		return fmt.Errorf("could not update .env: %w", err)
	}

	// 4. persist PR URL for 'codejob done'
	if prURL != "" {
		if err := env.Set(EnvKeyCodejobPR, prURL); err != nil {
			return fmt.Errorf("could not save %s: %w", EnvKeyCodejobPR, err)
		}
	}

	// 5. .gitignore update
	if git != nil {
		if err := git.GitIgnoreAdd("CHECK_*.md"); err != nil {
			return fmt.Errorf("could not update .gitignore: %w", err)
		}
	}

	return nil
}

const DefaultCheckPlanPath = "docs/CHECK_PLAN.md"

// MergePR merges the Jules PR persisted in .env as CODEJOB_PR,
// deletes docs/CHECK_PLAN.md, and cleans up state.
func MergePR() error {
	env := NewDotEnv(".env")
	prURL, ok := env.Get(EnvKeyCodejobPR)
	if !ok || prURL == "" {
		return fmt.Errorf("no pending PR found. Run 'codejob' first to check status")
	}

	// 1. merge PR and delete Jules branch
	var out string
	var err error
	for i := 0; i < 5; i++ {
		out, err = RunCommandSilent("gh", "pr", "merge", prURL, "--merge", "--delete-branch")
		if err == nil {
			break
		}
		if strings.Contains(out, "is not mergeable") || strings.Contains(err.Error(), "is not mergeable") {
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		return fmt.Errorf("gh pr merge failed: %w\n%s", err, out)
	}

	// 2. delete docs/CHECK_PLAN.md
	if _, err := os.Stat(DefaultCheckPlanPath); err == nil {
		if err := os.Remove(DefaultCheckPlanPath); err != nil {
			return fmt.Errorf("could not delete %s: %w", DefaultCheckPlanPath, err)
		}
	}

	// 3. clean up .env
	if err := env.Delete(EnvKeyCodejobPR); err != nil {
		return fmt.Errorf("could not clean up .env: %w", err)
	}

	return nil
}

// resolveDefaultBranch returns the repo's actual default branch (e.g. "main"
// or "master") by reading the cached origin/HEAD ref, falling back to "main"
// if that ref isn't set locally or the command fails.
func resolveDefaultBranch() string {
	out, err := RunCommandSilent("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	branch := strings.TrimSpace(out)
	if err == nil && branch != "" {
		return strings.TrimPrefix(branch, "origin/")
	}
	return "main"
}

// MergeAndPublish merges the Jules PR, pulls the merged commit, commits any
// cleanup files (e.g. .gitignore updated by HandleDone), and publishes via gopush.
func MergeAndPublish(publisher Publisher, message, overrideTag string) (PushResult, error) {
	if err := EnsureGHSession(); err != nil {
		return PushResult{}, err
	}
	env := NewDotEnv(".env")
	prURL, ok := env.Get(EnvKeyCodejobPR)
	if !ok || prURL == "" {
		return PushResult{}, fmt.Errorf("no pending PR found. Run 'codejob' first to check status")
	}

	// 0. Ensure we are on the Jules branch before committing anything
	if _, err := CheckoutPRBranch(prURL); err != nil {
		return PushResult{}, err
	}

	// 1. Pre-merge: if working tree is dirty, commit corrections to Jules branch and push
	statusOut, _ := RunCommandSilent("git", "status", "--porcelain")
	if strings.TrimSpace(statusOut) != "" {
		if out, err := RunCommandSilent("git", "add", "."); err != nil {
			return PushResult{}, fmt.Errorf("pre-merge git add failed: %w\n%s", err, out)
		}
		if out, err := RunCommandSilent("git", "commit", "-m", "review: corrections before merge"); err != nil {
			return PushResult{}, fmt.Errorf("pre-merge commit failed: %w\n%s", err, out)
		}
		if out, err := RunCommandSilent("git", "push"); err != nil {
			return PushResult{}, fmt.Errorf("pre-merge push failed: %w\n%s", err, out)
		}
	}

	// Switch to the repo's default branch before merging to avoid 'gh pr
	// merge' branch-switch errors. Not every repo uses "main" (e.g. forks of
	// pre-2020 projects commonly still use "master") — resolve it instead of
	// assuming.
	defaultBranch := resolveDefaultBranch()
	if out, err := RunCommandSilent("git", "checkout", defaultBranch); err != nil {
		return PushResult{}, fmt.Errorf("git checkout %s failed: %w\n%s", defaultBranch, err, out)
	}

	// 2. merge PR and delete Jules branch on GitHub
	var mergeOut string
	var mergeErr error
	for i := 0; i < 5; i++ {
		mergeOut, mergeErr = RunCommandSilent("gh", "pr", "merge", prURL, "--merge", "--delete-branch")
		if mergeErr == nil {
			break
		}
		errMsg := mergeErr.Error()
		if strings.Contains(mergeOut, "is not mergeable") || strings.Contains(errMsg, "is not mergeable") ||
			strings.Contains(mergeOut, "Base branch was modified") || strings.Contains(errMsg, "Base branch was modified") {
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	if mergeErr != nil {
		return PushResult{}, fmt.Errorf("gh pr merge failed: %w\n%s", mergeErr, mergeOut)
	}

	// 2. pull the merged commit locally
	if _, err := RunCommandSilent("git", "pull"); err != nil {
		return PushResult{}, fmt.Errorf("git pull failed: %w", err)
	}

	// 3. remove CHECK_PLAN.md (gitignored, local cleanup only)
	var planMeta PlanMeta
	if _, err := os.Stat(DefaultCheckPlanPath); err == nil {
		planMeta, _ = ReadPlanMeta(DefaultCheckPlanPath)
		if err := os.Remove(DefaultCheckPlanPath); err != nil {
			return PushResult{}, fmt.Errorf("could not delete %s: %w", DefaultCheckPlanPath, err)
		}
	}

	// 4. clean up state
	if err := env.Delete(EnvKeyCodejobPR); err != nil {
		return PushResult{}, fmt.Errorf("could not clean up .env: %w", err)
	}

	// 5. Check for new PLAN.md to re-dispatch
	if _, err := os.Stat(DefaultIssuePromptPath); err == nil {
		// PLAN.md exists -> call Publisher.Publish (skip deps + tag) + dispatch to agent
		res, err := publisher.Publish("chore: sync before re-dispatch", "", true, true, true, true, true, true)
		if err != nil {
			return PushResult{}, fmt.Errorf("re-dispatch sync failed: %w", err)
		}

		res.Summary = "✅ PR merged, 🚀 New plan detected, re-dispatching..."
		res.Tag = "RE_DISPATCH"
		return res, nil
	}

	// No PLAN.md -> call full gopush
	effMsg, effTag, err := ResolvePublishMessage(message, overrideTag, planMeta)
	if err != nil {
		return PushResult{}, err
	}
	return publisher.Publish(effMsg, effTag, false, false, false, false, false, false)
}
