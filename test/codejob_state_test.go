package devflow_test

import (
	"io"
	"net/http"
	"os"
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
