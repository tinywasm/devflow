package devflow_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

type mockStateHTTPClient struct {
	resp *http.Response
	err  error
}

func (m *mockStateHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestJulesSessionState(t *testing.T) {
	// Case 1: Working
	client := &mockStateHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"S1","outputs":[]}`)),
		},
	}
	msg, prURL, done, err := devflow.JulesSessionState("S1", "key", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done=false while working")
	}
	if prURL != "" {
		t.Errorf("expected empty PR URL while working, got %q", prURL)
	}
	if !strings.Contains(msg, "working") {
		t.Errorf("expected working message, got %q", msg)
	}

	// Case 2: Done (PR Ready)
	client = &mockStateHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"id":"S1",
				"outputs":[{"pullRequest":{"url":"https://github.com/test/pull/1","title":"feat: test"}}]
			}`)),
		},
	}
	msg, prURL, done, err = devflow.JulesSessionState("S1", "key", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true when PR is ready")
	}
	if prURL != "https://github.com/test/pull/1" {
		t.Errorf("expected PR URL 'https://github.com/test/pull/1', got %q", prURL)
	}
	if !strings.Contains(msg, "PR ready") {
		t.Errorf("expected PR ready message, got %q", msg)
	}
}

func TestHandleDone(t *testing.T) {
	envPath := "test_handle_done.env"
	planPath := "docs/PLAN.md"
	checkPlanPath := "docs/CHECK_PLAN.md"

	_ = os.MkdirAll("docs", 0755)
	defer os.RemoveAll("docs")
	defer os.Remove(envPath)

	_ = os.WriteFile(planPath, []byte("my plan"), 0644)
	_ = os.WriteFile(envPath, []byte("CODEJOB=jules:S1\nOTHER=val"), 0644)

	env := devflow.NewDotEnv(envPath)

	prURL := "https://github.com/test/pull/1"
	err := devflow.HandleDone(env, nil, prURL)
	if err != nil && !strings.Contains(err.Error(), "git fetch failed") {
		t.Errorf("expected git fetch error or success, got: %v", err)
	}

	// Verify PLAN.md renamed
	if _, err := os.Stat(planPath); err == nil {
		t.Error("PLAN.md should have been renamed")
	}
	if _, err := os.Stat(checkPlanPath); os.IsNotExist(err) {
		t.Error("CHECK_PLAN.md should exist")
	}

	// Verify env cleaned
	val, ok := env.Get("CODEJOB")
	if ok || val != "" {
		t.Error("CODEJOB should be deleted from .env")
	}
	val, ok = env.Get("OTHER")
	if !ok || val != "val" {
		t.Error("OTHER should be preserved in .env")
	}

	// Verify PR URL persisted
	val, ok = env.Get("CODEJOB_PR")
	if !ok || val != prURL {
		t.Errorf("expected CODEJOB_PR=%q, got %q", prURL, val)
	}
}

func TestMergePR_NoPRURL(t *testing.T) {
	envPath := "test_merge_no_pr.env"
	defer os.Remove(envPath)
	_ = os.WriteFile(envPath, []byte(""), 0644)

	// Since NewDotEnv is hardcoded to .env in MergePR, we'll temporarily swap it or
	// just expect it to fail if .env doesn't have the key.
	// Actually MergePR() calls NewDotEnv(".env"), so it's hard to test without .env
	// But in a test environment, we might not have .env, so it should fail.

	err := devflow.MergePR()
	if err == nil {
		t.Fatal("expected error when no PR URL in .env, got nil")
	}
	if !strings.Contains(err.Error(), "no pending PR found") {
		t.Errorf("expected 'no pending PR found' error, got: %v", err)
	}
}

func TestMergeAndPublish_NoPRURL(t *testing.T) {
	// MergeAndPublish reads ".env" via NewDotEnv(".env") — no CODEJOB_PR present
	// in the test environment means it returns immediately before any Git calls.
	_, err := devflow.MergeAndPublish(nil)
	if err == nil {
		t.Fatal("expected error when no PR URL in .env, got nil")
	}
	if !strings.Contains(err.Error(), "no pending PR found") {
		t.Errorf("expected 'no pending PR found' error, got: %v", err)
	}
}

// mockExecFor returns an ExecCommand replacement that records all calls and
// simulates a dirty or clean working tree. All other commands succeed silently.
// The returned *[]string grows with each command invocation.
func mockExecFor(dirtyStatus bool) (fn func(string, ...string) *exec.Cmd, calls *[]string) {
	recorded := []string{}
	calls = &recorded
	fn = func(name string, args ...string) *exec.Cmd {
		full := name + " " + strings.Join(args, " ")
		*calls = append(*calls, full)
		switch {
		case full == "git status --porcelain":
			if dirtyStatus {
				// Simulate two modified tracked files
				return exec.Command("echo", " M errors.go")
			}
			return exec.Command("true")
		case full == "git symbolic-ref --short HEAD":
			return exec.Command("echo", "main")
		case strings.HasPrefix(full, "git rev-parse v"):
			// Tag doesn't exist (TagExists returns false → CreateTag proceeds)
			return exec.Command("sh", "-c", "exit 1")
		default:
			return exec.Command("true")
		}
	}
	return
}

