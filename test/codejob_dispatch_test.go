package devflow_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

// mockCodeJobDriver is a spy driver that records calls.
type mockCodeJobDriver struct {
	Called bool
	Output string
}

func (m *mockCodeJobDriver) Name() string { return "MockDriver" }
func (m *mockCodeJobDriver) SetLog(fn func(...any)) {}
func (m *mockCodeJobDriver) Send(prompt, title string) (string, error) {
	m.Called = true
	m.Output = "MockDriver dispatched: " + prompt
	return "Mock job dispatched", nil
}

// TestMergeAndPublish_DispatchesCodeJob verifies that MergeAndPublish triggers
// the next CodeJob dispatch if a new PLAN.md is pulled from the remote.
func TestMergeAndPublish_DispatchesCodeJob(t *testing.T) {
	dir := t.TempDir()

	// Manually implement testChdir logic since we can't easily import it from another test file in the same package
	// when running `go test test/file.go`.
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to %s: %v", dir, err)
	}
	defer os.Chdir(oldDir)

	// Setup: .env with a pending PR (required for MergeAndPublish to proceed)
	os.WriteFile(".env", []byte("CODEJOB_PR=https://github.com/test/pull/1\n"), 0644)

	// Setup: Mock git commands.
	// Critical part: "git pull" must CREATE docs/PLAN.md to simulate fetching a new plan.
	mockFn := func(name string, args ...string) *exec.Cmd {
		full := name + " " + strings.Join(args, " ")

		switch {
		// Simulate git pull creating docs/PLAN.md
		case full == "git pull":
			// We need to actually create the file so DispatchCodeJob sees it
			return exec.Command("sh", "-c", "mkdir -p docs && echo 'New Plan' > docs/PLAN.md")

		// Simulate clean git status
		case full == "git status --porcelain":
			return exec.Command("true")

		// Simulate getting current branch for PushWithTags
		case full == "git symbolic-ref --short HEAD":
			return exec.Command("echo", "main")

		// Simulate getting latest tag for GenerateNextTag (returns v0.0.1)
		case full == "git tag -l --sort=-version:refname":
			return exec.Command("echo", "v0.0.1")

		// Simulate checking if tag exists (returns false -> create new)
		case strings.HasPrefix(full, "git rev-parse v"):
			return exec.Command("sh", "-c", "exit 1")

		// Default success for other commands (gh pr merge, git checkout, git push, etc.)
		default:
			return exec.Command("true")
		}
	}

	orig := devflow.ExecCommand
	defer func() { devflow.ExecCommand = orig }()
	devflow.ExecCommand = mockFn

	// Create Git handler
	git, err := devflow.NewGit()
	if err != nil {
		t.Fatal(err)
	}

	// Inject Mock Driver
	driver := &mockCodeJobDriver{}
	git.SetCodeJobDrivers(driver)

	// Execute MergeAndPublish
	result, err := devflow.MergeAndPublish(git)
	if err != nil {
		t.Fatalf("MergeAndPublish failed: %v", err)
	}

	// Assertions
	if !driver.Called {
		t.Error("CodeJobDriver.Send() was NOT called. Dispatch flow failed.")
	}

	expectedSummaryPart := "Mock job dispatched"
	if !strings.Contains(result.Summary, expectedSummaryPart) {
		t.Errorf("Result summary missing dispatch info.\nExpected to contain: %q\nGot: %q", expectedSummaryPart, result.Summary)
	}
}
