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
	msg, done, err := devflow.JulesSessionState("S1", "key", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done=false while working")
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
	msg, done, err = devflow.JulesSessionState("S1", "key", client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true when PR is ready")
	}
	if !strings.Contains(msg, "PR ready") {
		t.Errorf("expected PR ready message, got %q", msg)
	}
	if !strings.Contains(msg, "github.com/test/pull/1") {
		t.Error("missing PR URL in message")
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

	// Since we can't easily mock RunCommandSilent without refactoring,
	// we'll just test the file operations and env cleanup.
	// If git fetch fails in this environment, we might get an error,
	// so we'll ignore it for this specific test or mock it if there was a way.

	err := devflow.HandleDone(env, nil)
	// We expect an error because 'git fetch' will likely fail in a non-git dir,
	// but let's see what happens.
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
}