// TestMergeAndPublish_DirtyStateCommitsBeforeMerge verifies that when there are
// local uncommitted changes (review corrections), MergeAndPublish:
//  1. commits + pushes them to the Jules branch
//  2. then explicitly switches to main
//  3. then runs gh pr merge
func TestMergeAndPublish_DirtyStateCommitsBeforeMerge(t *testing.T) {
	dir := t.TempDir()
	defer testChdir(t, dir)()

	os.WriteFile(".env", []byte("CODEJOB_PR=https://github.com/test/pull/1\n"), 0644)

	mockFn, calls := mockExecFor(true)
	orig := devflow.ExecCommand
	defer func() { devflow.ExecCommand = orig }()
	devflow.ExecCommand = mockFn

	idxOf := func(prefix string) int {
		for i, c := range *calls {
			if strings.HasPrefix(c, prefix) {
				return i
			}
		}
		return -1
	}

	git, err := devflow.NewGit()
	if err != nil {
		t.Fatal(err)
	}
	devflow.MergeAndPublish(git) //nolint: the result is not relevant; we test the call sequence

	statusIdx := idxOf("git status --porcelain")
	addIdx := idxOf("git add .")
	commitIdx := idxOf("git commit -m review:")
	pushIdx := idxOf("git push")
	checkoutIdx := idxOf("git checkout main")
	mergeIdx := idxOf("gh pr merge")

	if statusIdx < 0 {
		t.Error("git status --porcelain was not called")
	}
	if addIdx < 0 {
		t.Error("git add . was not called (review corrections not staged)")
	}
	if commitIdx < 0 {
		t.Error("git commit review: corrections was not called (corrections not committed)")
	}
	if pushIdx < 0 {
		t.Error("git push was not called (corrections not pushed to Jules branch)")
	}
	if checkoutIdx < 0 {
		t.Error("git checkout main was not called before merge")
	}
	if mergeIdx < 0 {
		t.Error("gh pr merge was not called")
	}

	// Verify ordering: status → add → commit → push → checkout main → merge
	if addIdx < statusIdx {
		t.Errorf("git add (%d) should come after git status (%d)", addIdx, statusIdx)
	}
	if commitIdx < addIdx {
		t.Errorf("git commit (%d) should come after git add (%d)", commitIdx, addIdx)
	}
	if pushIdx < commitIdx {
		t.Errorf("git push (%d) should come after git commit (%d)", pushIdx, commitIdx)
	}
	if checkoutIdx < pushIdx {
		t.Errorf("git checkout main (%d) should come after git push (%d)", checkoutIdx, pushIdx)
	}
	if mergeIdx < checkoutIdx {
		t.Errorf("gh pr merge (%d) should come after git checkout main (%d)", mergeIdx, checkoutIdx)
	}
}

// TestMergeAndPublish_CleanStateSkipsPreCommit verifies that when the working
// tree is clean, no pre-merge commit is attempted, but the branch switch to
// main and gh pr merge still happen in the correct order.
func TestMergeAndPublish_CleanStateSkipsPreCommit(t *testing.T) {
	dir := t.TempDir()
	defer testChdir(t, dir)()

	os.WriteFile(".env", []byte("CODEJOB_PR=https://github.com/test/pull/1\n"), 0644)

	mockFn, calls := mockExecFor(false)
	orig := devflow.ExecCommand
	defer func() { devflow.ExecCommand = orig }()
	devflow.ExecCommand = mockFn

	idxOf := func(prefix string) int {
		for i, c := range *calls {
			if strings.HasPrefix(c, prefix) {
				return i
			}
		}
		return -1
	}

	git, err := devflow.NewGit()
	if err != nil {
		t.Fatal(err)
	}
	devflow.MergeAndPublish(git) //nolint: the result is not relevant; we test the call sequence

	commitIdx := idxOf("git commit -m review:")
	checkoutIdx := idxOf("git checkout main")
	mergeIdx := idxOf("gh pr merge")

	if commitIdx >= 0 {
		t.Error("git commit review: should NOT be called when working tree is clean")
	}
	if checkoutIdx < 0 {
		t.Error("git checkout main was not called")
	}
	if mergeIdx < 0 {
		t.Error("gh pr merge was not called")
	}
	if mergeIdx < checkoutIdx {
		t.Errorf("gh pr merge (%d) should come after git checkout main (%d)", mergeIdx, checkoutIdx)
	}
}
